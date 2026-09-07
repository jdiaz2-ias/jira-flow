package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"jira-flow.local/jflow/internal/cache"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/ports"
)

type fakeReader struct {
	queries                    []domain.SearchRequest
	identityCalls, detailCalls int
	identityErr                error
	search                     func(domain.SearchRequest) (domain.IssuePage, error)
	detail                     func(domain.DetailOptions) (domain.IssueDetail, error)
}

func (f *fakeReader) Myself(context.Context) (domain.User, error) {
	f.identityCalls++
	return domain.User{ID: "user"}, f.identityErr
}
func (f *fakeReader) Search(_ context.Context, q domain.SearchRequest) (domain.IssuePage, error) {
	f.queries = append(f.queries, q)
	return f.search(q)
}
func (f *fakeReader) GetIssue(_ context.Context, ref domain.IssueRef, o domain.DetailOptions) (domain.IssueDetail, error) {
	f.detailCalls++
	if f.detail != nil {
		return f.detail(o)
	}
	return domain.IssueDetail{Issue: testIssue(1)}, nil
}
func testIssue(n int) domain.Issue {
	return domain.Issue{Ref: domain.IssueRef{ID: fmt.Sprint(n), Key: fmt.Sprintf("APP-%d", n)}, Summary: "Synthetic", Status: domain.Status{Category: domain.CategoryTodo}, URL: fmt.Sprintf("https://example.atlassian.net/browse/APP-%d", n)}
}
func testReader(f *fakeReader, m *cache.Memory) *Reader {
	p := pfixture("user@example.com")
	p.AccountID = "user"
	return NewReader(f, "work", p, ports.NewSecret("synthetic-token"), m)
}
func queryOptions() SearchOptions {
	return SearchOptions{Query: domain.SearchRequest{JQL: "project = APP", PageSize: 2, Fields: []string{"summary"}}, Limit: 2}
}
func TestCursorResumesPendingAndDeduplicates(t *testing.T) {
	f := &fakeReader{search: func(q domain.SearchRequest) (domain.IssuePage, error) {
		if q.PageToken == "" {
			return domain.IssuePage{Issues: []domain.Issue{testIssue(1), testIssue(2), testIssue(3)}, NextPageToken: "next"}, nil
		}
		if q.PageToken != "next" {
			t.Fatal(q.PageToken)
		}
		return domain.IssuePage{Issues: []domain.Issue{testIssue(3), testIssue(4)}, Complete: true}, nil
	}}
	r := testReader(f, nil)
	o := queryOptions()
	first, e := r.Search(context.Background(), o)
	if e != nil || first.Meta.Complete || len(first.Issues) != 2 || first.Meta.NextPageToken == "" || len(f.queries) != 1 {
		t.Fatal(first, e)
	}
	o.PageToken = first.Meta.NextPageToken
	second, e := r.Search(context.Background(), o)
	if e != nil || !second.Meta.Complete || len(second.Issues) != 2 || second.Issues[0].Ref.ID != "3" || second.Issues[1].Ref.ID != "4" || f.queries[1].PageSize != 1 {
		t.Fatal(second, e, f.queries)
	}
	for _, change := range []string{"query", "profile", "tamper", "fields"} {
		bad := o
		candidate := r
		switch change {
		case "query":
			bad.Query.JQL = "project = OTHER"
		case "profile":
			copy := *r
			copy.Scope = "other"
			candidate = &copy
		case "fields":
			bad.Query.Fields = []string{"status"}
		case "tamper":
			bad.PageToken = "x" + bad.PageToken
		}
		if _, e := candidate.Search(context.Background(), bad); e == nil {
			t.Fatal("accepted mismatched cursor", change)
		}
	}
}
func TestAllPartialFailureCapsAndRepeatedCursor(t *testing.T) {
	for _, kind := range []string{"failure", "repeat", "cap"} {
		t.Run(kind, func(t *testing.T) {
			f := &fakeReader{search: func(q domain.SearchRequest) (domain.IssuePage, error) {
				if q.PageToken == "" {
					return domain.IssuePage{Issues: []domain.Issue{testIssue(1)}, NextPageToken: "repeat"}, nil
				}
				if kind == "failure" {
					return domain.IssuePage{}, problem(domain.Forbidden, "Denied")
				}
				return domain.IssuePage{Issues: []domain.Issue{testIssue(2)}, NextPageToken: "repeat"}, nil
			}}
			r := testReader(f, nil)
			o := queryOptions()
			o.All = true
			o.MaxResults = 5
			if kind == "cap" {
				o.MaxResults = 1
			}
			result, e := r.Search(context.Background(), o)
			var public *domain.Error
			if !errors.As(e, &public) || public.Kind != domain.Partial || len(result.Issues) == 0 || result.Meta.Complete || len(f.queries) > 3 {
				t.Fatal(result, e, len(f.queries))
			}
		})
	}
}
func TestReadCacheTTLRefreshOfflineAndIdentityIsolation(t *testing.T) {
	now := time.Now()
	m := cache.New()
	m.Now = func() time.Time { return now }
	f := &fakeReader{search: func(domain.SearchRequest) (domain.IssuePage, error) {
		return domain.IssuePage{Issues: []domain.Issue{testIssue(1)}, Complete: true}, nil
	}}
	r := testReader(f, m)
	o := queryOptions()
	if _, e := r.Search(context.Background(), o); e != nil {
		t.Fatal(e)
	}
	second, e := r.Search(context.Background(), o)
	if e != nil || second.Meta.Source != "memory" || len(f.queries) != 1 || f.identityCalls != 1 {
		t.Fatal(second, e)
	}
	now = now.Add(61 * time.Second)
	o.Offline = true
	stale, e := r.Search(context.Background(), o)
	if e != nil || !stale.Meta.Stale {
		t.Fatal(stale, e)
	}
	o.Offline = false
	o.Refresh = true
	if _, e = r.Search(context.Background(), o); e != nil || len(f.queries) != 2 {
		t.Fatal(e)
	}
	f.identityErr = problem(domain.Authentication, "expired")
	if _, e = r.Search(context.Background(), o); e == nil {
		t.Fatal("cached auth failure")
	}
	o.Refresh = false
	o.Offline = true
	if _, e = r.Search(context.Background(), o); e == nil {
		t.Fatal("cache not invalidated")
	}
	other := testReader(f, m)
	other.Scope = "different-identity"
	if _, e = other.Search(context.Background(), o); e == nil {
		t.Fatal("identity cache leak")
	}
}
func TestShowCachePartialAndValidation(t *testing.T) {
	f := &fakeReader{}
	r := testReader(f, nil)
	o := domain.DetailOptions{IncludeDescription: true}
	first, e := r.Show(context.Background(), "APP-1", o, false, false)
	if e != nil || !first.Meta.Complete {
		t.Fatal(first, e)
	}
	if _, e = r.Show(context.Background(), "APP-1", o, false, false); e != nil || f.detailCalls != 1 {
		t.Fatal(e)
	}
	o.IncludeComments = true
	f.detail = func(domain.DetailOptions) (domain.IssueDetail, error) {
		return domain.IssueDetail{Issue: testIssue(1), Comments: &domain.CommentPage{}}, problem(domain.Partial, "comments unavailable")
	}
	partial, e := r.Show(context.Background(), "APP-1", o, false, false)
	if e == nil || partial.Detail.Issue.Ref.ID == "" || partial.Meta.Complete {
		t.Fatal(partial, e)
	}
	if _, e = r.Show(context.Background(), "../../etc", o, false, false); e == nil {
		t.Fatal("bad key accepted")
	}
}
func TestEmptyResultsAndBadTokenAreBounded(t *testing.T) {
	f := &fakeReader{search: func(domain.SearchRequest) (domain.IssuePage, error) {
		return domain.IssuePage{Issues: []domain.Issue{}, Complete: true}, nil
	}}
	r := testReader(f, nil)
	o := queryOptions()
	result, e := r.Search(context.Background(), o)
	if e != nil || !result.Meta.Complete || result.Issues == nil {
		t.Fatal(result, e)
	}
	o.PageToken = strings.Repeat("x", 1024*1024+1)
	if _, e = r.Search(context.Background(), o); e == nil {
		t.Fatal("oversized cursor")
	}
}

