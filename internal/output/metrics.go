package output

import (
	"fmt"
	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/domain"
	"strings"
	"time"
)

func ProgressEnvelope(r app.DetailResult, m domain.ProgressMetrics, zone string, err error) Envelope {
	e := DetailEnvelope(r, err)
	if r.Detail.Issue.Ref.ID == "" {
		return e
	}
	d := r.Detail
	e.Data = map[string]any{
		"issue":       IssueData(d.Issue),
		"subtasks":    map[string]any{"done": m.Done, "visible": m.Visible, "complete": m.Complete, "percent": m.Percent, "scope": "visible_subtasks"},
		"time":        map[string]any{"spent_seconds": d.Time.Spent, "remaining_seconds": d.Time.Remaining, "original_estimate_seconds": d.Time.Original, "estimate_consumed_percent": m.EstimateConsumedPercent, "scope": "issue_only"},
		"state_since": nil, "due_in_days": m.DueInDays, "timezone": zone,
	}
	if m.StateSince != nil {
		e.Data.(map[string]any)["state_since"] = timestamp(*m.StateSince)
	}
	e.Meta["method"] = "jira_fields_and_visible_subtasks"
	return e
}
func freshness(m app.ReadMeta) string {
	return fmt.Sprintf("Source: %s · captured %s · stale: %t", m.Source, m.FetchedAt.Format(time.RFC3339), m.Stale)
}
func ProgressText(r app.DetailResult, m domain.ProgressMetrics, zone string) string {
	if r.Detail.Issue.Ref.ID == "" {
		return ""
	}
	d := r.Detail
	var b strings.Builder
	fmt.Fprintf(&b, "%s  %s\nStatus: %s (%s)\n", d.Issue.Ref.Key, domain.CleanText(d.Issue.Summary, false), domain.CleanText(d.Issue.Status.Name, false), d.Issue.Status.Category)
	resolution := "No resolution"
	if d.Issue.Resolution != nil {
		resolution = domain.CleanText(d.Issue.Resolution.Name, false)
	}
	fmt.Fprintf(&b, "Resolution: %s\n", resolution)
	if m.Complete && m.Visible == 0 {
		b.WriteString("No subtasks.\n")
	} else {
		fmt.Fprintf(&b, "Visible subtasks: %d done of %d loaded", m.Done, m.Visible)
		if m.Percent != nil {
			fmt.Fprintf(&b, " · %.1f%%", *m.Percent)
		} else {
			b.WriteString(" · percentage unknown")
		}
		b.WriteByte('\n')
	}
	if d.Issue.Status.Category == domain.CategoryDone && m.Done < m.Visible {
		b.WriteString("Warning: parent is Done but visible subtasks remain unfinished or unknown.\n")
	}
	for _, metric := range []struct {
		name  string
		value *int64
	}{{"Logged time", d.Time.Spent}, {"Remaining estimate", d.Time.Remaining}, {"Original estimate", d.Time.Original}} {
		if metric.value == nil {
			fmt.Fprintf(&b, "%s: unknown\n", metric.name)
		} else {
			seconds := *metric.value
			fmt.Fprintf(&b, "%s: %dh %dm %ds (issue only)\n", metric.name, seconds/3600, seconds%3600/60, seconds%60)
		}
	}
	if m.EstimateConsumedPercent != nil {
		fmt.Fprintf(&b, "Estimate consumed: %.1f%%\n", *m.EstimateConsumedPercent)
	}
	if d.Issue.UpdatedAt.IsZero() {
		b.WriteString("Updated: unknown\n")
	} else {
		fmt.Fprintf(&b, "Updated: %s\n", d.Issue.UpdatedAt.Format(time.RFC3339))
	}
	if m.StateSince == nil {
		b.WriteString("In current state since: unknown (requires complete status history)\n")
	} else {
		fmt.Fprintf(&b, "In current state since: %s\n", m.StateSince.Format(time.RFC3339))
	}
	if m.DueInDays != nil {
		switch {
		case *m.DueInDays == 0:
			fmt.Fprintf(&b, "Due today (%s)\n", zone)
		case *m.DueInDays < 0:
			fmt.Fprintf(&b, "Overdue %d days (%s)\n", -*m.DueInDays, zone)
		default:
			fmt.Fprintf(&b, "Due in %d days (%s)\n", *m.DueInDays, zone)
		}
	} else {
		b.WriteString("Due date: unknown\n")
	}
	fmt.Fprintln(&b, freshness(r.Meta))
	for _, warning := range d.Warnings {
		fmt.Fprintf(&b, "Warning: %s\n", domain.CleanText(warning, false))
	}
	return strings.TrimSpace(b.String())
}
func SummaryEnvelope(r app.SearchResult, scope string, includesDone bool, err error) Envelope {
	e := SearchEnvelope(r, err)
	m := domain.MeasureSummary(r.Issues, r.Meta.Complete, includesDone)
	e.Data = map[string]any{"scope": scope, "processed": m.Processed, "counts": map[string]int{"todo": m.Todo, "in_progress": m.InProgress, "done": m.Done, "unknown": m.Unknown}, "completed_percent": m.Percent, "includes_done": includesDone}
	e.Meta["method"] = "visible_issues_by_status_category"
	return e
}
func SummaryText(r app.SearchResult, scope string, includesDone bool) string {
	m := domain.MeasureSummary(r.Issues, r.Meta.Complete, includesDone)
	var b strings.Builder
	fmt.Fprintf(&b, "Scope: %s\nProcessed: %d visible issues\nTo do: %d\nIn progress: %d\nDone: %d\nUnknown: %d\n", domain.CleanText(scope, false), m.Processed, m.Todo, m.InProgress, m.Done, m.Unknown)
	if m.Percent != nil {
		fmt.Fprintf(&b, "Completed: %.1f%% (Done includes all resolutions)\n", *m.Percent)
	} else {
		b.WriteString("Completed percentage: unknown or not applicable to this scope.\n")
	}
	if !r.Meta.Complete {
		b.WriteString("Incomplete result; global total unknown.\n")
	}
	fmt.Fprintln(&b, freshness(r.Meta))
	for _, warning := range r.Warnings {
		fmt.Fprintf(&b, "Warning: %s\n", domain.CleanText(warning, false))
	}
	return strings.TrimSpace(b.String())
}
