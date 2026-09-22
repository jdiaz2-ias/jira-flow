package tui

import (
	"context"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/domain"
)

type journeySource struct {
	state                                 domain.Status
	fields                                map[string]any
	writes                                int
	ambiguous, required, changed, unknown bool
}

func (f *journeySource) Myself(context.Context) (domain.User, error) {
	return domain.User{ID: "user"}, nil
}
func (f *journeySource) Search(context.Context, domain.SearchRequest) (domain.IssuePage, error) {
	d, _ := f.GetIssue(context.Background(), domain.IssueRef{}, domain.DetailOptions{})
	return domain.IssuePage{Issues: []domain.Issue{d.Issue}, Complete: true}, nil
}
func (f *journeySource) GetIssue(context.Context, domain.IssueRef, domain.DetailOptions) (domain.IssueDetail, error) {
	return domain.IssueDetail{Issue: domain.Issue{Ref: domain.IssueRef{ID: "1", Key: "APP-1"}, Status: f.state, Summary: "Synthetic", ProjectID: "p", IssueTypeID: "t"}, Values: f.fields}, nil
}
func (f *journeySource) ListTransitions(context.Context, domain.IssueRef) ([]domain.Transition, error) {
	ts := []domain.Transition{{ID: "start", Name: "Start", To: domain.Status{ID: "active", Name: "Active", Category: domain.CategoryInProgress}}, {ID: "done", Name: "Done", To: domain.Status{ID: "done", Name: "Done", Category: domain.CategoryDone}}}
	if f.required {
		ts[1].Fields = []domain.FieldSpec{{ID: "resolution", Type: "resolution", Name: "Resolution", Required: true, AllowedValues: []domain.NamedID{{ID: "fixed", Name: "Fixed"}}}}
	}
	if f.ambiguous {
		ts = append(ts, domain.Transition{ID: "cancel", Name: "Cancel", To: domain.Status{ID: "cancelled", Category: domain.CategoryDone}})
	}
	return ts, nil
}
func (f *journeySource) ApplyTransition(_ context.Context, r domain.TransitionRequest) (domain.ApplyResult, error) {
	f.writes++
	f.fields = r.Fields
	ts, _ := f.ListTransitions(context.Background(), r.Issue)
	for _, t := range ts {
		if t.ID == r.TransitionID {
			f.state = t.To
		}
	}
	state := domain.ApplyAcceptedUnverified
	if f.unknown {
		state = domain.ApplyUnknown
	}
	return domain.ApplyResult{State: state, Issue: r.Issue, TransitionID: r.TransitionID}, nil
}
func workflowModel(t *testing.T) (*Model, *journeySource) {
	t.Helper()
	f := &journeySource{state: domain.Status{ID: "todo", Name: "Todo", Category: domain.CategoryTodo}}
	w := &app.Workflow{Reader: f, Gateway: f, Profile: "fixture", Site: "https://example.atlassian.net"}
	m := New(context.Background(), &fakeReader{}, Options{Workflow: w})
	m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
	m.Update(m.Init()())
	return m, f
}
func press(m *Model, s string) tea.Cmd { _, cmd := m.Update(key(s)); return cmd }
func execute(m *Model, cmd tea.Cmd) {
	if cmd != nil {
		m.Update(cmd())
	}
}
func TestWorkflowReviewSingleWriteAndRefresh(t *testing.T) {
	m, f := workflowModel(t)
	execute(m, press(m, "s"))
	if m.dialog.stage != "review" || f.writes != 0 {
		t.Fatal("missing read-only preview")
	}
	execute(m, press(m, "enter"))
	if f.writes != 0 {
		t.Fatal("enter confirmed")
	}
	apply := press(m, "y")
	if apply == nil || !m.writing {
		t.Fatal("missing confirmation")
	}
	for _, k := range []string{"y", "d", "r", "q", "esc"} {
		if press(m, k) != nil {
			t.Fatal("action enabled during write", k)
		}
	}
	execute(m, apply)
	if f.writes != 1 || m.dialog.result.Apply.State != domain.ApplyVerified {
		t.Fatal(f.writes, m.dialog)
	}
	refresh := press(m, "enter")
	if refresh == nil {
		t.Fatal("no refresh")
	}
	execute(m, refresh)
	if m.dialog != nil || !m.detailMode || !strings.Contains(m.notice, "verified") {
		t.Fatal("outcome lost")
	}
}
func TestWorkflowAmbiguityFieldsAndDraftProtection(t *testing.T) {
	m, f := workflowModel(t)
	f.ambiguous = true
	f.required = true
	execute(m, press(m, "d"))
	if m.dialog.stage != "choose" {
		t.Fatal("ambiguous transition not offered")
	}
	execute(m, press(m, "enter"))
	if m.dialog.stage != "field" {
		t.Fatal("required field not prompted")
	}
	execute(m, press(m, "bad"))
	execute(m, press(m, "enter"))
	if m.err == nil || f.writes != 0 {
		t.Fatal("invalid option accepted")
	}
	m.dialog.input = "fixed"
	press(m, "esc")
	if !m.discard {
		t.Fatal("draft silently discarded")
	}
	press(m, "esc")
	execute(m, press(m, "enter"))
	if m.dialog.stage != "review" {
		t.Fatal(m.err, m.dialog.stage)
	}
	execute(m, press(m, "y"))
	if f.writes != 1 || f.fields["resolution"].(map[string]any)["id"] != "fixed" {
		t.Fatal("field missing")
	}
}
func TestStalePlanAndUncertainOutcome(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		m, f := workflowModel(t)
		f.unknown = unknown
		execute(m, press(m, "s"))
		if !unknown {
			f.state.ID = "changed"
		}
		execute(m, press(m, "y"))
		if unknown {
			if f.writes != 1 || m.dialog.result.Apply.State != domain.ApplyUnknown || m.err == nil {
				t.Fatal("uncertainty lost")
			}
			if press(m, "y") != nil || f.writes != 1 {
				t.Fatal("uncertain write retried")
			}
		} else if f.writes != 0 || m.err == nil {
			t.Fatal("stale plan applied")
		}
	}
}
func TestOfflineSmallTerminalAndLongReviewNeverWrite(t *testing.T) {
	m, f := workflowModel(t)
	m.options.Search.Offline = true
	if press(m, "s") != nil || m.dialog != nil {
		t.Fatal("offline action allowed")
	}
	m.options.Search.Offline = false
	execute(m, press(m, "s"))
	m.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	if press(m, "y") != nil {
		t.Fatal("hidden review applied")
	}
	m.Update(tea.WindowSizeMsg{Width: 60, Height: 15})
	m.dialog.plan.Catalog.Issue.Issue.Summary = strings.Repeat("long ", 100)
	if press(m, "y") != nil || f.writes != 0 {
		t.Fatal("unreviewed fields applied")
	}
}
func TestRemoteSearchDoesNotRunShortcutsAndResetsCursor(t *testing.T) {
	m, _ := workflowModel(t)
	m.Update(tea.KeyPressMsg{Code: 'f', Mod: tea.ModCtrl})
	if !m.searching {
		t.Fatal("search not focused")
	}
	press(m, "project = APP")
	if m.dialog != nil {
		t.Fatal("shortcut ran in search")
	}
	execute(m, press(m, "enter"))
	if m.searching || m.options.Search.Query.JQL != "project = APP" || m.scopeLabel != "Busqueda JQL" {
		t.Fatal("remote query not set")
	}
}
func TestThemesLayoutsAndResizePreserveDraft(t *testing.T) {
	m, _ := workflowModel(t)
	for _, theme := range []string{"light", "dark", "auto", "mono"} {
		m.options.Theme = theme
		for _, width := range []int{60, 80, 110, 160} {
			m.Update(tea.WindowSizeMsg{Width: width, Height: 24})
			content := m.View().Content
			if width >= 110 && !strings.Contains(content, "│") {
				t.Fatal("missing panels")
			}
			if theme == "mono" && strings.Contains(content, "\x1b") {
				t.Fatal("mono has color")
			}
		}
	}
	m.options.ASCII = true
	m.options.NoColor = true
	if strings.Contains(m.View().Content, "│") || strings.Contains(m.View().Content, "\x1b") {
		t.Fatal("ASCII/no-color failed")
	}
	execute(m, press(m, "s"))
	m.dialog.options.Fields["summary"] = "draft"
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 20})
	if m.dialog.options.Fields["summary"] != "draft" {
		t.Fatal("resize lost draft")
	}
}

