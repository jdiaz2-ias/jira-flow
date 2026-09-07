package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"jira-flow.local/jflow/internal/app"
	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/ports"
)

type fakeIdentity struct{}

func (fakeIdentity) Myself(_ context.Context, p config.Profile, s ports.Secret) (domain.User, error) {
	if s.Reveal() != "synthetic-token" {
		return domain.User{}, &domain.Error{Kind: domain.Authentication, Message: "Credencial inválida."}
	}
	return domain.User{ID: "u-123", DisplayName: "Test User"}, nil
}

type noSecrets struct{ t *testing.T }

func (s noSecrets) Get(context.Context, ports.CredentialRef) (ports.Secret, error) {
	s.t.Fatal("unexpected keyring access")
	return ports.Secret{}, nil
}
func (s noSecrets) Set(context.Context, ports.CredentialRef, ports.Secret) error {
	s.t.Fatal("unexpected keyring write")
	return nil
}
func (s noSecrets) Delete(context.Context, ports.CredentialRef) error {
	s.t.Fatal("unexpected keyring deletion")
	return nil
}
func TestFullEphemeralCommandFlow(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	env := map[string]string{"JFLOW_CONFIG": path, "JFLOW_TOKEN": "synthetic-token"}
	deps := Dependencies{Env: func(k string) string { return env[k] }, Jira: fakeIdentity{}, Secrets: noSecrets{t}}
	run := func(args []string, input string, want int) map[string]any {
		t.Helper()
		var out, stderr bytes.Buffer
		code := RunWithDependencies(context.Background(), append(args, "--format=json"), strings.NewReader(input), &out, &stderr, app.VersionInfo{}, deps)
		if code != want || stderr.Len() != 0 || strings.Contains(out.String(), "synthetic-token") {
			t.Fatalf("args=%v code=%d out=%s err=%s", args, code, &out, &stderr)
		}
		dec := json.NewDecoder(&out)
		var v map[string]any
		if e := dec.Decode(&v); e != nil {
			t.Fatal(e)
		}
		if dec.Decode(new(any)) != io.EOF {
			t.Fatal("multiple JSON documents")
		}
		return v
	}
	run([]string{"auth", "login", "--profile=work", "--site=https://example.atlassian.net", "--email=user@example.com", "--no-input"}, "", 0)
	raw, _ := os.ReadFile(path)
	if strings.Contains(string(raw), "synthetic-token") {
		t.Fatal("environment secret persisted")
	}
	for _, args := range [][]string{{"profile", "list"}, {"profile", "use", "work"}, {"auth", "status"}, {"me"}, {"doctor"}, {"config", "path"}, {"config", "validate"}, {"help", "auth", "login"}} {
		run(args, "", 0)
	}
	delete(env, "JFLOW_TOKEN")
	run([]string{"me"}, "", 3)
	run([]string{"me", "--token-stdin"}, "synthetic-token\n", 0)
	run([]string{"me", "--token-stdin"}, "wrong", 3)
	run([]string{"auth", "login", "--token=synthetic-token"}, "", 2)
	run([]string{"auth", "logout"}, "", 0)
	run([]string{"profile", "use", "work"}, "", 2)
}
func TestTokenInputBounds(t *testing.T) {
	for _, input := range []string{"", "one\ntwo", strings.Repeat("a", 65537), "one\x00two"} {
		if _, e := readToken(strings.NewReader(input)); e == nil {
			t.Fatal("accepted invalid token")
		}
	}
	s, e := readToken(strings.NewReader("valid\r\n"))
	if e != nil || s.Reveal() != "valid" {
		t.Fatal(e)
	}
}
func TestAccessWriteFailure(t *testing.T) {
	deps := Dependencies{Env: func(k string) string {
		if k == "JFLOW_CONFIG" {
			return filepath.Join(t.TempDir(), "config.json")
		}
		return ""
	}, Jira: fakeIdentity{}}
	code := RunWithDependencies(context.Background(), []string{"profile", "list", "--format=json"}, strings.NewReader(""), brokenWriter{}, io.Discard, app.VersionInfo{}, deps)
	if code != 1 {
		t.Fatal(code)
	}
}
