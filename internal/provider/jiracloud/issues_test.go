package jiracloud

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/output"
	"jira-flow.local/jflow/internal/ports"
)

func fixtureBytes(t *testing.T, name string) []byte {
	t.Helper()
	b, e := os.ReadFile("../../../testdata/jiracloud/" + name)
	if e != nil {
		t.Fatal(e)
	}
	return b
}
func TestSearchEnhancedEndpointAndFields(t *testing.T) {
	for _, method := range []string{"api-token-unscoped", "api-token-scoped"} {
		t.Run(method, func(t *testing.T) {
			calls := 0
			client := setup(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				want := "/rest/api/3/search/jql"
				if method == "api-token-scoped" {
					want = "/ex/jira/cloud-123" + want
				}
				if r.Method != "POST" || r.URL.Path != want {
					t.Error(r.Method, r.URL.Path)
				}
				var body struct {
					JQL    string   `json:"jql"`
					Token  string   `json:"nextPageToken"`
					Size   int      `json:"maxResults"`
					Fields []string `json:"fields"`
				}
				if json.NewDecoder(r.Body).Decode(&body) != nil || body.JQL != "project = APP" || body.Size != 2 || body.Token != "cursor" {
					t.Errorf("body %+v", body)
				}
				if strings.Contains(strings.Join(body.Fields, ","), "description") {
					t.Error("eager details")
				}
				if r.Header.Get("Content-Type") != "application/json" {
					t.Error("content type")
				}
				w.Write(fixtureBytes(t, "search-page-1.json"))
			})
			s := &Session{client, profile(method), ports.NewSecret("synthetic-token")}
			page, e := s.Search(context.Background(), domain.SearchRequest{JQL: "project = APP", PageSize: 2, PageToken: "cursor"})
			if e != nil || page.Complete || page.NextPageToken != "synthetic-cursor-2" || len(page.Issues) != 1 || calls != 1 {
				t.Fatal(page, e, calls)
			}
			i := page.Issues[0]
			if i.Status.Category != domain.CategoryInProgress || i.Assignee == nil || i.UpdatedAt.IsZero() || !strings.Contains(i.URL, "example.atlassian.net/browse/APP-123") {
				t.Fatal(i)
			}
		})
	}
}
func TestShowFieldsADFAndLazySections(t *testing.T) {
	calls := 0
	client := setup(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/rest/api/3/issue/APP-123" {
			t.Fatal("unexpected section request", r.URL.Path)
		}
		fields := r.URL.Query().Get("fields")
		if !strings.Contains(fields, "description") || strings.Contains(fields, "comment") {
			t.Error(fields)
		}
		w.Write(fixtureBytes(t, "issue.json"))
	})
	s := &Session{client, profile("api-token-unscoped"), ports.NewSecret("token")}
	d, e := s.GetIssue(context.Background(), domain.IssueRef{Key: "app-123"}, domain.DetailOptions{IncludeDescription: true, IncludeSubtasks: true, IncludeLinks: true})
	if e != nil || calls != 1 || !d.SubtasksComplete || d.Comments != nil || d.History != nil || !strings.Contains(output.BlocksText(d.Description), "Synthetic content for tests") {
		t.Fatal(d, e, calls)
	}
	if d.Issue.DueDate == nil || d.Issue.Resolution != nil {
		t.Fatal(d.Issue)
	}
}
func TestSectionPaginationPartialAndDeduplication(t *testing.T) {
	for _, all := range []bool{false, true} {
		t.Run(fmt.Sprint(all), func(t *testing.T) {
			calls := []string{}
			client := setup(t, func(w http.ResponseWriter, r *http.Request) {
				calls = append(calls, r.URL.Path+"?"+r.URL.RawQuery)
				switch {
				case strings.HasSuffix(r.URL.Path, "APP-123"):
					w.Write(fixtureBytes(t, "issue.json"))
				case strings.HasSuffix(r.URL.Path, "comment"):
					if r.URL.Query().Get("startAt") == "0" {
						fmt.Fprint(w, `{"startAt":0,"total":3,"comments":[{"id":"c1","body":{"type":"doc","content":[{"type":"paragraph","content":[{"type":"text","text":"first"}]}]}},{"id":"c2","body":"second"}]}`)
					} else {
						fmt.Fprint(w, `{"startAt":2,"total":3,"comments":[{"id":"c3","body":"third"}]}`)
					}
				case strings.HasSuffix(r.URL.Path, "changelog"):
					fmt.Fprint(w, `{"startAt":0,"total":1,"isLast":true,"values":[{"id":"h1","items":[{"field":"status","fromString":"Todo","toString":"Done"}]}]}`)
				}
			})
			s := &Session{client, profile("api-token-unscoped"), ports.NewSecret("token")}
			limit := 2
			if all {
				limit = 5
			}
			d, e := s.GetIssue(context.Background(), domain.IssueRef{Key: "APP-123"}, domain.DetailOptions{IncludeComments: true, IncludeHistory: true, All: all, PageSize: 2, SectionLimit: limit})
			if e != nil || d.Comments == nil || d.History == nil || !d.History.Complete {
				t.Fatal(d, e)
			}
			if all {
				if len(d.Comments.Items) != 3 || !d.Comments.Complete || len(calls) != 4 {
					t.Fatal(d, calls)
				}
			} else {
				if len(d.Comments.Items) != 2 || d.Comments.Complete || *d.Comments.NextStart != 2 || len(calls) != 3 {
					t.Fatal(d, calls)
				}
			}
		})
	}
}
func TestSectionFailureRetainsIssue(t *testing.T) {
	client := setup(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "comment") {
			w.WriteHeader(403)
			return
		}
		w.Write(fixtureBytes(t, "issue.json"))
	})
	s := &Session{client, profile("api-token-unscoped"), ports.NewSecret("token")}
	d, e := s.GetIssue(context.Background(), domain.IssueRef{Key: "APP-123"}, domain.DetailOptions{IncludeComments: true})
	if e == nil || d.Issue.Ref.Key != "APP-123" || d.Comments.Complete {
		t.Fatal(d, e)
	}
}
func TestReadRetriesRetryAfterAndRedaction(t *testing.T) {
	calls := 0
	waits := []time.Duration{}
	client := setup(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls < 3 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(429)
			fmt.Fprint(w, "synthetic-secret")
			return
		}
		fmt.Fprint(w, `{"issues":[],"isLast":true}`)
	})
	client.Sleep = func(ctx context.Context, d time.Duration) error { waits = append(waits, d); return nil }
	s := &Session{client, profile("api-token-unscoped"), ports.NewSecret("synthetic-secret")}
	_, e := s.Search(context.Background(), domain.SearchRequest{JQL: "x", PageSize: 1})
	if e != nil || calls != 3 || len(waits) != 2 || waits[0] != 0 {
		t.Fatal(calls, waits, e)
	}
	client = setup(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3600")
		w.WriteHeader(429)
	})
	s.Client = client
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if _, e = s.Search(ctx, domain.SearchRequest{JQL: "x", PageSize: 1}); e == nil || !strings.Contains(e.Error(), "wait") {
		t.Fatal(e)
	}
}
func TestReadRejectsBadPagesAndRedirects(t *testing.T) {
	for _, body := range []string{`{}`, `{"issues":[],"isLast":false}`, `{"issues":[{"id":"1","key":"../bad"}],"isLast":true}`, `<html>login</html>`} {
		client := setup(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) })
		s := &Session{client, profile("api-token-unscoped"), ports.NewSecret("token")}
		if _, e := s.Search(context.Background(), domain.SearchRequest{JQL: "x", PageSize: 1}); e == nil {
			t.Fatal("accepted", body)
		}
	}
	calls := 0
	client := setup(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Location", "https://evil.example")
		w.WriteHeader(302)
	})
	s := &Session{client, profile("api-token-unscoped"), ports.NewSecret("token")}
	if _, e := s.Search(context.Background(), domain.SearchRequest{JQL: "x", PageSize: 1}); e == nil || calls != 1 {
		t.Fatal(e, calls)
	}
}
func TestADFUnknownNodesLinksAndControls(t *testing.T) {
	raw := json.RawMessage(`{"type":"doc","content":[{"type":"bulletList","content":[{"type":"listItem","content":[{"type":"paragraph","content":[{"type":"text","text":"\u001b[31mhello\u001b[0m","marks":[{"type":"link","attrs":{"href":"https://example.com"}}]}]}]}]},{"type":"future","content":[{"type":"text","text":"descendant"}]},{"type":"media"},{"type":"inlineCard","attrs":{"url":"javascript:alert(1)"}}]}`)
	blocks, _ := normalizeADF(raw, func(s string) string { return domain.CleanText(s, true) })
	text := output.BlocksText(blocks)
	if !strings.Contains(text, "hello (https://example.com)") || !strings.Contains(text, "descendant") || !strings.Contains(text, "unsupported") || strings.ContainsAny(text, "\x1b") || strings.Contains(text, "javascript") {
		t.Fatal(text)
	}
}

