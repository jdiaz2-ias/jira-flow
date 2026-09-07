package browser

import (
	"context"
	"errors"
	"testing"

	"jira-flow.local/jflow/internal/domain"
)

func TestPlatformLaunchersAndHeadless(t *testing.T) {
	for _, os := range []string{"darwin", "linux"} {
		called := false
		b := &Launcher{OS: os, Env: func(string) string { return "display" }, Run: func(ctx context.Context, program string, args ...string) error {
			called = true
			want := "/usr/bin/open"
			if os == "linux" {
				want = "xdg-open"
			}
			if program != want || len(args) != 1 || args[0] != "https://example.atlassian.net/browse/APP-1" {
				t.Fatal(program, args)
			}
			return nil
		}}
		if e := b.Open(context.Background(), "https://example.atlassian.net/browse/APP-1"); e != nil || !called {
			t.Fatal(e)
		}
	}
	b := &Launcher{OS: "linux", Env: func(string) string { return "" }, Run: func(context.Context, string, ...string) error { t.Fatal("headless launched"); return nil }}
	var e *domain.Error
	if err := b.Open(context.Background(), "https://example.atlassian.net/browse/APP-1"); !errors.As(err, &e) || e.Kind != domain.Unsupported {
		t.Fatal(err)
	}
}

func TestRejectsNonIssueURLsWithoutLaunching(t *testing.T) {
	b := &Launcher{OS: "darwin", Run: func(context.Context, string, ...string) error { t.Fatal("unsafe launch"); return nil }}
	for _, u := range []string{"-a Calculator", "javascript:alert(1)", "https://evil.example/browse/APP-1", "https://example.atlassian.net/browse/../APP-1"} {
		if e := b.Open(context.Background(), u); e == nil {
			t.Fatal(u)
		}
	}
}
