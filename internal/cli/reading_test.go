package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"

	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/ports"
)

type cliReader struct {
	queries       []domain.SearchRequest
	searchErr     error
	details       int
	identityCalls int
}

func (f *cliReader) Myself(context.Context) (domain.User, error) {
	f.identityCalls++
	return domain.User{ID: "u-123"}, nil
}
func (f *cliReader) Search(_ context.Context, q domain.SearchRequest) (domain.IssuePage, error) {
	f.queries = append(f.queries, q)
	if f.searchErr != nil {
		return domain.IssuePage{}, f.searchErr
	}
	key := "APP-1"
	id := "1"
	complete := q.PageToken != ""
	if complete {
		key = "APP-2"
		id = "2"
	}
	return domain.IssuePage{Issues: []domain.Issue{{Ref: domain.IssueRef{ID: id, Key: key}, Summary: "Synthetic\x1b[31m summary\x1b[0m", Status: domain.Status{Name: "En progreso", Category: domain.CategoryInProgress}, URL: "https://example.atlassian.net/browse/" + key}}, Complete: complete, NextPageToken: "next"}, nil
}
func (f *cliReader) GetIssue(_ context.Context, key domain.IssueRef, o domain.DetailOptions) (domain.IssueDetail, error) {
	f.details++
	d := domain.IssueDetail{Issue: domain.Issue{Ref: domain.IssueRef{ID: "1", Key: key.Key}, Summary: "Synthetic detail", URL: "https://example.atlassian.net/browse/" + key.Key, Status: domain.Status{Category: domain.CategoryTodo}}, Description: []domain.Block{{Kind: "paragraph", Text: "Description"}}, Subtasks: []domain.Issue{}, Links: []domain.IssueLink{}, SubtasksComplete: true}
	if o.IncludeComments {
		d.Comments = &domain.CommentPage{}
		return d, &domain.Error{Kind: domain.Partial, Message: "Comments unavailable"}
	}
	return d, nil
}

type fakeBrowser struct {
	url string
	err error
}

