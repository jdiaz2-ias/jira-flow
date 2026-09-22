package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/browser"
	"jira-flow.local/jflow/internal/cache"
	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/output"
	"jira-flow.local/jflow/internal/provider/jiracloud"
	"jira-flow.local/jflow/internal/tui"
)

type uiWorkflow struct {
	*app.Workflow
	access   app.Access
	noRecord bool
}

func (w uiWorkflow) Apply(ctx context.Context, p app.Preparation) (app.WorkflowResult, error) {
	r, err := w.Workflow.Apply(ctx, p)
	recordWorkflow(ctx, w.access, w.Workflow, &r, w.noRecord)
	return r, err
}
func addUI(root *cobra.Command, deps Dependencies, access func() (app.Access, error), reader func(context.Context, *cobra.Command, readFlags, app.Access) (*app.Reader, error), profile, email *string, memory *cache.Memory) {
	var f readFlags
	var theme string
	var accessible, noRecord bool
	cmd := &cobra.Command{Use: "ui", Short: "Work with Jira interactively", Args: cobra.NoArgs}
	// The root and explicit ui entry share the same options and execution path.
	bind := func(c *cobra.Command) {
		c.Flags().BoolVar(&f.offline, "offline", false, "Use cached results without contacting Jira")
		c.Flags().BoolVar(&f.refresh, "refresh", false, "Skip cache and query Jira")
		c.Flags().DurationVar(&f.timeout, "timeout", 30*time.Second, "Timeout for each read or workflow operation")
		c.Flags().StringVar(&theme, "theme", "auto", "Terminal theme: auto, dark, light or mono")
		c.Flags().BoolVar(&accessible, "accessible", false, "Print a plain listing without a full-screen interface")
		c.Flags().BoolVar(&noRecord, "no-record", false, "Do not store local action metadata")
	}
	bind(cmd)
	bind(root)
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		env := deps.Env
		if env == nil {
			env = os.Getenv
		}
		noInput, _ := cmd.Flags().GetBool("no-input")
		interactive := terminalUI(cmd, deps)
		if cmd.Flags().Changed("format") || (!accessible && (noInput || env("JFLOW_NO_INPUT") == "1" || env("JFLOW_NO_INPUT") == "true" || env("TERM") == "dumb" || !interactive)) {
			return &domain.Error{Kind: domain.InvalidInput, Message: "Use jflow --help or jflow mine / show --format plain. The interactive UI requires terminal stdin/stdout, without --format or --no-input; --accessible prints a plain listing."}
		}
		if f.timeout <= 0 || f.timeout > 10*time.Minute || (f.offline && f.refresh) {
			return &domain.Error{Kind: domain.InvalidInput, Message: "Invalid timeout or incompatible --offline/--refresh."}
		}
		if theme != "auto" && theme != "dark" && theme != "light" && theme != "mono" {
			return &domain.Error{Kind: domain.InvalidInput, Message: "Use --theme auto, dark, light or mono."}
		}
		a, err := access()
		if err != nil {
			return err
		}
		cfg, err := config.Load(a.Path)
		if err != nil {
			return err
		}
		if len(cfg.Profiles) == 0 && !accessible && !f.offline {
			if err = setupUIProfile(root, cmd, deps); err != nil {
				return err
			}
		}
		setup, cancel := context.WithTimeout(cmd.Context(), f.timeout)
		r, err := reader(setup, cmd, f, a)
		cancel()
		if err != nil {
			return err
		}
		// Resolve the same project defaults used by mine.
		cfg, err = config.Load(a.Path)
		if err != nil {
			return err
		}
		_, p, err := cfg.Select(*profile, a.Env)
		if err != nil {
			return err
		}
		project := a.Env("JFLOW_PROJECT")
		if project == "" {
			project = p.DefaultProject
		}
		jql, err := (domain.QueryOptions{Mode: "mine", Project: project}).Build()
		if err != nil {
			return err
		}
		options := tui.Options{Profile: r.Profile, Site: p.SiteURL, Theme: theme, Timeout: f.timeout, Search: app.SearchOptions{Query: domain.SearchRequest{JQL: jql, Fields: append([]string(nil), jiracloud.ListFields...), PageSize: 50}, Limit: 50, MaxResults: 5000, Refresh: f.refresh, Offline: f.offline}}
		if accessible {
			ctx, cancel := context.WithTimeout(cmd.Context(), f.timeout)
			defer cancel()
			result, readErr := r.Search(ctx, options.Search)
			if _, err = fmt.Fprintln(cmd.OutOrStdout(), output.Rows(result, false)); err != nil {
				return &domain.Error{Kind: domain.Internal, Message: "Could not write listing.", Cause: err}
			}
			return readErr
		}
		options.ASCII, _ = cmd.Flags().GetBool("ascii")
		options.NoColor, _ = cmd.Flags().GetBool("no-color")
		options.NoColor = options.NoColor || env("NO_COLOR") != ""
		launcher := deps.Browser
		if launcher == nil {
			launcher = browser.New()
		}
		options.Open = launcher.Open
		// Freeze the selected profile, rather than following later active-profile changes.
		selectedProfile := r.Profile
		options.Link = func(key string) (string, error) { _, _, link, err := a.Link(selectedProfile, key); return link, err }
		if !f.offline {
			ctx, cancel := context.WithTimeout(cmd.Context(), f.timeout)
			w, wa, err := setupWorkflow(ctx, cmd, workflowFlags{timeout: f.timeout}, deps, access, &selectedProfile, email, memory)
			cancel()
			if err != nil {
				return err
			}
			options.Workflow = uiWorkflow{Workflow: w, access: wa, noRecord: noRecord}
		}
		ctx, stop := context.WithCancel(cmd.Context())
		defer stop()
		model := tui.New(ctx, r, options)
		run := deps.RunUI
		if run == nil {
			run = func(ctx context.Context, m *tui.Model, in io.Reader, out io.Writer) error {
				_, err := tea.NewProgram(m, tea.WithContext(ctx), tea.WithInput(in), tea.WithOutput(out), tea.WithoutSignalHandler()).Run()
				return err
			}
		}
		err = run(ctx, model, cmd.InOrStdin(), cmd.OutOrStdout())
		if model.PendingWrite() || model.UncertainOutcome() {
			return &domain.Error{Kind: domain.Uncertain, Message: "The session ended while applying a transition. Inspect Jira before retrying; the write outcome may be unknown.", Cause: err}
		}
		if err != nil {
			if errors.Is(err, tea.ErrProgramKilled) || ctx.Err() != nil {
				return &domain.Error{Kind: domain.Canceled, Message: "Interactive session canceled.", Cause: err}
			}
			return &domain.Error{Kind: domain.Internal, Message: "Could not run the interactive terminal.", Cause: err}
		}
		return nil
	}
	root.RunE = cmd.RunE
	root.AddCommand(cmd)
}
func terminalUI(cmd *cobra.Command, deps Dependencies) bool {
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
func setupUIProfile(root, cmd *cobra.Command, deps Dependencies) error {
	// Run the existing hidden-token wizard before entering raw/full-screen mode.
	login, _, err := root.Find([]string{"auth", "login"})
	if err != nil {
		return err
	}
	login.SetContext(cmd.Context())
	ask := deps.Prompt
	if ask == nil {
		ask = func(ctx context.Context, label string) (string, error) {
			if _, err := fmt.Fprint(cmd.ErrOrStderr(), label); err != nil {
				return "", err
			}
			return readLine(ctx, cmd.InOrStdin())
		}
	}
	method, err := ask(cmd.Context(), "Configuracion inicial. Token scoped? [y/N] (Ctrl+C cancela): ")
	if err != nil {
		return err
	}
	value := "api-token-unscoped"
	if affirmative(method) {
		value = "api-token-scoped"
	}
	if err = login.Flags().Set("method", value); err != nil {
		return err
	}
	return login.RunE(login, nil)
}
