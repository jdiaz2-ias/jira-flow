package app

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/ports"
)

type memoryStore struct {
	values     map[ports.CredentialRef]ports.Secret
	fail       bool
	sets       int
	failDelete bool
}

func (s *memoryStore) Get(_ context.Context, r ports.CredentialRef) (ports.Secret, error) {
	v, ok := s.values[r]
	if !ok || s.fail {
		return ports.Secret{}, problem(domain.Authentication, "Keyring locked.")
	}
	return v, nil
}
func (s *memoryStore) Set(_ context.Context, r ports.CredentialRef, v ports.Secret) error {
	s.sets++
	if s.fail {
		return problem(domain.Authentication, "Keyring locked.")
	}
	s.values[r] = v
	return nil
}
func (s *memoryStore) Delete(_ context.Context, r ports.CredentialRef) error {
	if s.fail || s.failDelete {
		return problem(domain.Authentication, "Keyring locked.")
	}
	delete(s.values, r)
	return nil
}

type identityFunc func(context.Context, config.Profile, ports.Secret) (domain.User, error)

func (f identityFunc) Myself(ctx context.Context, p config.Profile, s ports.Secret) (domain.User, error) {
	return f(ctx, p, s)
}
func authFixture(t *testing.T) (Access, *memoryStore) {
	t.Helper()
	store := &memoryStore{values: map[ports.CredentialRef]ports.Secret{}}
	return Access{Path: filepath.Join(t.TempDir(), "config.json"), Env: func(string) string { return "" }, Secrets: store, Jira: identityFunc(func(ctx context.Context, p config.Profile, s ports.Secret) (domain.User, error) {
		if s.Reveal() == "bad" {
			return domain.User{}, problem(domain.Authentication, "Invalid credential.")
		}
		return domain.User{ID: p.Auth.Email, DisplayName: "Synthetic"}, nil
	})}, store
}
func pfixture(email string) config.Profile {
	return config.Profile{Provider: "jira-cloud", SiteURL: "https://example.atlassian.net", Auth: config.Auth{Method: "api-token-unscoped", Email: email}}
}
func TestLoginIsolationPrecedenceAndLogout(t *testing.T) {
	a, store := authFixture(t)
	ctx := context.Background()
	for _, n := range []string{"a", "b"} {
		if _, e := a.Login(ctx, n, pfixture(n+"@example.com"), ports.NewSecret("synthetic-secret-"+n), true); e != nil {
			t.Fatal(e)
		}
	}
	c, e := config.Load(a.Path)
	if e != nil || len(store.values) != 2 || c.Profiles["a"].Auth.CredentialRef == c.Profiles["b"].Auth.CredentialRef {
		t.Fatal(c, e)
	}
	raw, _ := os.ReadFile(a.Path)
	if strings.Contains(string(raw), "synthetic-secret") {
		t.Fatal("secret on disk")
	}
	if e = a.Use(ctx, "a"); e != nil {
		t.Fatal(e)
	}
	r, e := a.Me(ctx, "", "", ports.Secret{})
	if e != nil || r.AccountID != "a@example.com" {
		t.Fatal(r, e)
	}
	a.Env = func(k string) string {
		if k == "JFLOW_PROFILE" {
			return "b"
		}
		if k == "JFLOW_EMAIL" {
			return "override@example.com"
		}
		return ""
	}
	if _, e = a.Me(ctx, "", "", ports.Secret{}); e == nil {
		t.Fatal("identity mismatch accepted")
	}
	r, e = a.Me(ctx, "a", "a@example.com", ports.Secret{})
	if e != nil || r.Profile != "a" {
		t.Fatal("flag precedence", r, e)
	}
	a.Env = func(k string) string {
		if k == "JFLOW_TOKEN" {
			return "ephemeral"
		}
		return ""
	}
	status, e := a.Status(ctx, "a", "")
	if e != nil || status.Source != "environment" || status.Verified {
		t.Fatal(status, e)
	}
	result, e := a.Logout(ctx, "a")
	if e != nil || result["environment_token_present"] != true || len(store.values) != 1 {
		t.Fatal(result, e)
	}
	c, _ = config.Load(a.Path)
	if _, ok := c.Profiles["a"]; ok {
		t.Fatal("profile retained")
	}
	if c.Profiles["b"].Auth.Email != "b@example.com" {
		t.Fatal("other profile changed")
	}
}
func TestLoginFailureNeverPersistsAndEphemeralSkipsKeyring(t *testing.T) {
	a, s := authFixture(t)
	ctx := context.Background()
	if _, e := a.Login(ctx, "work", pfixture("u@example.com"), ports.NewSecret("bad"), true); e == nil {
		t.Fatal("bad auth accepted")
	}
	if s.sets != 0 {
		t.Fatal("stored invalid credential")
	}
	if _, e := os.Stat(a.Path); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("wrote failed login")
	}
	s.fail = true
	if _, e := a.Login(ctx, "work", pfixture("u@example.com"), ports.NewSecret("valid"), true); e == nil {
		t.Fatal("locked keyring accepted")
	}
	if _, e := os.Stat(a.Path); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("wrote locked login")
	}
	if _, e := a.Login(ctx, "work", pfixture("u@example.com"), ports.NewSecret("valid"), false); e != nil {
		t.Fatal(e)
	}
	c, _ := config.Load(a.Path)
	if c.Profiles["work"].Auth.CredentialRef != "" || len(s.values) != 0 {
		t.Fatal("ephemeral persisted")
	}
	if _, e := a.Status(ctx, "work", ""); e == nil {
		t.Fatal("missing credential accepted")
	}
}