func (f *fakeBrowser) Open(_ context.Context, u string) error { f.url = u; return f.err }
func readingFixture(t *testing.T) (Dependencies, *cliReader, *fakeBrowser) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config.json")
	p := config.Profile{Provider: "jira-cloud", SiteURL: "https://example.atlassian.net", AccountID: "u-123", Auth: config.Auth{Email: "user@example.com", Method: "api-token-scoped"}, CloudID: "cloud-123"}
	if e := config.Update(context.Background(), path, func(c *config.Config) error { c.Profiles["work"] = p; c.ActiveProfile = "work"; return nil }); e != nil {
		t.Fatal(e)
	}
	reader := &cliReader{}
	opener := &fakeBrowser{}
	return Dependencies{Env: func(k string) string {
		return map[string]string{"JFLOW_CONFIG": path, "JFLOW_TOKEN": "synthetic-token"}[k]
	}, Secrets: noSecrets{t}, IssueReader: func(config.Profile, ports.Secret) (ports.IssueReader, error) { return reader, nil }, Browser: opener}, reader, opener
}
func runReading(t *testing.T, deps Dependencies, args []string, expected int) (map[string]any, string) {
	t.Helper()
	var out, stderr bytes.Buffer
	code := RunWithDependencies(context.Background(), args, strings.NewReader(""), &out, &stderr, app.VersionInfo{}, deps)
	if code != expected {
		t.Fatalf("%v code=%d out=%s err=%s", args, code, &out, &stderr)
	}
	if strings.Contains(out.String()+stderr.String(), "synthetic-token") {
		t.Fatal("token leaked")
	}
	if requestedFormat(args) == "json" {
		dec := json.NewDecoder(&out)
		var data map[string]any
		if e := dec.Decode(&data); e != nil {
			t.Fatal(e)
		}
		if dec.Decode(new(any)) != io.EOF || stderr.Len() != 0 {
			t.Fatal("JSON contract broken", stderr.String())
		}
		return data, ""
	}
	return nil, out.String()
}
func TestReadCLIQueriesTablesAndDefaults(t *testing.T) {
	deps, source, _ := readingFixture(t)
	first, _ := runReading(t, deps, []string{"mine", "--limit=1", "--format=json"}, 0)
	meta := first["meta"].(map[string]any)
	if meta["complete"] != false || meta["returned"] != float64(1) {
		t.Fatal(first)
	}
	cursor := meta["next_page_token"].(string)
	runReading(t, deps, []string{"mine", "--limit=1", "--page-token", cursor, "--format=json"}, 0)
	_, table := runReading(t, deps, []string{"mine", "--all", "--format=table"}, 0)
	if !strings.Contains(table, "CLAVE") || strings.Contains(table, "\x1b") || source.details != 0 {
		t.Fatal(table)
	}
	runReading(t, deps, []string{"config", "set", "default_project", "APP", "--format=json"}, 0)
	runReading(t, deps, []string{"list", "--limit=1", "--format=json"}, 0)
	if !strings.Contains(source.queries[len(source.queries)-1].JQL, `project = "APP"`) {
		t.Fatal(source.queries)
	}
	runReading(t, deps, []string{"search", "--jql", "project = OTHER", "--limit=1", "--format=json"}, 0)
	if source.queries[len(source.queries)-1].JQL != "project = OTHER" {
		t.Fatal("default project contaminated raw JQL")
	}
	for _, args := range [][]string{{"search", "--jql=x", "--project=APP"}, {"mine", "--jql=x"}, {"mine", "--page-size=0"}, {"mine", "--all", "--limit=3"}, {"mine", "--fields=description"}, {"mine", "--sort=evil()"}, {"show", "APP-1", "--format=table"}, {"version", "--format=table"}, {"show", "APP-1", "--comments-start=1"}, {"mine", "--timeout=0s"}, {"mine", "--offline", "--refresh"}} {
		if strings.Contains(strings.Join(args, " "), "--format=") {
			runReading(t, deps, args, 2)
		} else {
			runReading(t, deps, append(args, "--format=json"), 2)
		}
	}
}
func TestLinkAndOpenAreOfflineAndBounded(t *testing.T) {
	deps, source, opener := readingFixture(t)
	env := deps.Env
	deps.Env = func(k string) string {
		if k == "JFLOW_TOKEN" {
			return ""
		}
		return env(k)
	}
	_, plain := runReading(t, deps, []string{"link", "app-123"}, 0)
	if plain != "https://example.atlassian.net/browse/APP-123\n" || source.identityCalls != 0 {
		t.Fatal(plain)
	}
	runReading(t, deps, []string{"open", "APP-123", "--format=json"}, 0)
	if opener.url != "https://example.atlassian.net/browse/APP-123" {
		t.Fatal(opener.url)
	}
	opener.err = &domain.Error{Kind: domain.Unsupported, Message: "No launcher"}
	data, _ := runReading(t, deps, []string{"open", "APP-123", "--format=json"}, 11)
	if data["data"].(map[string]any)["url"] == nil {
		t.Fatal("fallback URL missing")
	}
	runReading(t, deps, []string{"link", "../bad", "--format=json"}, 2)
}
func TestPartialJSONKeepsDataAndWriteFailuresFail(t *testing.T) {
	deps, source, _ := readingFixture(t)
	data, _ := runReading(t, deps, []string{"show", "APP-1", "--comments", "--format=json"}, 10)
	if data["ok"] != false || data["data"].(map[string]any)["issue"] == nil {
		t.Fatal(data)
	}
	source.searchErr = &domain.Error{Kind: domain.Forbidden, Message: "Forbidden"}
	runReading(t, deps, []string{"mine", "--format=json"}, 4)
	for _, cmd := range []string{"link", "show"} {
		code := RunWithDependencies(context.Background(), []string{cmd, "APP-1", "--format=json"}, strings.NewReader(""), brokenWriter{}, io.Discard, app.VersionInfo{}, deps)
		if code != 1 {
			t.Fatal(fmt.Sprint(cmd, code))
		}
	}
}