func TestSectionRepeatedOffsetsAndProtector(t *testing.T) {
	client := setup(t, func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "comment") {
			fmt.Fprint(w, `{"startAt":0,"total":10,"comments":[{"id":"c1","body":"hello"}]}`)
			return
		}
		w.Write(fixtureBytes(t, "issue.json"))
	})
	s := &Session{client, profile("api-token-unscoped"), ports.NewSecret("token")}
	for _, limit := range []int{1, 5} {
		d, e := s.GetIssue(context.Background(), domain.IssueRef{Key: "APP-123"}, domain.DetailOptions{IncludeComments: true, All: true, SectionLimit: limit, PageSize: 1})
		if domain.ExitCode(e) != 10 || d.Comments.Complete || len(d.Comments.Items) != 1 {
			t.Fatal(d, e)
		}
	}
}

func TestReadStatusMappingLimitsAndCancellation(t *testing.T) {
	for _, tc := range []struct {
		status int
		code   int
		calls  int
	}{{400, 2, 1}, {401, 3, 1}, {403, 4, 1}, {404, 5, 1}, {409, 7, 1}, {500, 8, 1}, {502, 8, 3}, {503, 8, 3}, {504, 8, 3}} {
		t.Run(fmt.Sprint(tc.status), func(t *testing.T) {
			calls := 0
			client := setup(t, func(w http.ResponseWriter, r *http.Request) {
				calls++
				w.WriteHeader(tc.status)
				fmt.Fprint(w, "synthetic-secret "+r.Header.Get("Authorization"))
			})
			client.Sleep = func(context.Context, time.Duration) error { return nil }
			s := &Session{client, profile("api-token-unscoped"), ports.NewSecret("synthetic-secret")}
			_, e := s.Search(context.Background(), domain.SearchRequest{JQL: "x", PageSize: 1})
			if domain.ExitCode(e) != tc.code || calls != tc.calls || strings.Contains(e.Error(), "synthetic-secret") {
				t.Fatal(e, calls)
			}
		})
	}
	client := setup(t, func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, strings.Repeat("x", 4*1024*1024+1)) })
	s := &Session{client, profile("api-token-unscoped"), ports.NewSecret("token")}
	if _, e := s.Search(context.Background(), domain.SearchRequest{JQL: "x", PageSize: 1}); e == nil {
		t.Fatal("oversized response accepted")
	}
	client = setup(t, func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })
	s.Client = client
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, e := s.Search(ctx, domain.SearchRequest{JQL: "x", PageSize: 1}); domain.ExitCode(e) != 130 {
		t.Fatal(e)
	}
}
