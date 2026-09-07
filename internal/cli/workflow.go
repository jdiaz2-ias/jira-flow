package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"
	"jira-flow.local/jflow/internal/actionlog"
	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/cache"
	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/output"
	"jira-flow.local/jflow/internal/ports"
	"jira-flow.local/jflow/internal/provider/jiracloud"
)

type workflowSession interface {
	ports.IssueReader
	ports.TransitionGateway
}
type workflowFlags struct {
	timeout    time.Duration
	tokenStdin bool
}

func (f *workflowFlags) bind(cmd *cobra.Command) {
	cmd.Flags().DurationVar(&f.timeout, "timeout", 30*time.Second, "Total command timeout")
	cmd.Flags().BoolVar(&f.tokenStdin, "token-stdin", false, "Read ephemeral credential from stdin")
}
func (f workflowFlags) context(cmd *cobra.Command) (context.Context, context.CancelFunc, error) {
	return (readFlags{timeout: f.timeout}).context(cmd)
}
func addWorkflow(root *cobra.Command, deps Dependencies, access func() (app.Access, error), profile, email *string, emit func(output.Envelope, string, error) error) {
	memory := deps.Cache
	if memory == nil {
		memory = cache.New()
	}
	setup := func(ctx context.Context, cmd *cobra.Command, f workflowFlags) (*app.Workflow, app.Access, error) {
		a, err := access()
		if err != nil {
			return nil, a, err
		}
		var token ports.Secret
		if f.tokenStdin {
			token, err = readToken(cmd.InOrStdin())
			if err != nil {
				return nil, a, err
			}
		}
		name, p, token, err := a.ReadSession(ctx, *profile, *email, token)
		if err != nil {
			return nil, a, err
		}
		var source workflowSession
		if deps.Workflow != nil {
			source, err = deps.Workflow(p, token)
		} else {
			var client *jiracloud.Client
			client, err = jiracloud.New(a.Env("JFLOW_CA_CERT"))
			if err == nil {
				source = &jiracloud.Session{Client: client, Profile: p, Secret: token}
			}
		}
		if err != nil {
			return nil, a, err
		}
		scope := app.NewReader(source, name, p, token, memory).Scope
		return &app.Workflow{Reader: source, Gateway: source, Profile: name, Site: p.SiteURL, ExpectedAccount: p.AccountID, Rules: p.WorkflowRules, Invalidate: func() { memory.DeletePrefix(scope + ":") }}, a, nil
	}
	interactive := func(cmd *cobra.Command, a app.Access, tokenStdin bool) bool {
		noInput, _ := cmd.Flags().GetBool("no-input")
		format, _ := cmd.Flags().GetString("format")
		if noInput || format == "json" || a.Env("JFLOW_NO_INPUT") == "1" || a.Env("JFLOW_NO_INPUT") == "true" || tokenStdin {
			return false
		}
		if deps.Interactive != nil {
			return deps.Interactive()
		}
		in, ok := cmd.InOrStdin().(*os.File)
		if !ok {
			return false
		}
		out, ok := cmd.OutOrStdout().(*os.File)
		return ok && term.IsTerminal(int(in.Fd())) && term.IsTerminal(int(out.Fd()))
	}
	ask := func(ctx context.Context, cmd *cobra.Command, label string) (string, error) {
		if deps.Prompt != nil {
			return deps.Prompt(ctx, label)
		}
		if _, err := fmt.Fprint(cmd.ErrOrStderr(), label); err != nil {
			return "", &domain.Error{Kind: domain.Internal, Message: "Could not write the prompt."}
		}
		type answer struct {
			value string
			err   error
		}
		ready := make(chan answer, 1)
		go func() { value, err := readLine(ctx, cmd.InOrStdin()); ready <- answer{value, err} }()
		select {
		case result := <-ready:
			return result.value, result.err
		case <-ctx.Done():
			return "", &domain.Error{Kind: domain.Canceled, Message: "Operation cancelled or deadline exceeded."}
		}
	}
	var inspectFlags workflowFlags
	inspect := &cobra.Command{Use: "transitions KEY", Short: "List fresh transitions and field metadata", Args: cobra.ExactArgs(1), Annotations: map[string]string{"collection": "true"}}
	inspect.RunE = func(cmd *cobra.Command, args []string) error {
		if _, err := domain.IssueKey(args[0]); err != nil {
			return err
		}
		ctx, cancel, err := inspectFlags.context(cmd)
		if err != nil {
			return err
		}
		defer cancel()
		w, _, err := setup(ctx, cmd, inspectFlags)
		if err != nil {
			return err
		}
		c, e := w.Inspect(ctx, args[0])
		if e != nil {
			return e
		}
		return emit(output.TransitionsEnvelope(w.Profile, c, nil), output.TransitionsText(c), nil)
	}
	inspectFlags.bind(inspect)
	root.AddCommand(inspect)
	for _, name := range []string{"transition", "start", "done", "close"} {
		var f workflowFlags
		var id, fieldsFile string
		var yes, dryRun, noRecord bool
		cmd := &cobra.Command{Use: name + " KEY", Short: map[string]string{"transition": "Prepare and apply an explicit Jira transition", "start": "Move an issue into an In progress state", "done": "Move an issue into a Done state", "close": "Apply an explicitly chosen or mapped closing transition"}[name], Args: cobra.ExactArgs(1)}
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			key, err := domain.IssueKey(args[0])
			if err != nil {
				return err
			}
			fields, err := loadFields(fieldsFile)
			if err != nil {
				return err
			}
			ctx, cancel, err := f.context(cmd)
			if err != nil {
				return err
			}
			defer cancel()
			w, a, err := setup(ctx, cmd, f)
			if err != nil {
				return err
			}
			canAsk := interactive(cmd, a, f.tokenStdin)
			intent := domain.Intent(name)
			if name == "transition" {
				intent = ""
			}
			options := app.PrepareOptions{Key: key, Intent: intent, TransitionID: id, Fields: fields}
			plan, prepareErr := w.Prepare(ctx, options)
			for round := 0; prepareErr != nil && canAsk && round < 4; round++ {
				var public *domain.Error
				if !errors.As(prepareErr, &public) {
					break
				}
				if public.Kind == domain.TransitionAmbiguous {
					if _, err := fmt.Fprintln(cmd.ErrOrStderr(), output.TransitionsText(app.Catalog{Issue: plan.Catalog.Issue, Transitions: plan.Candidates})); err != nil {
						return &domain.Error{Kind: domain.Internal, Message: "Could not display candidate transitions."}
					}
					choice, err := ask(ctx, cmd, "Transition ID: ")
					if err != nil {
						return err
					}
					valid := false
					for _, candidate := range plan.Candidates {
						if candidate.ID == choice {
							valid = true
						}
					}
					if !valid {
						return &domain.Error{Kind: domain.Validation, Message: "Choose one of the displayed transition IDs."}
					}
					options.TransitionID = choice
				} else if public.Kind == domain.Validation && plan.Selected != nil && len(plan.Missing) > 0 {
					for _, field := range plan.Missing {
						value, err := promptField(ctx, cmd, field, ask)
						if err != nil {
							return err
						}
						options.Fields[field.ID] = value
					}
				} else {
					break
				}
				plan, prepareErr = w.Prepare(ctx, options)
			}
			if prepareErr != nil {
				return emit(output.PreparationEnvelope(plan, prepareErr), preparationFailureText(plan), prepareErr)
			}
			// Optional values are opt-in and use only fields exposed by the chosen transition.
			if canAsk && !yes && !plan.Noop && plan.Selected != nil {
				answer, err := ask(ctx, cmd, "Edit an optional transition field? [y/N]: ")
				if err != nil {
					return err
				}
				for edits := 0; affirmative(answer) && edits < 50; edits++ {
					names := []string{}
					for _, field := range plan.Selected.Fields {
						if !field.Required {
							names = append(names, field.ID)
						}
					}
					if len(names) == 0 {
						break
					}
					if _, err = fmt.Fprintln(cmd.ErrOrStderr(), "Optional fields: "+strings.Join(names, ", ")); err != nil {
						return &domain.Error{Kind: domain.Internal, Message: "Could not display fields."}
					}
					fieldID, err := ask(ctx, cmd, "Field ID: ")
					if err != nil {
						return err
					}
					var selected *domain.FieldSpec
					for _, field := range plan.Selected.Fields {
						if field.ID == fieldID && !field.Required {
							copy := field
							selected = &copy
						}
					}
					if selected == nil {
						return &domain.Error{Kind: domain.Validation, Message: "Choose an available optional field ID."}
					}
					value, err := promptField(ctx, cmd, *selected, ask)
					if err != nil {
						return err
					}
					options.Fields[fieldID] = value
					options.TransitionID = plan.Selected.ID
					plan, err = w.Prepare(ctx, options)
					if err != nil {
						return emit(output.PreparationEnvelope(plan, err), preparationFailureText(plan), err)
					}
					answer, err = ask(ctx, cmd, "Edit another optional field? [y/N]: ")
					if err != nil {
						return err
					}
				}
			}
			if dryRun {
				return emit(output.PreparationEnvelope(plan, nil), output.Preview(plan)+"\nDry run: no write sent.", nil)
			}
			if !plan.Noop && !yes {
				if !canAsk {
					err = &domain.Error{Kind: domain.InvalidInput, Message: "Confirmation is required. Use --yes to apply or --dry-run to preview; no write was sent."}
					return emit(output.PreparationEnvelope(plan, err), output.Preview(plan), err)
				}
				if _, err = fmt.Fprintln(cmd.ErrOrStderr(), output.Preview(plan)); err != nil {
					return &domain.Error{Kind: domain.Internal, Message: "Could not show the action preview; no write was sent."}
				}
				answer, err := ask(ctx, cmd, "Apply this change? [y/N]: ")
				if err != nil {
					return err
				}
				if !affirmative(answer) {
					return &domain.Error{Kind: domain.Canceled, Message: "Action cancelled; no write was sent."}
				}
			}
			result, applyErr := w.Apply(ctx, plan)
			actionID := ""
			if result.Attempted && !noRecord {
				home, homeErr := os.UserHomeDir()
				if homeErr == nil {
					paths := config.ResolvePaths(runtime.GOOS, home, a.Env)
					if a.Env("JFLOW_CONFIG") != "" {
						paths.State = filepath.Join(filepath.Dir(a.Path), "state")
					}
					scope := sha256.Sum256([]byte(w.Profile + "\x00" + w.Site + "\x00" + w.ExpectedAccount))
					recordCtx, recordCancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
					var logErr error
					actionID, logErr = actionlog.Save(recordCtx, filepath.Join(paths.State, "actions", hex.EncodeToString(scope[:])), actionlog.Record{Key: result.Apply.Issue.Key, Intent: plan.Intent, TransitionID: result.Apply.TransitionID, Result: result.Apply.State})
					recordCancel()
					if logErr != nil {
						result.Warnings = append(result.Warnings, "Action metadata could not be fully saved or pruned; this does not change the Jira outcome.")
					}
				} else {
					result.Warnings = append(result.Warnings, "Could not resolve the local action-history directory.")
				}
			}
			envelope := output.WorkflowEnvelope(result, applyErr)
			envelope.Data.(map[string]any)["action_id"] = actionID
			return emit(envelope, output.WorkflowText(result), applyErr)
		}
		f.bind(cmd)
		cmd.Flags().StringVar(&id, "transition-id", "", "Explicit available transition ID")
		cmd.Flags().StringVar(&id, "id", "", "Alias for --transition-id")
		cmd.Flags().StringVar(&fieldsFile, "fields-file", "", "JSON object of transition fields (up to 1 MiB)")
		cmd.Flags().BoolVar(&yes, "yes", false, "Confirm a fully resolved action; does not choose ambiguous transitions")
		cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Read and validate without sending a write")
		cmd.Flags().BoolVar(&noRecord, "no-record", false, "Do not store local action metadata")
		root.AddCommand(cmd)
	}
	workflows := &cobra.Command{Use: "workflow", Short: "Map workflow intents to validated transition IDs"}
	var mapFlags workflowFlags
	var mapIntent, mapID string
	mapping := &cobra.Command{Use: "map KEY", Short: "Save a local rule from an issue's current workflow context", Args: cobra.ExactArgs(1)}
	mapping.RunE = func(cmd *cobra.Command, args []string) error {
		if _, err := domain.IssueKey(args[0]); err != nil {
			return err
		}
		intent := domain.Intent(mapIntent)
		if intent != domain.IntentStart && intent != domain.IntentDone && intent != domain.IntentClose {
			return &domain.Error{Kind: domain.InvalidInput, Message: "--intent must be start, done or close."}
		}
		ctx, cancel, err := mapFlags.context(cmd)
		if err != nil {
			return err
		}
		defer cancel()
		w, a, err := setup(ctx, cmd, mapFlags)
		if err != nil {
			return err
		}
		c, err := w.Inspect(ctx, args[0])
		if err != nil {
			return err
		}
		id := mapID
		if id == "" && interactive(cmd, a, mapFlags.tokenStdin) {
			if _, err = fmt.Fprintln(cmd.ErrOrStderr(), output.TransitionsText(c)); err != nil {
				return &domain.Error{Kind: domain.Internal, Message: "Could not show transitions."}
			}
			id, err = ask(ctx, cmd, "Transition ID to map: ")
			if err != nil {
				return err
			}
		}
		var choice *domain.Transition
		for _, t := range c.Transitions {
			if t.ID == id {
				copy := t
				choice = &copy
			}
		}
		if choice == nil {
			return &domain.Error{Kind: domain.Validation, Message: "Choose an available --transition-id for the mapping."}
		}
		if (intent == domain.IntentStart && choice.To.Category != domain.CategoryInProgress) || (intent == domain.IntentDone && choice.To.Category != domain.CategoryDone) {
			return &domain.Error{Kind: domain.Validation, Message: "The mapped destination does not satisfy this intent."}
		}
		i := c.Issue.Issue
		rule := domain.WorkflowRule{ProjectID: i.ProjectID, IssueTypeID: i.IssueTypeID, Intent: intent, FromStatusID: i.Status.ID, TransitionID: id, ExpectedToStatusID: choice.To.ID}
		if err = rule.Validate(); err != nil {
			return err
		}
		err = config.Update(ctx, a.Path, func(cfg *config.Config) error {
			p, ok := cfg.Profiles[w.Profile]
			if !ok || p.SiteURL != w.Site || p.AccountID != w.ExpectedAccount {
				return &domain.Error{Kind: domain.Conflict, Message: "The profile changed while creating the mapping."}
			}
			out := []domain.WorkflowRule{}
			for _, old := range p.WorkflowRules {
				if old.ProjectID == rule.ProjectID && old.IssueTypeID == rule.IssueTypeID && old.Intent == rule.Intent && old.FromStatusID == rule.FromStatusID {
					continue
				}
				out = append(out, old)
			}
			p.WorkflowRules = append(out, rule)
			cfg.Profiles[w.Profile] = p
			return nil
		})
		if err != nil {
			return err
		}
		return emit(output.Success(map[string]any{"profile": w.Profile, "rule": rule}), fmt.Sprintf("Mapped %s: %s -> transition %s -> state %s. No Jira write sent.", intent, i.Ref.Key, id, choice.To.ID), nil)
	}
	mapFlags.bind(mapping)
	mapping.Flags().StringVar(&mapIntent, "intent", "", "Intent to map: start, done or close")
	mapping.Flags().StringVar(&mapID, "transition-id", "", "Available transition ID")
	workflows.AddCommand(mapping)
	root.AddCommand(workflows)
}
func preparationFailureText(p app.Preparation) string {
	if len(p.Candidates) > 0 {
		return output.TransitionsText(app.Catalog{Issue: p.Catalog.Issue, Transitions: p.Candidates})
	}
	if p.Catalog.Issue.Issue.Ref.ID != "" {
		return output.Preview(p)
	}
	return ""
}
func affirmative(s string) bool {
	return strings.EqualFold(strings.TrimSpace(s), "y") || strings.EqualFold(strings.TrimSpace(s), "yes")
}
func readLine(ctx context.Context, in io.Reader) (string, error) {
	one := make([]byte, 1)
	b := []byte{}
	for len(b) < 65536 {
		if ctx.Err() != nil {
			return "", &domain.Error{Kind: domain.Canceled, Message: "Operation cancelled or deadline exceeded."}
		}
		n, err := in.Read(one)
		if n > 0 {
			if one[0] == '\n' {
				return strings.TrimSuffix(string(b), "\r"), nil
			}
			b = append(b, one[0])
		}
		if err != nil {
			if err == io.EOF && len(b) > 0 {
				return string(b), nil
			}
			return "", &domain.Error{Kind: domain.InvalidInput, Message: "Input ended before an answer was provided."}
		}
	}
	return "", &domain.Error{Kind: domain.InvalidInput, Message: "Input exceeds 64 KiB."}
}
func loadFields(path string) (map[string]domain.FieldValue, error) {
	result := map[string]domain.FieldValue{}
	if path == "" {
		return result, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, &domain.Error{Kind: domain.InvalidInput, Message: "Could not read the fields file."}
	}
	defer f.Close()
	b, err := io.ReadAll(io.LimitReader(f, 1024*1024+1))
	if err != nil || len(b) > 1024*1024 || json.Unmarshal(b, &result) != nil || result == nil {
		return nil, &domain.Error{Kind: domain.InvalidInput, Message: "Fields file must contain one JSON object of at most 1 MiB."}
	}
	for id := range result {
		if !domain.ValidFieldID(id) || id == "transition" || id == "fields" || id == "update" {
			return nil, &domain.Error{Kind: domain.InvalidInput, Message: "Fields file must contain field IDs, not a complete request body."}
		}
	}
	return result, nil
}
func promptField(ctx context.Context, cmd *cobra.Command, f domain.FieldSpec, ask func(context.Context, *cobra.Command, string) (string, error)) (any, error) {
	if len(f.AllowedValues) > 0 {
		names := []string{}
		for _, value := range f.AllowedValues {
			names = append(names, value.ID+"="+domain.CleanText(value.Name, false))
		}
		sort.Strings(names)
		if _, err := fmt.Fprintln(cmd.ErrOrStderr(), "Allowed values: "+strings.Join(names, ", ")); err != nil {
			return nil, &domain.Error{Kind: domain.Internal, Message: "Could not show field choices."}
		}
	}
	label := fmt.Sprintf("%s [%s, %s]: ", domain.CleanText(f.Name, false), f.ID, f.Type)
	answer, err := ask(ctx, cmd, label)
	if err != nil {
		return nil, err
	}
	var value any
	switch f.Type {
	case "adf":
		value = domain.TextADF(answer)
	case "string", "date", "datetime":
		value = answer
	case "number", "integer":
		n, e := strconv.ParseFloat(answer, 64)
		if e != nil {
			return nil, &domain.Error{Kind: domain.Validation, Message: "Enter a valid number."}
		}
		value = n
	case "boolean":
		v, e := strconv.ParseBool(answer)
		if e != nil {
			return nil, &domain.Error{Kind: domain.Validation, Message: "Enter true or false."}
		}
		value = v
	case "user":
		value = map[string]any{"accountId": answer}
	case "option", "resolution", "priority", "issuetype", "project", "version", "component":
		value = map[string]any{"id": answer}
	case "array":
		if json.Unmarshal([]byte(answer), &value) != nil {
			return nil, &domain.Error{Kind: domain.Validation, Message: "Enter a JSON array of values or ID objects."}
		}
	default:
		return nil, &domain.Error{Kind: domain.Unsupported, Message: "Unsupported field schema. Use jflow open to complete this transition in Jira."}
	}
	if err = domain.ValidateFields([]domain.FieldSpec{f}, map[string]any{f.ID: value}, true); err != nil {
		return nil, err
	}
	return value, nil
}