func TestCancellationKeepsExitCodeAndCollectedData(t *testing.T) {
	f := &fakeReader{search: func(q domain.SearchRequest) (domain.IssuePage, error) {
		if q.PageToken == "" {
			return domain.IssuePage{Issues: []domain.Issue{testIssue(1)}, NextPageToken: "next"}, nil
		}
		return domain.IssuePage{}, problem(domain.Canceled, "Canceled")
	}}
	r := testReader(f, nil)
	o := queryOptions()
	o.All = true
	result, e := r.Search(context.Background(), o)
	if domain.ExitCode(e) != 130 || len(result.Issues) != 1 || result.Meta.Complete {
		t.Fatal(result, e)
	}
}
func TestCursorChainCannotGrowPastIDCap(t *testing.T) {
	f := &fakeReader{search: func(q domain.SearchRequest) (domain.IssuePage, error) {
		t.Fatal("unnecessary page after cap")
		return domain.IssuePage{}, nil
	}}
	r := testReader(f, nil)
	o := queryOptions()
	scope := hash([]any{r.Scope, o.Query.JQL, o.Query.Fields})
	cursor := searchCursor{Version: 1, Scope: scope, Pending: []domain.Issue{testIssue(5000), testIssue(5001)}, Done: true}
	for i := 1; i < 5000; i++ {
		cursor.Seen = append(cursor.Seen, fmt.Sprint(i))
	}
	o.PageToken, _ = r.encodeCursor(cursor)
	result, e := r.Search(context.Background(), o)
	if domain.ExitCode(e) != 10 || len(result.Issues) != 1 {
		t.Fatal(result, e)
	}
	decoded, e := r.decodeCursor(result.Meta.NextPageToken, scope)
	if e != nil || len(decoded.Seen) != 5000 || len(decoded.Pending) != 1 {
		t.Fatal(e, len(decoded.Seen))
	}
}
