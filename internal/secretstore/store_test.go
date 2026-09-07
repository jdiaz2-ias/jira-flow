package secretstore

import (
	"context"
	"encoding/hex"
	"errors"
	"strings"
	"testing"

	"jira-flow.local/jflow/internal/ports"
)

const ref ports.CredentialRef = "0123456789abcdef0123456789abcdef"

func TestMacTokenNeverInArgvAndRoundTrip(t *testing.T) {
	token := `synthetic-"'\-secret`
	calls := 0
	s := &Store{OS: "darwin", Run: func(ctx context.Context, path string, args []string, in string) ([]byte, error) {
		calls++
		if path != "/usr/bin/security" || strings.Contains(strings.Join(args, " "), token) {
			t.Fatal("secret in argv")
		}
		if args[0] == "-i" {
			if len(in) >= 4096 || !strings.Contains(in, hex.EncodeToString([]byte(token))) {
				t.Fatal("bad stdin command")
			}
			return nil, nil
		}
		if args[0] == "find-generic-password" {
			return []byte(hex.EncodeToString([]byte(token)) + "\n"), nil
		}
		return nil, nil
	}}
	if e := s.Set(context.Background(), ref, ports.NewSecret(token)); e != nil {
		t.Fatal(e)
	}
	got, e := s.Get(context.Background(), ref)
	if e != nil || got.Reveal() != token {
		t.Fatal("round trip", e)
	}
	if e = s.Delete(context.Background(), ref); e != nil {
		t.Fatal(e)
	}
	if calls != 4 {
		t.Fatal(calls)
	}
}
func TestRejectBeforeSpawningAndRedactBackendErrors(t *testing.T) {
	calls := 0
	s := &Store{OS: "darwin", Run: func(context.Context, string, []string, string) ([]byte, error) {
		calls++
		return nil, errors.New("synthetic-secret")
	}}
	for _, token := range []string{"", strings.Repeat("x", 1501), "one\ntwo"} {
		if e := s.Set(context.Background(), ref, ports.NewSecret(token)); e == nil {
			t.Fatal("invalid token accepted")
		}
	}
	if calls != 0 {
		t.Fatal("spawned before validation")
	}
	e := s.Set(context.Background(), ref, ports.NewSecret("synthetic-secret"))
	if e == nil || strings.Contains(e.Error(), "synthetic-secret") {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if e = s.Delete(ctx, ref); e == nil || e.Error() != "Operation canceled." {
		t.Fatal(e)
	}
}
func TestLinuxStdinSecretService(t *testing.T) {
	s := &Store{OS: "linux", Run: func(ctx context.Context, path string, args []string, in string) ([]byte, error) {
		if path != "secret-tool" || args[0] != "store" || in != "synthetic-secret" || strings.Contains(strings.Join(args, " "), in) {
			t.Fatal("bad Secret Service invocation")
		}
		return nil, nil
	}}
	if e := s.Set(context.Background(), ref, ports.NewSecret("synthetic-secret")); e != nil {
		t.Fatal(e)
	}
}
