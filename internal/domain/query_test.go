package domain

import (
	"strings"
	"testing"
)

func TestQueryBuilder(t *testing.T) {
	for _, tc := range []struct {
		o    QueryOptions
		want string
	}{{QueryOptions{Mode: "mine"}, "assignee = currentUser() AND statusCategory != Done ORDER BY updated DESC, key ASC"}, {QueryOptions{Mode: "mine", Category: "done"}, `assignee = currentUser() AND statusCategory = "Done" ORDER BY updated DESC, key ASC`}, {QueryOptions{Mode: "list", Project: `A" OR project = "B`, Sort: "-priority,-updated"}, `project = "A\" OR project = \"B" ORDER BY priority DESC, updated DESC, key ASC`}, {QueryOptions{Mode: "search", JQL: "project = APP"}, "project = APP"}} {
		got, e := tc.o.Build()
		if e != nil || got != tc.want {
			t.Fatalf("%+v got=%s want=%s err=%v", tc.o, got, tc.want, e)
		}
	}
	for _, o := range []QueryOptions{{Mode: "list"}, {Mode: "mine", Category: "closed"}, {Mode: "mine", Sort: "updated; DROP"}, {Mode: "mine", Project: "one\ntwo"}, {Mode: "mine", UpdatedSince: "2026-02-30"}, {Mode: "search", JQL: "x", Project: "APP"}, {Mode: "search"}} {
		if _, e := o.Build(); e == nil {
			t.Fatalf("accepted %+v", o)
		}
	}
}
func TestIssueKeysAndTerminalSanitization(t *testing.T) {
	key, e := IssueKey("app-123")
	if e != nil || key != "APP-123" {
		t.Fatal(key, e)
	}
	for _, key := range []string{"../APP-1", "APP-1?x=y", "-APP-1", "APP-0", "APP-1;open"} {
		if _, e := IssueKey(key); e == nil {
			t.Fatal(key)
		}
	}
	for _, tc := range []struct{ in, want string }{{"\x1b[31mred\x1b[0m", "red"}, {"\x1b]8;;https://evil\x07label\x1b]8;;\x07", "label"}, {"\x1b]52;c;data\x1b\\safe", "safe"}, {"\u009b31mred", "red"}, {"\u202evisible", "visible"}, {"safe\x1b]unterminated", "safe"}} {
		if got := CleanText(tc.in, true); got != tc.want {
			t.Fatalf("%q -> %q want %q", tc.in, got, tc.want)
		}
	}
	link, e := BrowseURL("https://example.atlassian.net", "app-123")
	if e != nil || !strings.HasSuffix(link, "/browse/APP-123") {
		t.Fatal(link, e)
	}
}