func TestReplacingProfileCleansOldSecret(t *testing.T) {
	a, s := authFixture(t)
	ctx := context.Background()
	p := pfixture("u@example.com")
	if _, e := a.Login(ctx, "work", p, ports.NewSecret("first"), true); e != nil {
		t.Fatal(e)
	}
	before, _ := config.Load(a.Path)
	ref := ports.CredentialRef(before.Profiles["work"].Auth.CredentialRef)
	if _, e := a.Login(ctx, "work", p, ports.NewSecret("second"), true); e != nil {
		t.Fatal(e)
	}
	after, _ := config.Load(a.Path)
	if len(s.values) != 1 || after.Profiles["work"].Auth.CredentialRef == string(ref) || len(after.Profiles["work"].RetiredCredentialRefs) != 0 {
		t.Fatal("rotation did not clean up")
	}
	if _, ok := s.values[ref]; ok {
		t.Fatal("old secret retained")
	}
}

func TestCleanupFailureRemainsRecoverable(t *testing.T) {
	a, s := authFixture(t)
	ctx := context.Background()
	p := pfixture("u@example.com")
	if _, e := a.Login(ctx, "work", p, ports.NewSecret("first"), true); e != nil {
		t.Fatal(e)
	}
	s.failDelete = true
	if _, e := a.Login(ctx, "work", p, ports.NewSecret("second"), true); e == nil {
		t.Fatal("cleanup failure hidden")
	}
	c, e := config.Load(a.Path)
	if e != nil || len(c.Profiles["work"].RetiredCredentialRefs) != 1 || len(s.values) != 2 {
		t.Fatal("lost recovery reference", c, e)
	}
	s.failDelete = false
	if _, e := a.Logout(ctx, "work"); e != nil {
		t.Fatal(e)
	}
	if len(s.values) != 0 {
		t.Fatal("retired credential leaked")
	}
}

func TestLoginPreservesDefaultProject(t *testing.T) {
	a, _ := authFixture(t)
	p := pfixture("u@example.com")
	p.DefaultProject = "APP"
	ctx := context.Background()
	if _, e := a.Login(ctx, "work", p, ports.NewSecret("first"), false); e != nil {
		t.Fatal(e)
	}
	p.DefaultProject = ""
	if _, e := a.Login(ctx, "work", p, ports.NewSecret("second"), false); e != nil {
		t.Fatal(e)
	}
	c, e := config.Load(a.Path)
	if e != nil || c.Profiles["work"].DefaultProject != "APP" {
		t.Fatal(c, e)
	}
}
