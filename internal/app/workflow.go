package app

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/ports"
)

type Workflow struct {
	Reader                         ports.IssueReader
	Gateway                        ports.TransitionGateway
	Profile, Site, ExpectedAccount string
	Rules                          []domain.WorkflowRule
	Invalidate                     func()
	Sleep                          func(context.Context, time.Duration) error
}
type Catalog struct {
	Issue       domain.IssueDetail
	Transitions []domain.Transition
}
type PrepareOptions struct {
	Key          string
	Intent       domain.Intent
	TransitionID string
	Fields       map[string]domain.FieldValue
}
type Preparation struct {
	Catalog       Catalog
	Intent        domain.Intent
	Selected      *domain.Transition
	Fields        map[string]domain.FieldValue
	Missing       []domain.FieldSpec
	Candidates    []domain.Transition
	Noop          bool
	Profile, Site string
	seal          string
}
type WorkflowResult struct {
	Attempted bool
	Plan      Preparation
	Apply     domain.ApplyResult
	Warnings  []string
}

func (w *Workflow) identity(ctx context.Context) error {
	u, e := w.Reader.Myself(ctx)
	if e != nil {
		return e
	}
	if w.ExpectedAccount != "" && w.ExpectedAccount != u.ID {
		return problem(domain.Authentication, "The credential belongs to another identity; run auth login again.")
	}
	return nil
}
func (w *Workflow) Inspect(ctx context.Context, key string) (Catalog, error) {
	var c Catalog
	key, e := domain.IssueKey(key)
	if e != nil {
		return c, e
	}
	if e = w.identity(ctx); e != nil {
		return c, e
	}
	c.Issue, e = w.Reader.GetIssue(ctx, domain.IssueRef{Key: key}, domain.DetailOptions{})
	if e != nil {
		return c, e
	}
	if c.Issue.Issue.Ref.ID == "" || c.Issue.Issue.Status.ID == "" {
		return c, problem(domain.Unavailable, "Jira omitted the issue identity or current state.")
	}
	c.Transitions, e = w.Gateway.ListTransitions(ctx, c.Issue.Issue.Ref)
	return c, e
}
func (w *Workflow) Prepare(ctx context.Context, o PrepareOptions) (Preparation, error) {
	p := Preparation{Intent: o.Intent, Fields: map[string]domain.FieldValue{}, Missing: []domain.FieldSpec{}, Candidates: []domain.Transition{}, Profile: w.Profile, Site: w.Site}
	if o.Intent != "" && o.Intent != domain.IntentStart && o.Intent != domain.IntentDone && o.Intent != domain.IntentClose && o.Intent != domain.IntentReopen {
		return p, problem(domain.InvalidInput, "Invalid workflow intent.")
	}
	b, e := json.Marshal(o.Fields)
	if e != nil || len(b) > 1024*1024 {
		return p, problem(domain.InvalidInput, "Fields must be valid JSON of at most 1 MiB.")
	}
	if o.Fields != nil {
		if e = json.Unmarshal(b, &p.Fields); e != nil {
			return p, problem(domain.InvalidInput, "Invalid fields object.")
		}
	}
	p.Catalog, e = w.Inspect(ctx, o.Key)
	if e != nil {
		return p, e
	}
	issue := p.Catalog.Issue.Issue
	var exact *domain.WorkflowRule
	hasFamily, atMappedTarget := false, false
	for _, rule := range w.Rules {
		if rule.ProjectID == issue.ProjectID && rule.IssueTypeID == issue.IssueTypeID && rule.Intent == o.Intent {
			hasFamily = true
			if rule.ExpectedToStatusID == issue.Status.ID {
				atMappedTarget = true
			}
			if rule.FromStatusID == issue.Status.ID {
				copy := rule
				exact = &copy
			}
		}
	}
	selectedID := o.TransitionID
	if selectedID == "" && exact != nil {
		selectedID = exact.TransitionID
	}
	if o.TransitionID == "" {
		if exact != nil && issue.Status.ID == exact.ExpectedToStatusID {
			p.Noop = true
		} else if exact == nil && atMappedTarget {
			p.Noop = true
		} else if !hasFamily && ((o.Intent == domain.IntentStart && issue.Status.Category == domain.CategoryInProgress) || (o.Intent == domain.IntentDone && issue.Status.Category == domain.CategoryDone)) {
			p.Noop = true
		}
		if p.Noop {
			if len(p.Fields) > 0 {
				return p, problem(domain.Validation, "This intent is already satisfied; use an explicit transition to apply fields or loop effects.")
			}
			p.seal = w.seal(p)
			return p, nil
		}
		if exact == nil && hasFamily {
			return p, problem(domain.Conflict, "No workflow rule applies to this source state. Review the mapping; no fallback was selected.")
		}
	}
	for _, t := range p.Catalog.Transitions {
		if selectedID != "" {
			if t.ID == selectedID {
				copy := t
				p.Selected = &copy
			}
			continue
		}
		eligible := false
		switch o.Intent {
		case domain.IntentStart:
			eligible = t.To.Category == domain.CategoryInProgress && t.To.ID != issue.Status.ID
		case domain.IntentDone:
			eligible = t.To.Category == domain.CategoryDone
		default:
			eligible = true
		}
		if eligible {
			p.Candidates = append(p.Candidates, t)
		}
	}
	if selectedID != "" {
		if p.Selected == nil {
			return p, problem(domain.Conflict, "The requested or mapped transition is not currently available.")
		}
		if o.TransitionID == "" && exact != nil && p.Selected.To.ID != exact.ExpectedToStatusID {
			return p, problem(domain.Conflict, "The mapped transition now has a different destination; review the rule.")
		}
		if (o.Intent == domain.IntentStart && p.Selected.To.Category != domain.CategoryInProgress) || (o.Intent == domain.IntentDone && p.Selected.To.Category != domain.CategoryDone) {
			return p, problem(domain.Validation, "The transition destination does not satisfy the requested intent.")
		}
	}
	if p.Selected == nil {
		if len(p.Candidates) == 1 && (o.Intent == domain.IntentStart || o.Intent == domain.IntentDone) {
			copy := p.Candidates[0]
			p.Selected = &copy
		} else if len(p.Candidates) > 0 {
			return p, &domain.Error{Kind: domain.TransitionAmbiguous, Message: "Choose an explicit transition ID; --yes never resolves ambiguity.", Details: map[string]any{"candidate_transition_ids": candidateIDs(p.Candidates)}}
		} else {
			return p, &domain.Error{Kind: domain.Unsupported, Message: "No direct transition satisfies this intent; no multi-step path was attempted.", Details: map[string]any{"available_transition_ids": candidateIDs(p.Catalog.Transitions)}}
		}
	}
	if e = domain.ValidateFields(p.Selected.Fields, p.Fields, false); e != nil {
		return p, e
	}
	p.Missing = domain.MissingFields(p.Selected.Fields, p.Fields)
	if e = domain.ValidateFields(p.Selected.Fields, p.Fields, true); e != nil {
		return p, e
	}
	p.seal = w.seal(p)
	return p, nil
}
func candidateIDs(ts []domain.Transition) []string {
	ids := []string{}
	for _, t := range ts {
		ids = append(ids, t.ID)
	}
	return ids
}
func issueSignature(i domain.Issue) string {
	return hash([]any{i.Ref.ID, i.ProjectID, i.IssueTypeID, i.Status.ID, i.UpdatedAt})
}
func transitionSignature(t domain.Transition) string {
	copy := t
	copy.Fields = append([]domain.FieldSpec(nil), t.Fields...)
	sort.Slice(copy.Fields, func(i, j int) bool { return copy.Fields[i].ID < copy.Fields[j].ID })
	return hash(copy)
}
func (w *Workflow) seal(p Preparation) string {
	var selected string
	if p.Selected != nil {
		selected = transitionSignature(*p.Selected)
	}
	return hash([]any{w.Profile, w.Site, w.ExpectedAccount, p.Profile, p.Site, issueSignature(p.Catalog.Issue.Issue), p.Intent, selected, p.Fields, p.Noop})
}
func (w *Workflow) Apply(ctx context.Context, p Preparation) (WorkflowResult, error) {
	issue := p.Catalog.Issue.Issue
	r := WorkflowResult{Plan: p, Apply: domain.ApplyResult{State: domain.ApplyFailed, Issue: issue.Ref, Before: &issue.Status}, Warnings: []string{}}
	if p.seal == "" || p.seal != w.seal(p) {
		return r, problem(domain.Conflict, "The prepared action changed; prepare and confirm it again.")
	}
	if p.Noop {
		r.Apply.State = domain.ApplyNoop
		r.Apply.Observed = &issue.Status
		return r, nil
	}
	if p.Selected == nil {
		return r, problem(domain.InvalidInput, "A transition must be selected.")
	}
	r.Apply.TransitionID = p.Selected.ID
	// Revalidate even in automation: no cached state or old metadata authorizes a write.
	fresh, e := w.Inspect(ctx, issue.Ref.Key)
	if e != nil {
		return r, e
	}
	if issueSignature(fresh.Issue.Issue) != issueSignature(issue) {
		return r, problem(domain.Conflict, "The issue changed after preparation. Review and confirm a new action.")
	}
	var current *domain.Transition
	for _, t := range fresh.Transitions {
		if t.ID == p.Selected.ID {
			copy := t
			current = &copy
		}
	}
	if current == nil || transitionSignature(*current) != transitionSignature(*p.Selected) {
		return r, problem(domain.Conflict, "Transition availability or field metadata changed after preparation.")
	}
	if e = domain.ValidateFields(current.Fields, p.Fields, true); e != nil {
		return r, e
	}
	r.Attempted = true
	applied, sendErr := w.Gateway.ApplyTransition(ctx, domain.TransitionRequest{Issue: issue.Ref, TransitionID: current.ID, Fields: p.Fields})
	applied.Before = &issue.Status
	r.Apply = applied
	if w.Invalidate != nil {
		w.Invalidate()
	}
	if applied.State == domain.ApplyFailed {
		if sendErr == nil {
			sendErr = problem(domain.Internal, "The transition adapter reported failure without an error.")
		}
		return r, sendErr
	}
	if applied.State != domain.ApplyAcceptedUnverified && applied.State != domain.ApplyUnknown {
		r.Apply.State = domain.ApplyUnknown
		return r, problem(domain.Uncertain, "The adapter returned an inconclusive write result; inspect Jira before retrying.")
	}
	requested := []string{}
	for id := range p.Fields {
		requested = append(requested, id)
	}
	sort.Strings(requested)
	for attempt := 0; attempt < 3 && ctx.Err() == nil; attempt++ {
		if attempt > 0 {
			if e = w.pause(ctx, time.Duration(attempt)*200*time.Millisecond); e != nil {
				break
			}
		}
		observed, e := w.Reader.GetIssue(ctx, issue.Ref, domain.DetailOptions{Fields: requested})
		if e != nil {
			continue
		}
		status := observed.Issue.Status
		r.Apply.Observed = &status
		if applied.State == domain.ApplyAcceptedUnverified && status.ID == current.To.ID && domain.FieldsMatch(p.Fields, observed.Values) {
			r.Apply.State = domain.ApplyVerified
			return r, nil
		}
		if applied.State == domain.ApplyUnknown {
			break
		}
	}
	if applied.State == domain.ApplyUnknown {
		return r, &domain.Error{Kind: domain.Uncertain, Message: "The write outcome is unknown. Observed state does not prove this request caused it; do not retry blindly.", Cause: sendErr}
	}
	r.Apply.State = domain.ApplyAcceptedUnverified
	return r, problem(domain.Uncertain, "Jira accepted the transition, but its destination and requested fields could not be verified. Do not retry blindly.")
}
func (w *Workflow) pause(ctx context.Context, d time.Duration) error {
	if w.Sleep != nil {
		return w.Sleep(ctx, d)
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
func (p Preparation) Summary() string {
	if p.Noop {
		return fmt.Sprintf("%s already satisfies the requested intent.", p.Catalog.Issue.Issue.Ref.Key)
	}
	if p.Selected == nil {
		return "Choose a transition."
	}
	return fmt.Sprintf("%s: %s -> %s (%s)", p.Catalog.Issue.Issue.Ref.Key, domain.CleanText(p.Catalog.Issue.Issue.Status.Name, false), domain.CleanText(p.Selected.To.Name, false), p.Selected.ID)
}
