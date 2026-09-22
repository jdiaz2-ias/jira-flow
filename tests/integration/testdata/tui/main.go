// Synthetic executable used by the PTY integration test. Never contacts Jira,
// a real keyring, or a real browser.
package main

import (
	"context"
	"encoding/json"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"

	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/cli"
	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/ports"
)

type fixture struct {
	mu     sync.Mutex
	state  domain.Status
	fields map[string]any
	secret ports.Secret
}

func event(v any) {
	b, _ := json.Marshal(v)
	f, err := os.OpenFile(os.Getenv("JFLOW_PTY_LOG"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	f.Write(append(b, '\n'))
}
func (f *fixture) Myself(context.Context) (domain.User, error) {
	return domain.User{ID: "fixture-user", DisplayName: "Fixture"}, nil
}
func (f *fixture) Search(_ context.Context, q domain.SearchRequest) (domain.IssuePage, error) {
	event(map[string]string{"search": q.JQL})
	d, _ := f.GetIssue(context.Background(), domain.IssueRef{}, domain.DetailOptions{})
	return domain.IssuePage{Issues: []domain.Issue{d.Issue}, Complete: true}, nil
}
func (f *fixture) GetIssue(context.Context, domain.IssueRef, domain.DetailOptions) (domain.IssueDetail, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return domain.IssueDetail{Issue: domain.Issue{Ref: domain.IssueRef{ID: "1", Key: "APP-1"}, Summary: "PTY issue", ProjectID: "p", IssueTypeID: "t", Status: f.state, URL: "https://example.atlassian.net/browse/APP-1"}, Description: []domain.Block{{Kind: "paragraph", Text: "PTY description"}}, SubtasksComplete: true, Values: f.fields}, nil
}
func (f *fixture) ListTransitions(context.Context, domain.IssueRef) ([]domain.Transition, error) {
	return []domain.Transition{{ID: "start", Name: "Start", To: domain.Status{ID: "active", Name: "Active", Category: domain.CategoryInProgress}}, {ID: "done", Name: "Done", To: domain.Status{ID: "done", Name: "Done", Category: domain.CategoryDone}, Fields: []domain.FieldSpec{{ID: "resolution", Name: "Resolution", Type: "resolution", Required: true, AllowedValues: []domain.NamedID{{ID: "fixed", Name: "Fixed"}}}}}}, nil
}
func (f *fixture) ApplyTransition(ctx context.Context, r domain.TransitionRequest) (domain.ApplyResult, error) {
	if os.Getenv("JFLOW_PTY_BLOCK_WRITE") == "1" {
		event(map[string]string{"write_pending": r.TransitionID})
		<-ctx.Done()
		return domain.ApplyResult{State: domain.ApplyUnknown, Issue: r.Issue, TransitionID: r.TransitionID}, &domain.Error{Kind: domain.Uncertain, Message: "Synthetic canceled write"}
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	ts, _ := f.ListTransitions(context.Background(), r.Issue)
	for _, t := range ts {
		if t.ID == r.TransitionID {
			f.state = t.To
		}
	}
	f.fields = r.Fields
	event(map[string]any{"write": r.TransitionID, "fields": r.Fields})
	return domain.ApplyResult{State: domain.ApplyAcceptedUnverified, Issue: r.Issue, TransitionID: r.TransitionID}, nil
}
func (f *fixture) Open(_ context.Context, link string) error {
	event(map[string]string{"open": link})
	return nil
}
func (f *fixture) Get(context.Context, ports.CredentialRef) (ports.Secret, error) {
	return f.secret, nil
}
func (f *fixture) Set(_ context.Context, _ ports.CredentialRef, s ports.Secret) error {
	f.secret = s
	event(map[string]bool{"stored": true})
	return nil
}
func (f *fixture) Delete(context.Context, ports.CredentialRef) error { return nil }

type identity struct{ f *fixture }

func (i identity) Myself(_ context.Context, p config.Profile, _ ports.Secret) (domain.User, error) {
	event(map[string]string{"login": p.Auth.Method, "cloud": p.CloudID})
	return i.f.Myself(context.Background())
}
func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	f := &fixture{state: domain.Status{ID: "todo", Name: "Todo", Category: domain.CategoryTodo}}
	path := os.Getenv("JFLOW_CONFIG")
	if os.Getenv("JFLOW_PTY_SETUP") == "" {
		err := config.Update(ctx, path, func(c *config.Config) error {
			c.ActiveProfile = "fixture"
			c.Profiles["fixture"] = config.Profile{Provider: "jira-cloud", SiteURL: "https://example.atlassian.net", AccountID: "fixture-user", Auth: config.Auth{Email: "fixture@example.com", Method: "api-token-unscoped"}}
			return nil
		})
		if err != nil {
			panic(err)
		}
	}
	deps := cli.Dependencies{Env: func(k string) string {
		switch k {
		case "JFLOW_CONFIG":
			return path
		case "JFLOW_CACHE":
			return filepath.Join(filepath.Dir(path), "cache")
		case "JFLOW_PROFILE", "JFLOW_EMAIL", "JFLOW_PROJECT", "JFLOW_NO_INPUT":
			return ""
		case "JFLOW_TOKEN":
			if os.Getenv("JFLOW_PTY_SETUP") == "" {
				return "synthetic-token"
			}
			return ""
		}
		return os.Getenv(k)
	}, Secrets: f, Jira: identity{f}, Browser: f, IssueReader: func(config.Profile, ports.Secret) (ports.IssueReader, error) { return f, nil }, Workflow: func(config.Profile, ports.Secret) (cli.WorkflowSession, error) { return f, nil }}
	code := cli.RunWithDependencies(ctx, os.Args[1:], os.Stdin, os.Stdout, os.Stderr, app.VersionInfo{}, deps)
	os.Exit(code)
}
