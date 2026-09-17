package cli

import (
	"context"
	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/ports"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMetricsCLIContractsAndPartialSummary(t *testing.T) {
	deps, source, _ := readingFixture(t)
	data, _ := runReading(t, deps, []string{"progress", "APP-1", "--format=json"}, 0)
	d := data["data"].(map[string]any)
	if d["subtasks"].(map[string]any)["percent"] != nil || d["time"].(map[string]any)["spent_seconds"] != nil || d["state_since"] != nil {
		t.Fatal(data)
	}
	_, plain := runReading(t, deps, []string{"progress", "APP-1"}, 0)
	if !strings.Contains(plain, "No subtasks") || !strings.Contains(plain, "unknown") {
		t.Fatal(plain)
	}
	data, _ = runReading(t, deps, []string{"summary", "--include-done", "--format=json"}, 0)
	d = data["data"].(map[string]any)
	if d["processed"] != float64(2) || d["completed_percent"] != float64(0) || data["meta"].(map[string]any)["complete"] != true {
		t.Fatal(data)
	}
	if strings.Contains(source.queries[len(source.queries)-1].JQL, "!= Done") {
		t.Fatal("excluded completed population")
	}
	data, _ = runReading(t, deps, []string{"summary", "--format=json"}, 0)
	if data["data"].(map[string]any)["completed_percent"] != nil {
		t.Fatal("percentage over pending issues")
	}
	data, _ = runReading(t, deps, []string{"summary", "--project=APP", "--max-results=1", "--format=json"}, 10)
	if data["data"].(map[string]any)["processed"] != float64(1) || data["data"].(map[string]any)["completed_percent"] != nil || data["ok"] != false {
		t.Fatal(data)
	}
	if data["meta"].(map[string]any)["next_page_token"] != nil {
		t.Fatal("summary exposed an unusable continuation", data)
	}
	data, _ = runReading(t, deps, []string{"summary", "--jql=project=APP", "--format=json"}, 0)
	if data["data"].(map[string]any)["completed_percent"] != nil {
		t.Fatal("inferred JQL denominator")
	}
	for _, args := range [][]string{{"summary", "--jql=x", "--project=APP"}, {"summary", "--page-size=0"}, {"summary", "--max-results=0"}, {"progress", "APP-1", "--timezone=bad-zone"}, {"progress", "APP-1", "--history-limit=0"}, {"progress", "bad-key"}, {"config", "set", "cache.persist", "yes"}} {
		runReading(t, deps, append(args, "--format=json"), 2)
	}
}
func TestPersistentCLIExactQueryOfflineAndPrivateDetails(t *testing.T) {
	deps, source, _ := readingFixture(t)
	dir := t.TempDir()
	env := deps.Env
	deps.Env = func(k string) string {
		if k == "JFLOW_CACHE" {
			return dir
		}
		return env(k)
	}
	runReading(t, deps, []string{"cache", "status", "--format=json"}, 0)
	if _, err := os.Stat(filepath.Join(dir, "v1")); !os.IsNotExist(err) {
		t.Fatal("status created cache", err)
	}
	runReading(t, deps, []string{"config", "set", "cache.persist", "true", "--format=json"}, 0)
	runReading(t, deps, []string{"summary", "--include-done", "--format=json"}, 0)
	runReading(t, deps, []string{"show", "APP-1", "--format=json"}, 0)
	onlineCalls := source.identityCalls
	deps.IssueReader = func(config.Profile, ports.Secret) (ports.IssueReader, error) {
		t.Fatal("offline constructed network source")
		return nil, nil
	}
	data, _ := runReading(t, deps, []string{"summary", "--include-done", "--offline", "--format=json"}, 0)
	if data["meta"].(map[string]any)["source"] != "disk" || source.identityCalls != onlineCalls {
		t.Fatal(data)
	}
	data, _ = runReading(t, deps, []string{"show", "APP-1", "--offline", "--format=json"}, 0)
	d := data["data"].(map[string]any)
	if len(d["description"].([]any)) != 0 || data["meta"].(map[string]any)["complete"] != false || len(data["warnings"].([]any)) == 0 {
		t.Fatal(data)
	}
	files, err := os.ReadDir(filepath.Join(dir, "v1"))
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(dir, "v1", f.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), `"Text":"Description"`) || strings.Contains(string(b), "synthetic-token") {
			t.Fatal("persisted sensitive content")
		}
	}
	runReading(t, deps, []string{"summary", "--project=OTHER", "--offline", "--format=json"}, 5)
	runReading(t, deps, []string{"cache", "clear", "--format=json"}, 0)
	runReading(t, deps, []string{"summary", "--include-done", "--offline", "--format=json"}, 5)
}
func TestPersistentOnlineAuthFailureAndCredentialGeneration(t *testing.T) {
	deps, _, _ := readingFixture(t)
	env := deps.Env
	dir := t.TempDir()
	deps.Env = func(k string) string {
		if k == "JFLOW_CACHE" {
			return dir
		}
		return env(k)
	}
	runReading(t, deps, []string{"config", "set", "cache.persist", "true", "--format=json"}, 0)
	runReading(t, deps, []string{"mine", "--format=json"}, 0)
	deps.IssueReader = func(config.Profile, ports.Secret) (ports.IssueReader, error) { return &deniedReader{}, nil }
	runReading(t, deps, []string{"mine", "--format=json"}, 3)
	runReading(t, deps, []string{"mine", "--offline", "--format=json"}, 5)
	// Generation changes make even an identical credential's old data unreachable.
	deps, _, _ = readingFixture(t)
	env2 := deps.Env
	deps.Env = func(k string) string {
		if k == "JFLOW_CACHE" {
			return dir
		}
		return env2(k)
	}
	runReading(t, deps, []string{"config", "set", "cache.persist", "true", "--format=json"}, 0)
	runReading(t, deps, []string{"mine", "--format=json"}, 0)
	if err := config.Update(context.Background(), env2("JFLOW_CONFIG"), func(c *config.Config) error {
		p := c.Profiles["work"]
		p.CacheGeneration++
		c.Profiles["work"] = p
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	runReading(t, deps, []string{"mine", "--offline", "--format=json"}, 5)
}

type deniedReader struct{ cliReader }

func (*deniedReader) Myself(context.Context) (domain.User, error) {
	return domain.User{}, &domain.Error{Kind: domain.Authentication, Message: "Expired credential"}
}

func TestWorkflowInvalidatesPersistedReadsEvenWhenUncertain(t *testing.T) {
	for _, uncertain := range []bool{false, true} {
		deps, w := workflowCLI(t)
		env := deps.Env
		dir := t.TempDir()
		deps.Env = func(k string) string {
			if k == "JFLOW_CACHE" {
				return dir
			}
			return env(k)
		}
		runReading(t, deps, []string{"config", "set", "cache.persist", "true", "--format=json"}, 0)
		runReading(t, deps, []string{"mine", "--format=json"}, 0)
		runReading(t, deps, []string{"start", "APP-1", "--dry-run", "--format=json"}, 0)
		runReading(t, deps, []string{"mine", "--offline", "--format=json"}, 0)
		code := 0
		if uncertain {
			w.writeState = domain.ApplyUnknown
			code = 9
		}
		runReading(t, deps, []string{"start", "APP-1", "--yes", "--no-record", "--format=json"}, code)
		if w.writes != 1 {
			t.Fatal(w.writes)
		}
		runReading(t, deps, []string{"mine", "--offline", "--format=json"}, 5)
	}
}
