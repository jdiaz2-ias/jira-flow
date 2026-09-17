package domain

import (
	"testing"
	"time"
)

func number(n int64) *int64 { return &n }
func TestProgressUnknownPartialAndOverEstimate(t *testing.T) {
	d := IssueDetail{SubtasksComplete: true, Time: TimeTracking{Spent: number(5400), Original: number(3600), Remaining: number(0)}}
	m := MeasureProgress(d, time.Now(), time.UTC)
	if m.Percent != nil || !m.Complete || m.EstimateConsumedPercent == nil || *m.EstimateConsumedPercent != 150 {
		t.Fatal(m)
	}
	d.Subtasks = []Issue{{Status: Status{Category: CategoryDone}}, {Status: Status{Category: CategoryTodo}}}
	m = MeasureProgress(d, time.Now(), time.UTC)
	if m.Percent == nil || *m.Percent != 50 || m.Done != 1 {
		t.Fatal(m)
	}
	for _, kind := range []string{"partial", "unknown"} {
		copy := d
		if kind == "partial" {
			copy.SubtasksComplete = false
		} else {
			copy.Subtasks = append([]Issue{}, d.Subtasks...)
			copy.Subtasks[1].Status.Category = CategoryUnknown
		}
		if m := MeasureProgress(copy, time.Now(), time.UTC); m.Percent != nil || m.Complete {
			t.Fatal(kind, m)
		}
	}
	d.Time.Original = number(0)
	if MeasureProgress(d, time.Now(), time.UTC).EstimateConsumedPercent != nil {
		t.Fatal("zero denominator")
	}
}
func TestDueDateUsesLocalCalendarAcrossDST(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 3, 8, 23, 30, 0, 0, loc)
	d := IssueDetail{Issue: Issue{DueDate: &LocalDate{2026, time.March, 9}}}
	m := MeasureProgress(d, now, loc)
	if m.DueInDays == nil || *m.DueInDays != 1 {
		t.Fatal(m)
	}
	if got := MeasureProgress(d, now, time.UTC); got.DueInDays == nil || *got.DueInDays != 0 {
		t.Fatal(got)
	}
}
func TestStateSinceRequiresCompleteHistoryAndMatchingID(t *testing.T) {
	at := time.Date(2026, 9, 1, 10, 0, 0, 0, time.UTC)
	d := IssueDetail{Issue: Issue{Status: Status{ID: "active"}}, History: &HistoryPage{PageInfo: PageInfo{Complete: true}, Items: []HistoryEntry{{CreatedAt: at, Changes: []Change{{FieldID: "status", ToID: "active"}}}}}}
	if m := MeasureProgress(d, at.Add(time.Hour), time.UTC); m.StateSince == nil || !m.StateSince.Equal(at) {
		t.Fatal(m)
	}
	d.History.Complete = false
	if MeasureProgress(d, at.Add(time.Hour), time.UTC).StateSince != nil {
		t.Fatal("partial history")
	}
	d.History.Complete = true
	d.History.StartAt = 1
	if MeasureProgress(d, at.Add(time.Hour), time.UTC).StateSince != nil {
		t.Fatal("tail-only history")
	}
	d.History.StartAt = 0
	d.Issue.Status.ID = "other"
	if MeasureProgress(d, at.Add(time.Hour), time.UTC).StateSince != nil {
		t.Fatal("raced status")
	}
}
func TestSummaryPercentRequiresCompleteKnownPopulation(t *testing.T) {
	issues := []Issue{{Status: Status{Category: CategoryDone}}, {Status: Status{Category: CategoryTodo}}}
	if m := MeasureSummary(issues, true, true); m.Percent == nil || *m.Percent != 50 {
		t.Fatal(m)
	}
	for _, m := range []SummaryMetrics{MeasureSummary(issues, false, true), MeasureSummary(issues, true, false), MeasureSummary(nil, true, true), MeasureSummary(append(issues, Issue{}), true, true)} {
		if m.Percent != nil {
			t.Fatal(m)
		}
	}
}
