package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/browser"
	"jira-flow.local/jflow/internal/cache"
	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/output"
	"jira-flow.local/jflow/internal/ports"
	"jira-flow.local/jflow/internal/provider/jiracloud"
)

type readFlags struct {
	refresh, offline, tokenStdin bool
	timeout                      time.Duration
}

func (f *readFlags) bind(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&f.refresh, "refresh", false, "Skip cache and query Jira")
	cmd.Flags().BoolVar(&f.offline, "offline", false, "Use only cache from this process (no persistence in F2)")
	cmd.Flags().BoolVar(&f.tokenStdin, "token-stdin", false, "Read ephemeral credential from stdin")
	cmd.Flags().DurationVar(&f.timeout, "timeout", 30*time.Second, "Total command timeout")
}
func (f readFlags) context(cmd *cobra.Command) (context.Context, context.CancelFunc, error) {
	if f.timeout <= 0 || f.timeout > 10*time.Minute || (f.offline && f.refresh) {
		return nil, nil, &domain.Error{Kind: domain.InvalidInput, Message: "--timeout must be greater than zero and at most 10m; --offline and --refresh are incompatible."}
	}
	ctx, cancel := context.WithTimeout(cmd.Context(), f.timeout)
	return ctx, cancel, nil
}
func addReading(root *cobra.Command, deps Dependencies, access func() (app.Access, error), profile, email *string, emit func(output.Envelope, string, error) error) {
	memory := deps.Cache
	if memory == nil {
		memory = cache.New()
	}
	selected := func() (app.Access, string, config.Profile, error) {
		a, err := access()
		if err != nil {
			return a, "", config.Profile{}, err
		}
		c, err := config.Load(a.Path)
		if err != nil {
			return a, "", config.Profile{}, err
		}
		name, p, err := c.Select(*profile, a.Env)
		return a, name, p, err
	}
	reader := func(ctx context.Context, cmd *cobra.Command, f readFlags, a app.Access) (*app.Reader, error) {
		var token ports.Secret
		var err error
		if f.tokenStdin {
			token, err = readToken(cmd.InOrStdin())
			if err != nil {
				return nil, err
			}
		}
		name, p, token, err := a.ReadSession(ctx, *profile, *email, token)
		if err != nil {
			return nil, err
		}
		var source ports.IssueReader
		if deps.IssueReader != nil {
			source, err = deps.IssueReader(p, token)
		} else {
			var client *jiracloud.Client
			client, err = jiracloud.New(a.Env("JFLOW_CA_CERT"))
			if err == nil {
				source = &jiracloud.Session{Client: client, Profile: p, Secret: token}
			}
		}
		if err != nil {
			return nil, err
		}
		return app.NewReader(source, name, p, token, memory), nil
	}
	for _, mode := range []string{"mine", "list", "search"} {
		var f readFlags
		var q domain.QueryOptions
		var limit, pageSize, maxResults int
		var all bool
		var pageToken string
		var fields []string
		q.Mode = mode
		cmd := &cobra.Command{Use: mode, Short: map[string]string{"mine": "Query my pending issues", "list": "List issues with an explicit scope", "search": "Execute a JQL query"}[mode], Args: cobra.NoArgs, Annotations: map[string]string{"collection": "true"}}
		cmd.RunE = func(cmd *cobra.Command, _ []string) error {
			ctx, cancel, err := f.context(cmd)
			if err != nil {
				return err
			}
			defer cancel()
			if limit < 1 || limit > 5000 || pageSize < 1 || pageSize > 100 || maxResults < 1 || maxResults > 5000 {
				return &domain.Error{Kind: domain.InvalidInput, Message: "--limit and --max-results must be between 1 and 5000; --page-size between 1 and 100."}
			}
			if all && cmd.Flags().Changed("limit") {
				return &domain.Error{Kind: domain.InvalidInput, Message: "--all does not combine with --limit; use --max-results as a guard limit."}
			}
			for _, field := range fields {
				allowed := false
				for _, known := range jiracloud.ListFields {
					if field == known {
						allowed = true
					}
				}
				if !allowed {
					return &domain.Error{Kind: domain.InvalidInput, Message: "--fields selection not allowed; see help."}
				}
			}
			if len(fields) == 0 {
				return &domain.Error{Kind: domain.InvalidInput, Message: "--fields requires at least one field."}
			}
			sort.Strings(fields)
			fields = compact(fields)
			if mode == "search" {
				for _, flag := range []string{"project", "status-category", "type", "priority", "include-done", "updated-since", "sort"} {
					if cmd.Flags().Changed(flag) {
						return &domain.Error{Kind: domain.InvalidInput, Message: "Do not combine search --jql with structured filters."}
					}
				}
				if _, err = q.Build(); err != nil {
					return err
				}
			}
			a, _, p, err := selected()
			if err != nil {
				return err
			}
			query := q
			if mode != "search" && !cmd.Flags().Changed("project") {
				query.Project = a.Env("JFLOW_PROJECT")
				if query.Project == "" {
					query.Project = p.DefaultProject
				}
			}
			jql, err := query.Build()
			if err != nil {
				return err
			}
			r, err := reader(ctx, cmd, f, a)
			if err != nil {
				return err
			}
			result, opErr := r.Search(ctx, app.SearchOptions{Query: domain.SearchRequest{JQL: jql, Fields: fields, PageSize: pageSize}, Limit: limit, MaxResults: maxResults, All: all, Refresh: f.refresh, Offline: f.offline, PageToken: pageToken})
			format, _ := cmd.Flags().GetString("format")
			return emit(output.SearchEnvelope(result, opErr), output.Rows(result, format == "table"), opErr)
		}
		f.bind(cmd)
		if mode == "search" {
			cmd.Flags().StringVar(&q.JQL, "jql", "", "Explicit JQL query")
		}
		if mode != "search" {
			cmd.Flags().StringVar(&q.Project, "project", "", "Project by key or name")
			cmd.Flags().StringVar(&q.Category, "status-category", "", "todo, in-progress or done")
			cmd.Flags().StringVar(&q.Type, "type", "", "Issue type")
			cmd.Flags().StringVar(&q.Priority, "priority", "", "Priority")
			cmd.Flags().BoolVar(&q.IncludeDone, "include-done", false, "Include completed in mine")
			cmd.Flags().StringVar(&q.UpdatedSince, "updated-since", "", "Date YYYY-MM-DD or interval like -7d")
			cmd.Flags().StringVar(&q.Sort, "sort", "", "Comma-separated fields; -updated is descending")
		}
		cmd.Flags().IntVar(&limit, "limit", 50, "Maximum issues returned")
		cmd.Flags().IntVar(&pageSize, "page-size", 50, "Page size (1-100)")
		cmd.Flags().IntVar(&maxResults, "max-results", 5000, "Guard limit for --all (1-5000)")
		cmd.Flags().BoolVar(&all, "all", false, "Traverse all pages up to the guard limit")
		cmd.Flags().StringVar(&pageToken, "page-token", "", "Opaque cursor from a previous query")
		cmd.Flags().StringSliceVar(&fields, "fields", append([]string(nil), jiracloud.ListFields...), "List fields: "+strings.Join(jiracloud.ListFields, ","))
		if mode != "search" {
			cmd.PreRunE = func(cmd *cobra.Command, _ []string) error {
				if cmd.Flags().Changed("jql") {
					return &domain.Error{Kind: domain.InvalidInput, Message: "--jql is only available with search."}
				}
				return nil
			}
		}
		root.AddCommand(cmd)
	}
	var f readFlags
	var opts domain.DetailOptions
	opts.IncludeDescription = true
	opts.IncludeSubtasks = true
	opts.IncludeLinks = true
	show := &cobra.Command{Use: "show KEY", Short: "Query an issue and its optional sections", Args: cobra.ExactArgs(1)}
	show.RunE = func(cmd *cobra.Command, args []string) error {
		key, err := domain.IssueKey(args[0])
		if err != nil {
			return err
		}
		ctx, cancel, err := f.context(cmd)
		if err != nil {
			return err
		}
		defer cancel()
		if opts.SectionLimit < 1 || opts.SectionLimit > 5000 || opts.PageSize < 1 || opts.PageSize > 100 || opts.CommentsStart < 0 || opts.HistoryStart < 0 {
			return &domain.Error{Kind: domain.InvalidInput, Message: "Invalid section limits or offsets."}
		}
		if (cmd.Flags().Changed("comments-start") && !opts.IncludeComments) || (cmd.Flags().Changed("history-start") && !opts.IncludeHistory) {
			return &domain.Error{Kind: domain.InvalidInput, Message: "Offsets require --comments or --history, respectively."}
		}
		o := opts
		if o.All && !cmd.Flags().Changed("section-limit") {
			o.SectionLimit = 5000
		}
		a, err := access()
		if err != nil {
			return err
		}
		r, err := reader(ctx, cmd, f, a)
		if err != nil {
			return err
		}
		result, opErr := r.Show(ctx, key, o, f.refresh, f.offline)
		return emit(output.DetailEnvelope(result, opErr), output.DetailText(result), opErr)
	}
	f.bind(show)
	show.Flags().BoolVar(&opts.IncludeComments, "comments", false, "Load paginated comments")
	show.Flags().BoolVar(&opts.IncludeHistory, "history", false, "Load paginated history")
	show.Flags().BoolVar(&opts.All, "all", false, "Traverse all pages of requested sections")
	show.Flags().IntVar(&opts.SectionLimit, "section-limit", 50, "Maximum per section; guard for --all")
	show.Flags().IntVar(&opts.PageSize, "page-size", 50, "Section page size (1-100)")
	show.Flags().IntVar(&opts.CommentsStart, "comments-start", 0, "Comment offset")
	show.Flags().IntVar(&opts.HistoryStart, "history-start", 0, "History offset")
	root.AddCommand(show)
	for _, mode := range []string{"link", "open"} {
		cmd := &cobra.Command{Use: mode + " KEY", Short: map[string]string{"link": "Show issue URL without querying Jira", "open": "Open URL in the default browser"}[mode], Args: cobra.ExactArgs(1)}
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			if _, err := domain.IssueKey(args[0]); err != nil {
				return err
			}
			a, err := access()
			if err != nil {
				return err
			}
			name, key, link, err := a.Link(*profile, args[0])
			if err != nil {
				return err
			}
			data := map[string]any{"key": key, "url": link}
			plain := link
			var opErr error
			if mode == "open" {
				launcher := deps.Browser
				if launcher == nil {
					launcher = browser.New()
				}
				opErr = launcher.Open(cmd.Context(), link)
				data["launcher_started"] = opErr == nil
				if opErr == nil {
					plain = fmt.Sprintf("Launcher started: %s", link)
				}
			}
			envelope := output.Success(data)
			if opErr != nil {
				envelope = output.Failure(opErr)
				envelope.Data = data
			}
			envelope.Meta["profile"] = name
			return emit(envelope, plain, opErr)
		}
		root.AddCommand(cmd)
	}
	// A typed, allowlisted setter makes the F2 default project usable without editing JSON.
	cfg, _, _ := root.Find([]string{"config"})
	if cfg != root {
		set := &cobra.Command{Use: "set KEY VALUE", Short: "Configure default_project for the selected profile", Args: cobra.ExactArgs(2)}
		set.RunE = func(cmd *cobra.Command, args []string) error {
			if args[0] != "default_project" || strings.TrimSpace(args[1]) == "" {
				return &domain.Error{Kind: domain.InvalidInput, Message: "F2 supports config set default_project PROJECT."}
			}
			if _, err := (domain.QueryOptions{Mode: "list", Project: args[1]}).Build(); err != nil {
				return err
			}
			a, err := access()
			if err != nil {
				return err
			}
			name := ""
			err = config.Update(cmd.Context(), a.Path, func(c *config.Config) error {
				n, _, e := c.Select(*profile, a.Env)
				if e != nil {
					return e
				}
				name = n
				stored := c.Profiles[n]
				stored.DefaultProject = args[1]
				c.Profiles[n] = stored
				return nil
			})
			if err != nil {
				return err
			}
			return emit(output.Success(map[string]string{"profile": name, "default_project": args[1]}), "Default project: "+args[1], nil)
		}
		cfg.AddCommand(set)
	}
}
func compact(values []string) []string {
	out := values[:0]
	for _, v := range values {
		if len(out) == 0 || out[len(out)-1] != v {
			out = append(out, v)
		}
	}
	return out
}
