package cli

import (
	"context"
	"github.com/spf13/cobra"
	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/output"
	"jira-flow.local/jflow/internal/provider/jiracloud"
	"time"
)

type readerFactory func(context.Context, *cobra.Command, readFlags, app.Access) (*app.Reader, error)

func addMetrics(root *cobra.Command, access func() (app.Access, error), reader readerFactory, emit func(output.Envelope, string, error) error) {
	var f readFlags
	var history bool
	var zone string
	var historyLimit int
	progress := &cobra.Command{Use: "progress KEY", Short: "Show measured progress, time and due date", Args: cobra.ExactArgs(1)}
	progress.RunE = func(cmd *cobra.Command, args []string) error {
		key, err := domain.IssueKey(args[0])
		if err != nil {
			return err
		}
		loc := time.Local
		if zone != "" {
			loc, err = time.LoadLocation(zone)
			if err != nil {
				return &domain.Error{Kind: domain.InvalidInput, Message: "Unknown timezone; use an IANA name such as America/Monterrey."}
			}
		}
		if historyLimit < 1 || historyLimit > 5000 {
			return &domain.Error{Kind: domain.InvalidInput, Message: "--history-limit must be between 1 and 5000."}
		}
		ctx, cancel, err := f.context(cmd)
		if err != nil {
			return err
		}
		defer cancel()
		a, err := access()
		if err != nil {
			return err
		}
		r, err := reader(ctx, cmd, f, a)
		if err != nil {
			return err
		}
		o := domain.DetailOptions{IncludeSubtasks: true, IncludeHistory: history, All: history, SectionLimit: historyLimit, PageSize: 100}
		result, opErr := r.Show(ctx, key, o, f.refresh, f.offline)
		m := domain.MeasureProgress(result.Detail, time.Now(), loc)
		return emit(output.ProgressEnvelope(result, m, loc.String(), opErr), output.ProgressText(result, m, loc.String()), opErr)
	}
	f.bind(progress)
	progress.Flags().BoolVar(&history, "history", false, "Read complete status history to determine time in state")
	progress.Flags().IntVar(&historyLimit, "history-limit", 5000, "Guard limit for status history (1-5000)")
	progress.Flags().StringVar(&zone, "timezone", "", "IANA timezone for due dates (default: system local)")
	root.AddCommand(progress)

	var sf readFlags
	var q domain.QueryOptions
	var maxResults, pageSize int
	summary := &cobra.Command{Use: "summary", Short: "Count visible issues by category over a stated scope", Args: cobra.NoArgs}
	summary.RunE = func(cmd *cobra.Command, _ []string) error {
		ctx, cancel, err := sf.context(cmd)
		if err != nil {
			return err
		}
		defer cancel()
		query := q
		query.Mode = "mine"
		if cmd.Flags().Changed("project") {
			query.Mode = "list"
		}
		if cmd.Flags().Changed("jql") {
			for _, flag := range []string{"project", "status-category", "type", "priority", "updated-since", "include-done"} {
				if cmd.Flags().Changed(flag) {
					return &domain.Error{Kind: domain.InvalidInput, Message: "Do not combine summary --jql with structured filters."}
				}
			}
			query.Mode = "search"
		}
		jql, err := query.Build()
		if err != nil {
			return err
		}
		if maxResults < 1 || maxResults > 5000 || pageSize < 1 || pageSize > 100 {
			return &domain.Error{Kind: domain.InvalidInput, Message: "Invalid summary limits: max-results 1-5000, page-size 1-100."}
		}
		a, err := access()
		if err != nil {
			return err
		}
		r, err := reader(ctx, cmd, sf, a)
		if err != nil {
			return err
		}
		result, opErr := r.Search(ctx, app.SearchOptions{Query: domain.SearchRequest{JQL: jql, Fields: append([]string(nil), jiracloud.ListFields...), PageSize: pageSize}, All: true, MaxResults: maxResults, Refresh: sf.refresh, Offline: sf.offline})
		if !result.Meta.Complete {
			// Summary is one bounded traversal, not a resumable page command.
			result.Meta.NextPageToken = ""
			for i, warning := range result.Warnings {
				if warning == "Incomplete result; use next_page_token with --page-token to continue." {
					result.Warnings[i] = "Summary is incomplete; narrow the scope or increase --max-results."
				}
			}
		}
		// Arbitrary JQL cannot prove the population includes completed work;
		// counts remain useful but no global percentage is inferred from it.
		includesDone := query.Mode != "search" && query.Category == "" && (query.Mode == "list" || query.IncludeDone)
		return emit(output.SummaryEnvelope(result, jql, includesDone, opErr), output.SummaryText(result, jql, includesDone), opErr)
	}
	sf.bind(summary)
	summary.Flags().StringVar(&q.JQL, "jql", "", "Explicit JQL (counts only; no inferred completion percentage)")
	summary.Flags().StringVar(&q.Project, "project", "", "Summarize an explicit project instead of my assigned issues")
	summary.Flags().StringVar(&q.Category, "status-category", "", "todo, in-progress or done")
	summary.Flags().StringVar(&q.Type, "type", "", "Issue type")
	summary.Flags().StringVar(&q.Priority, "priority", "", "Priority")
	summary.Flags().StringVar(&q.UpdatedSince, "updated-since", "", "Date YYYY-MM-DD or interval like -7d")
	summary.Flags().BoolVar(&q.IncludeDone, "include-done", false, "Include completed issues in the personal scope")
	summary.Flags().IntVar(&maxResults, "max-results", 5000, "Guard limit; incomplete traversal returns exit 10")
	summary.Flags().IntVar(&pageSize, "page-size", 100, "Search page size (1-100)")
	root.AddCommand(summary)
}
