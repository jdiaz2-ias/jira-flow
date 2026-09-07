package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"jira-flow.local/jflow/internal/domain"
)

type fakeWorkflow struct {
	issue                            domain.IssueDetail
	transitions                      []domain.Transition
	reads, lists, writes, identities int
	requests                         []domain.TransitionRequest
	identityErr, errorOnWrite        error
	writeState                       domain.ApplyState
	afterRead                        func(int, *domain.IssueDetail)
	afterList                        func(int, []domain.Transition) []domain.Transition
	observedFields                   map[string]any
	leaveState                       bool
}

func (f *fakeWorkflow) Myself(context.Context) (domain.User, error) {
	f.identities++
	return domain.User{ID: "user"}, f.identityErr
}
func (f *fakeWorkflow) Search(context.Context, domain.SearchRequest) (domain.IssuePage, error) {
	return domain.IssuePage{}, fmt.Errorf("unused")
}
func (f *fakeWorkflow) GetIssue(_ context.Context, _ domain.IssueRef, _ domain.DetailOptions) (domain.IssueDetail, error) {
	f.reads++
	copy := f.issue
	if f.afterRead != nil {
		f.afterRead(f.reads, &copy)
	}
	return copy, nil
}
func (f *fakeWorkflow) ListTransitions(context.Context, domain.IssueRef) ([]domain.Transition, error) {
	f.lists++
	if f.afterList != nil {
		return f.afterList(f.lists, append([]domain.Transition(nil), f.transitions...)), nil
	}
	return f.transitions, nil
}
func (f *fakeWorkflow) ApplyTransition(_ context.Context, q domain.TransitionRequest) (domain.ApplyResult, error) {
	f.writes++
	f.requests = append(f.requests, q)
	state := f.writeState
	if state == "" {
		state = domain.ApplyAcceptedUnverified
	}
	if !f.leaveState && state != domain.ApplyFailed {
		for _, t := range f.transitions {
			if t.ID == q.TransitionID {
				f.issue.Issue.Status = t.To
			}
		}
		f.issue.Values = q.Fields
		if f.observedFields != nil {
			f.issue.Values = f.observedFields
		}
	}
	return domain.ApplyResult{State: state, Issue: q.Issue, TransitionID: q.TransitionID}, f.errorOnWrite
}
func workflowFixture() (*Workflow, *fakeWorkflow) {
	f := &fakeWorkflow{issue: domain.IssueDetail{Issue: domain.Issue{Ref: domain.IssueRef{ID: "i-1", Key: "APP-1"}, ProjectID: "p-1", IssueTypeID: "type-1", Status: domain.Status{ID: "todo", Name: "To Do", Category: domain.CategoryTodo}, UpdatedAt: time.Date(2026, 9, 7, 10, 0, 0, 0, time.UTC)}}, transitions: []domain.Transition{{ID: "begin", Name: "Begin review", To: domain.Status{ID: "active", Name: "Investigating", Category: domain.CategoryInProgress}, Fields: []domain.FieldSpec{}}, {ID: "resolve", Name: "Resolve", To: domain.Status{ID: "resolved", Name: "Resolved", Category: domain.CategoryDone}, Fields: []domain.FieldSpec{{ID: "resolution", Type: "resolution", Required: true, AllowedValues: []domain.NamedID{{ID: "fixed", Name: "Fixed"}}}}}, {ID: "cancel", Name: "Cancel", To: domain.Status{ID: "cancelled", Name: "Cancelled", Category: domain.CategoryDone}, Fields: []domain.FieldSpec{}}}}
	return &Workflow{Reader: f, Gateway: f, Profile: "work", Site: "https://example.atlassian.net", ExpectedAccount: "user", Sleep: func(context.Context, time.Duration) error { return nil }}, f
}
func TestWorkflowPrepareAmbiguityFieldsNoopAndExplicitLoops(t *testing.T) {
	w, f := workflowFixture()
	ctx := context.Background()
	p, e := w.Prepare(ctx, PrepareOptions{Key: "APP-1", Intent: domain.IntentDone})
	if domain.ExitCode(e) != 6 || len(p.Candidates) != 2 || f.writes != 0 {
		t.Fatal(p, e)
	}
	p, e = w.Prepare(ctx, PrepareOptions{Key: "APP-1", Intent: domain.IntentDone, TransitionID: "resolve"})
	if domain.ExitCode(e) != 6 || len(p.Missing) != 1 {
		t.Fatal(p, e)
	}
	p, e = w.Prepare(ctx, PrepareOptions{Key: "APP-1", Intent: domain.IntentStart, TransitionID: "resolve"})
	if domain.ExitCode(e) != 6 {
		t.Fatal(p, e)
	}
	f.issue.Issue.Status = domain.Status{ID: "active", Category: domain.CategoryInProgress}
	p, e = w.Prepare(ctx, PrepareOptions{Key: "APP-1", Intent: domain.IntentStart})
	if e != nil || !p.Noop {
		t.Fatal(p, e)
	}
	result, e := w.Apply(ctx, p)
	if e != nil || result.Apply.State != domain.ApplyNoop || f.writes != 0 {
		t.Fatal(result, e)
	}
	p, e = w.Prepare(ctx, PrepareOptions{Key: "APP-1", Intent: domain.IntentStart, TransitionID: "begin"})
	if e != nil || p.Noop {
		t.Fatal("explicit loop must remain executable", p, e)
	}
	p, e = w.Prepare(ctx, PrepareOptions{Key: "APP-1", Intent: domain.IntentClose})
	if domain.ExitCode(e) != 6 {
		t.Fatal("close guessed", p, e)
	}
}
func TestWorkflowRulesAreExactAndNeverSilentlyFallBack(t *testing.T) {
	w, f := workflowFixture()
	w.Rules = []domain.WorkflowRule{{ProjectID: "p-1", IssueTypeID: "type-1", Intent: domain.IntentDone, FromStatusID: "todo", TransitionID: "resolve", ExpectedToStatusID: "resolved"}}
	o := PrepareOptions{Key: "APP-1", Intent: domain.IntentDone, Fields: map[string]any{"resolution": map[string]any{"id": "fixed"}}}
	p, e := w.Prepare(context.Background(), o)
	if e != nil || p.Selected.ID != "resolve" {
		t.Fatal(p, e)
	}
	w.Rules[0].ExpectedToStatusID = "different"
	if _, e = w.Prepare(context.Background(), o); domain.ExitCode(e) != 7 {
		t.Fatal(e)
	}
	w.Rules[0].ExpectedToStatusID = "resolved"
	f.issue.Issue.Status = domain.Status{ID: "resolved", Category: domain.CategoryDone}
	o.Fields = nil
	p, e = w.Prepare(context.Background(), o)
	if e != nil || !p.Noop {
		t.Fatal(p, e)
	}
	f.issue.Issue.Status.ID = "cancelled"
	if _, e = w.Prepare(context.Background(), o); domain.ExitCode(e) != 7 {
		t.Fatal("cancelled incorrectly satisfied mapped resolution", e)
	}
}
func TestWorkflowRevalidationPreventsRacesAndTampering(t *testing.T) {
	for _, mode := range []string{"status", "updated", "metadata", "removed", "tamper"} {
		t.Run(mode, func(t *testing.T) {
			w, f := workflowFixture()
			p, e := w.Prepare(context.Background(), PrepareOptions{Key: "APP-1", Intent: domain.IntentStart})
			if e != nil {
				t.Fatal(e)
			}
			switch mode {
			case "status":
				f.issue.Issue.Status.ID = "other"
			case "updated":
				f.issue.Issue.UpdatedAt = f.issue.Issue.UpdatedAt.Add(time.Second)
			case "metadata":
				f.transitions[0].Fields = []domain.FieldSpec{{ID: "newRequired", Type: "string", Required: true}}
			case "removed":
				f.transitions = f.transitions[1:]
			case "tamper":
				p.Fields["summary"] = "unapproved"
			}
			_, e = w.Apply(context.Background(), p)
			if domain.ExitCode(e) != 7 || f.writes != 0 {
				t.Fatal(e, f.writes)
			}
		})
	}
}
func TestWorkflowSendsOnceAndVerifiesFields(t *testing.T) {
	w, f := workflowFixture()
	invalidations := 0
	w.Invalidate = func() { invalidations++ }
	p, e := w.Prepare(context.Background(), PrepareOptions{Key: "APP-1", Intent: domain.IntentDone, TransitionID: "resolve", Fields: map[string]any{"resolution": map[string]any{"id": "fixed"}}})
	if e != nil {
		t.Fatal(e)
	}
	result, e := w.Apply(context.Background(), p)
	if e != nil || result.Apply.State != domain.ApplyVerified || f.writes != 1 || invalidations != 1 || f.lists != 2 || f.identities != 2 || f.reads != 3 {
		t.Fatal(result, e, f)
	}
	if len(f.requests[0].Fields) != 1 {
		t.Fatal("unexpected fields")
	}
}
func TestWorkflowUnknownAndAcceptedUnverifiedNeverResend(t *testing.T) {
	for _, mode := range []string{"unknown", "different-state", "different-field", "rejected"} {
		t.Run(mode, func(t *testing.T) {
			w, f := workflowFixture()
			switch mode {
			case "unknown":
				f.writeState = domain.ApplyUnknown
				f.errorOnWrite = problem(domain.Uncertain, "transport lost")
			case "different-state":
				f.leaveState = true
			case "different-field":
				f.observedFields = map[string]any{"resolution": map[string]any{"id": "cancelled"}}
			case "rejected":
				f.writeState = domain.ApplyFailed
				f.errorOnWrite = problem(domain.Forbidden, "denied")
			}
			p, e := w.Prepare(context.Background(), PrepareOptions{Key: "APP-1", Intent: domain.IntentDone, TransitionID: "resolve", Fields: map[string]any{"resolution": map[string]any{"id": "fixed"}}})
			if e != nil {
				t.Fatal(e)
			}
			r, e := w.Apply(context.Background(), p)
			want := 9
			if mode == "rejected" {
				want = 4
			}
			if domain.ExitCode(e) != want || f.writes != 1 {
				t.Fatal(r, e, f.writes)
			}
			if mode == "unknown" && r.Apply.State != domain.ApplyUnknown {
				t.Fatal("observation promoted an unknown send", r)
			}
			if (mode == "different-field" || mode == "different-state") && r.Apply.State != domain.ApplyAcceptedUnverified {
				t.Fatal(r)
			}
		})
	}
}
