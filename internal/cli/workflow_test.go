package cli

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/ports"
)

type commandWorkflow struct {
	state              domain.Status
	fields             map[string]any
	writes, reads      int
	ambiguous          bool
	required           bool
	changeAfterPrepare bool
	writeState         domain.ApplyState
}

func (f *commandWorkflow) Myself(context.Context) (domain.User, error) {
	return domain.User{ID: "u-123"}, nil
}
func (f *commandWorkflow) Search(context.Context, domain.SearchRequest) (domain.IssuePage, error) {
	return domain.IssuePage{}, errors.New("unused")
}
func (f *commandWorkflow) GetIssue(context.Context, domain.IssueRef, domain.DetailOptions) (domain.IssueDetail, error) {
	f.reads++
	state := f.state
	if f.changeAfterPrepare && f.reads > 1 && f.writes == 0 {
		state.ID = "other"
	}
	return domain.IssueDetail{Issue: domain.Issue{Ref: domain.IssueRef{ID: "i-1", Key: "APP-1"}, ProjectID: "p-1", IssueTypeID: "type-1", Status: state, URL: "https://example.atlassian.net/browse/APP-1"}, Values: f.fields}, nil
}
func (f *commandWorkflow) ListTransitions(context.Context, domain.IssueRef) ([]domain.Transition, error) {
	ts := []domain.Transition{{ID: "begin", Name: "Investigate", To: domain.Status{ID: "active", Name: "Investigating", Category: domain.CategoryInProgress}, Fields: []domain.FieldSpec{}}, {ID: "resolve", Name: "Resolve", To: domain.Status{ID: "resolved", Name: "Resolved", Category: domain.CategoryDone}, Fields: []domain.FieldSpec{}}}
	if f.required {
		ts[1].Fields = []domain.FieldSpec{{ID: "resolution", Name: "Resolution", Type: "resolution", Required: true, AllowedValues: []domain.NamedID{{ID: "fixed", Name: "Fixed"}}}}
	}
	if f.ambiguous {
		ts = append(ts, domain.Transition{ID: "cancel", Name: "Cancel", To: domain.Status{ID: "cancelled", Category: domain.CategoryDone}})
	}
	return ts, nil
}
func (f *commandWorkflow) ApplyTransition(_ context.Context, q domain.TransitionRequest) (domain.ApplyResult, error) {
	f.writes++
	ts, _ := f.ListTransitions(context.Background(), q.Issue)
	for _, t := range ts {
		if t.ID == q.TransitionID {
			f.state = t.To
		}
	}
	f.fields = q.Fields
	state := f.writeState
	if state == "" {
		state = domain.ApplyAcceptedUnverified
	}
	result := domain.ApplyResult{State: state, Issue: q.Issue, TransitionID: q.TransitionID}
	if state == domain.ApplyUnknown {
		return result, &domain.Error{Kind: domain.Uncertain, Message: "Lost response"}
	}
	return result, nil
}
func workflowCLI(t *testing.T) (Dependencies, *commandWorkflow) {
	t.Helper()
	deps, _, _ := readingFixture(t)
	f := &commandWorkflow{state: domain.Status{ID: "todo", Name: "To Do", Category: domain.CategoryTodo}}
	deps.Workflow = func(config.Profile, ports.Secret) (workflowSession, error) { return f, nil }
	return deps, f
}
func TestWorkflowCommandsNeverWriteWithoutResolvedApproval(t *testing.T) {
	for _, tc := range []struct {
		args      []string
		code      int
		ambiguous bool
	}{{[]string{"start", "APP-1"}, 2, false}, {[]string{"start", "APP-1", "--dry-run"}, 0, false}, {[]string{"done", "APP-1", "--yes"}, 6, true}, {[]string{"close", "APP-1", "--yes"}, 6, false}, {[]string{"transition", "APP-1", "--yes"}, 6, false}, {[]string{"transition", "APP-1", "--id=missing", "--yes"}, 7, false}, {[]string{"start", "APP-1", "--transition-id=resolve", "--yes"}, 6, false}} {
		deps, f := workflowCLI(t)
		f.ambiguous = tc.ambiguous
		runReading(t, deps, append(tc.args, "--format=json"), tc.code)
		if f.writes != 0 {
			t.Fatal("unexpected Jira mutation", tc.args)
		}
	}
}
func TestWorkflowJSONSuccessFieldsAndUnknownOutcome(t *testing.T) {
	deps, f := workflowCLI(t)
	f.required = true
	path := filepath.Join(t.TempDir(), "fields.json")
	os.WriteFile(path, []byte(`{"resolution":{"id":"fixed"}}`), 0600)
	data, _ := runReading(t, deps, []string{"done", "APP-1", "--fields-file", path, "--yes", "--no-record", "--format=json"}, 0)
	result := data["data"].(map[string]any)
	if result["result"] != "verified" || f.writes != 1 || len(f.fields) != 1 {
		t.Fatal(data, f.writes)
	}
	deps, f = workflowCLI(t)
	f.writeState = domain.ApplyUnknown
	data, _ = runReading(t, deps, []string{"start", "APP-1", "--yes", "--no-record", "--format=json"}, 9)
	if data["data"].(map[string]any)["result"] != "unknown" || f.writes != 1 {
		t.Fatal(data, f.writes)
	}
}
func TestInteractiveSelectionRequiredFieldsAndConfirmation(t *testing.T) {
	deps, f := workflowCLI(t)
	f.ambiguous = true
	f.required = true
	deps.Interactive = func() bool { return true }
	answers := []string{"resolve", "fixed", "n", "yes"}
	deps.Prompt = func(_ context.Context, label string) (string, error) {
		if len(answers) == 0 {
			t.Fatal("unexpected prompt", label)
		}
		answer := answers[0]
		answers = answers[1:]
		return answer, nil
	}
	_, plain := runReading(t, deps, []string{"done", "APP-1", "--no-record"}, 0)
	if f.writes != 1 || len(answers) != 0 || !strings.Contains(plain, "verified") {
		t.Fatal(plain, answers, f.writes)
	}
	deps, f = workflowCLI(t)
	deps.Interactive = func() bool { return true }
	deps.Prompt = func(context.Context, string) (string, error) { return "n", nil }
	runReading(t, deps, []string{"start", "APP-1", "--no-record"}, 130)
	if f.writes != 0 {
		t.Fatal("decline sent a write")
	}
}
func TestWorkflowMapIsLocalAndScoped(t *testing.T) {
	deps, f := workflowCLI(t)
	runReading(t, deps, []string{"workflow", "map", "APP-1", "--intent=close", "--transition-id=resolve", "--format=json"}, 0)
	c, e := config.Load(deps.Env("JFLOW_CONFIG"))
	if e != nil || len(c.Profiles["work"].WorkflowRules) != 1 || f.writes != 0 {
		t.Fatal(c, e)
	}
	rule := c.Profiles["work"].WorkflowRules[0]
	if rule.ProjectID != "p-1" || rule.IssueTypeID != "type-1" || rule.FromStatusID != "todo" || rule.ExpectedToStatusID != "resolved" {
		t.Fatal(rule)
	}
	runReading(t, deps, []string{"close", "APP-1", "--yes", "--no-record", "--format=json"}, 0)
	if f.writes != 1 {
		t.Fatal(f.writes)
	}
}
func TestConflictsStopBeforeSendAndFieldsFileRejectsBodies(t *testing.T) {
	deps, f := workflowCLI(t)
	f.changeAfterPrepare = true
	runReading(t, deps, []string{"start", "APP-1", "--yes", "--no-record", "--format=json"}, 7)
	if f.writes != 0 {
		t.Fatal("stale preparation applied")
	}
	for _, body := range []string{`null`, `[]`, `{"fields":{"resolution":{"id":"1"}}}`, `{} {}`, strings.Repeat("a", 1024*1024+1)} {
		path := filepath.Join(t.TempDir(), "fields.json")
		os.WriteFile(path, []byte(body), 0600)
		if _, err := loadFields(path); err == nil {
			t.Fatal("accepted invalid fields file")
		}
	}
}
func TestActionRecordsContainOnlyMetadata(t *testing.T) {
	deps, f := workflowCLI(t)
	data, _ := runReading(t, deps, []string{"start", "APP-1", "--yes", "--format=json"}, 0)
	id := data["data"].(map[string]any)["action_id"].(string)
	if id == "" || f.writes != 1 {
		t.Fatal(data)
	}
	matches, _ := filepath.Glob(filepath.Join(filepath.Dir(deps.Env("JFLOW_CONFIG")), "state", "actions", "*", id+".json"))
	if len(matches) != 1 {
		t.Fatal(matches)
	}
	b, _ := os.ReadFile(matches[0])
	var record map[string]any
	json.Unmarshal(b, &record)
	if len(record) != 6 || strings.Contains(string(b), "synthetic-token") {
		t.Fatal(string(b))
	}
}
