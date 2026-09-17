package domain

import "time"

type ProgressMetrics struct {
	Done, Visible                    int
	Complete                         bool
	Percent, EstimateConsumedPercent *float64
	StateSince                       *time.Time
	DueInDays                        *int
}

// MeasureProgress uses the visible set only. A missing status also makes the
// numerator unknown. Time consumption is not a measure of work completion.
func MeasureProgress(d IssueDetail, now time.Time, loc *time.Location) ProgressMetrics {
	m := ProgressMetrics{Visible: len(d.Subtasks), Complete: d.SubtasksComplete}
	for _, s := range d.Subtasks {
		switch s.Status.Category {
		case CategoryDone:
			m.Done++
		case CategoryTodo, CategoryInProgress:
		default:
			m.Complete = false
		}
	}
	if m.Complete && m.Visible > 0 {
		v := 100 * float64(m.Done) / float64(m.Visible)
		m.Percent = &v
	}
	if d.Time.Spent != nil && *d.Time.Spent >= 0 && d.Time.Original != nil && *d.Time.Original > 0 {
		v := 100 * float64(*d.Time.Spent) / float64(*d.Time.Original)
		m.EstimateConsumedPercent = &v
	}
	if h := d.History; h != nil && h.Complete && h.StartAt == 0 {
		var latest time.Time
		var to string
		valid := true
		for _, entry := range h.Items {
			for _, change := range entry.Changes {
				if change.FieldID == "status" || (change.FieldID == "" && change.Field == "status") {
					if entry.CreatedAt.IsZero() || entry.CreatedAt.After(now) {
						valid = false
					}
					if entry.CreatedAt.After(latest) {
						latest = entry.CreatedAt
						to = change.ToID
					}
				}
			}
		}
		if valid && !latest.IsZero() && to != "" && to == d.Issue.Status.ID {
			m.StateSince = &latest
		}
	}
	if date := d.Issue.DueDate; date != nil {
		if loc == nil {
			loc = time.Local
		}
		today := now.In(loc)
		// UTC here is calendar arithmetic, after resolving the local date; it
		// avoids dividing a 23/25-hour DST day into 24-hour periods.
		start := time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, time.UTC)
		due := time.Date(date.Year, date.Month, date.Day, 0, 0, 0, 0, time.UTC)
		days := int(due.Sub(start).Hours() / 24)
		m.DueInDays = &days
	}
	return m
}

type SummaryMetrics struct {
	Todo, InProgress, Done, Unknown, Processed int
	Percent                                    *float64
}

func MeasureSummary(issues []Issue, complete, includesDone bool) SummaryMetrics {
	m := SummaryMetrics{Processed: len(issues)}
	for _, i := range issues {
		switch i.Status.Category {
		case CategoryTodo:
			m.Todo++
		case CategoryInProgress:
			m.InProgress++
		case CategoryDone:
			m.Done++
		default:
			m.Unknown++
		}
	}
	if complete && includesDone && m.Unknown == 0 && m.Processed > 0 {
		p := 100 * float64(m.Done) / float64(m.Processed)
		m.Percent = &p
	}
	return m
}
