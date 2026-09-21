package cli

import (
	"context"
	"errors"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/provider/jiracloud"
	"jira-flow.local/jflow/internal/tui"
)

func addUI(root *cobra.Command, deps Dependencies, access func() (app.Access, error), reader func(context.Context, *cobra.Command, readFlags, app.Access) (*app.Reader, error)) {
	var f readFlags
	f.timeout = 30 * time.Second
	cmd := &cobra.Command{Use: "ui", Short: "Browse assigned issues interactively (reading preview)", Args: cobra.NoArgs}
	cmd.Flags().BoolVar(&f.offline, "offline", false, "Use cached results without contacting Jira")
	cmd.Flags().BoolVar(&f.refresh, "refresh", false, "Skip cache and query Jira")
	cmd.Flags().DurationVar(&f.timeout, "timeout", 30*time.Second, "Timeout for each read")
	cmd.RunE = func(cmd *cobra.Command, _ []string) error {
		env := deps.Env
		if env == nil {
			env = os.Getenv
		}
		noInput, _ := cmd.Flags().GetBool("no-input")
		input, inOK := cmd.InOrStdin().(*os.File)
		out, outOK := cmd.OutOrStdout().(*os.File)
		if cmd.Flags().Changed("format") || noInput || env("JFLOW_NO_INPUT") == "1" || env("JFLOW_NO_INPUT") == "true" || env("TERM") == "dumb" || !inOK || !outOK || !term.IsTerminal(int(input.Fd())) || !term.IsTerminal(int(out.Fd())) {
			return &domain.Error{Kind: domain.InvalidInput, Message: "ui requires interactive stdin/stdout and a capable terminal, without --format or --no-input. Use jflow mine / show --format plain instead."}
		}
		if f.timeout <= 0 || f.timeout > 10*time.Minute || (f.offline && f.refresh) {
			return &domain.Error{Kind: domain.InvalidInput, Message: "Invalid timeout or incompatible --offline/--refresh."}
		}
		a, err := access()
		if err != nil {
			return err
		}
		setup, cancel := context.WithTimeout(cmd.Context(), f.timeout)
		r, err := reader(setup, cmd, f, a)
		cancel()
		if err != nil {
			return err
		}
		jql, err := (domain.QueryOptions{Mode: "mine"}).Build()
		if err != nil {
			return err
		}
		ctx, stop := context.WithCancel(cmd.Context())
		defer stop()
		model := tui.New(ctx, r, tui.Options{Profile: r.Profile, Timeout: f.timeout, Search: app.SearchOptions{Query: domain.SearchRequest{JQL: jql, Fields: append([]string(nil), jiracloud.ListFields...), PageSize: 50}, Limit: 50, MaxResults: 5000, Refresh: f.refresh, Offline: f.offline}})
		_, err = tea.NewProgram(model, tea.WithContext(ctx), tea.WithInput(input), tea.WithOutput(out)).Run()
		if err != nil {
			if errors.Is(err, tea.ErrProgramKilled) || ctx.Err() != nil {
				return &domain.Error{Kind: domain.Canceled, Message: "Interactive session canceled.", Cause: err}
			}
			return &domain.Error{Kind: domain.InvalidInput, Message: "Could not run the interactive terminal.", Cause: err}
		}
		return nil
	}
	root.AddCommand(cmd)
}