func TestCloseRequiresSelectionAndNoopDoesNotWrite(t *testing.T) {
	m, f := workflowModel(t)
	execute(m, press(m, "x"))
	if m.dialog.stage != "choose" || f.writes != 0 {
		t.Fatal("close chose a transition implicitly")
	}
	press(m, "esc")
	f.state = domain.Status{ID: "active", Category: domain.CategoryInProgress}
	execute(m, press(m, "s"))
	if !m.dialog.plan.Noop {
		t.Fatal("already active did not prepare noop")
	}
	execute(m, press(m, "y"))
	if f.writes != 0 || m.dialog.result.Apply.State != domain.ApplyNoop {
		t.Fatal("noop wrote to Jira")
	}
}
func TestPasteOwnsFocusAndPendingWriteCancellationKeepsOutcome(t *testing.T) {
	m, f := workflowModel(t)
	f.required = true
	execute(m, press(m, "d"))
	m.Update(tea.PasteMsg{Content: "fixed"})
	if m.dialog.input != "fixed" || f.writes != 0 {
		t.Fatal("paste triggered action")
	}
	execute(m, press(m, "enter"))
	apply := press(m, "y")
	m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !m.PendingWrite() || m.dialog == nil {
		t.Fatal("cancellation discarded pending write")
	}
	execute(m, apply)
	if m.PendingWrite() || m.dialog.stage != "result" {
		t.Fatal("outcome lost")
	}
}
