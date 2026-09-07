package ports

import (
	"context"
	"fmt"
	"io"
	"log/slog"
)

// CredentialRef is an opaque per-profile identifier, never an email or a token.
type CredentialRef string

type Secret struct{ value string }

func NewSecret(value string) Secret { return Secret{value: value} }

// Reveal is only for authentication and secret-store adapters.
func (s Secret) Reveal() string               { return s.value }
func (Secret) String() string                 { return "[REDACTED]" }
func (Secret) Format(state fmt.State, _ rune) { _, _ = io.WriteString(state, "[REDACTED]") }
func (Secret) MarshalJSON() ([]byte, error)   { return []byte(`"[REDACTED]"`), nil }
func (Secret) LogValue() slog.Value           { return slog.StringValue("[REDACTED]") }

type SecretStore interface {
	Get(context.Context, CredentialRef) (Secret, error)
	Set(context.Context, CredentialRef, Secret) error
	Delete(context.Context, CredentialRef) error
}
