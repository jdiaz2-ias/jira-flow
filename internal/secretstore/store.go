// Package secretstore provides cancelable OS credential-store subprocesses.
// Secret values travel through stdin/stdout, never process arguments.
package secretstore

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/domain"
	"jira-flow.local/jflow/internal/ports"
)

type Runner func(context.Context, string, []string, string) ([]byte, error)
type Store struct {
	OS  string
	Run Runner
}

func New() *Store { return &Store{OS: runtime.GOOS, Run: run} }
func run(ctx context.Context, path string, args []string, input string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Stdin = strings.NewReader(input)
	cmd.WaitDelay = time.Second
	// Never include stderr in an error: security may echo the interactive command.
	return cmd.Output()
}
func unavailable(ctx context.Context) error {
	if ctx.Err() != nil {
		return &domain.Error{Kind: domain.Canceled, Message: "Operation canceled."}
	}
	return &domain.Error{Kind: domain.Authentication, Message: "Secret store unavailable or credential missing. Unlock it or use JFLOW_TOKEN/--token-stdin with --no-store."}
}
func validate(ref ports.CredentialRef) error {
	if !config.ValidRef(string(ref)) {
		return &domain.Error{Kind: domain.InvalidInput, Message: "Invalid credential reference."}
	}
	return nil
}
func (s *Store) Get(ctx context.Context, ref ports.CredentialRef) (ports.Secret, error) {
	if err := validate(ref); err != nil {
		return ports.Secret{}, err
	}
	var b []byte
	var err error
	switch s.OS {
	case "darwin":
		b, err = s.Run(ctx, "/usr/bin/security", []string{"find-generic-password", "-s", "jflow", "-a", string(ref), "-w"}, "")
	case "linux":
		b, err = s.Run(ctx, "secret-tool", []string{"lookup", "service", "jflow", "account", string(ref)}, "")
	default:
		return ports.Secret{}, unavailable(ctx)
	}
	if err != nil {
		return ports.Secret{}, unavailable(ctx)
	}
	value := strings.TrimSuffix(string(b), "\n")
	// macOS stores our hex encoding to avoid interactive command escaping hazards.
	if s.OS == "darwin" {
		decoded, e := hex.DecodeString(value)
		if e != nil {
			return ports.Secret{}, unavailable(ctx)
		}
		value = string(decoded)
	}
	if value == "" {
		return ports.Secret{}, unavailable(ctx)
	}
	return ports.NewSecret(value), nil
}
func (s *Store) Set(ctx context.Context, ref ports.CredentialRef, secret ports.Secret) error {
	if err := validate(ref); err != nil {
		return err
	}
	value := secret.Reveal()
	if value == "" || len(value) > 1500 || strings.ContainsAny(value, "\x00\r\n") {
		return &domain.Error{Kind: domain.InvalidInput, Message: "Token is not persistable: maximum 1500 bytes, no line breaks. Use ephemeral mode for larger tokens."}
	}
	var err error
	switch s.OS {
	case "darwin":
		command := fmt.Sprintf("add-generic-password -U -s jflow -a %s -w %s\n", ref, hex.EncodeToString([]byte(value)))
		if len(command) >= 4096 {
			return &domain.Error{Kind: domain.InvalidInput, Message: "Token too large for Keychain."}
		}
		_, err = s.Run(ctx, "/usr/bin/security", []string{"-i"}, command)
		// Interactive security may exit successfully after reporting a failed command.
		// Verify persistence by reading back through the same bounded backend.
		if err == nil {
			var got ports.Secret
			got, err = s.Get(ctx, ref)
			if err == nil && !bytes.Equal([]byte(got.Reveal()), []byte(value)) {
				err = fmt.Errorf("verification failed")
			}
		}
	case "linux":
		_, err = s.Run(ctx, "secret-tool", []string{"store", "--label=Jira Flow", "service", "jflow", "account", string(ref)}, value)
	default:
		return unavailable(ctx)
	}
	if err != nil {
		return unavailable(ctx)
	}
	return nil
}
func (s *Store) Delete(ctx context.Context, ref ports.CredentialRef) error {
	if err := validate(ref); err != nil {
		return err
	}
	var err error
	switch s.OS {
	case "darwin":
		_, err = s.Run(ctx, "/usr/bin/security", []string{"delete-generic-password", "-s", "jflow", "-a", string(ref)}, "")
	case "linux":
		_, err = s.Run(ctx, "secret-tool", []string{"clear", "service", "jflow", "account", string(ref)}, "")
	default:
		return unavailable(ctx)
	}
	if s.OS == "darwin" {
		if e, ok := err.(*exec.ExitError); ok && e.ExitCode() == 44 {
			return nil
		}
	}
	if err != nil {
		return unavailable(ctx)
	}
	return nil
}
