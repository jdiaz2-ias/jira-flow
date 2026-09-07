package ports

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"testing"
)

func TestSecretRedaction(t *testing.T) {
	secret := NewSecret("synthetic-do-not-display")
	if secret.Reveal() != "synthetic-do-not-display" {
		t.Fatal("adapter access broken")
	}
	var log bytes.Buffer
	slog.New(slog.NewJSONHandler(&log, nil)).Info("test", "secret", secret)
	encoded, err := json.Marshal(struct{ Token Secret }{secret})
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{fmt.Sprintf("%v %#v %s %q", secret, secret, secret, secret), string(encoded), log.String()} {
		if strings.Contains(value, secret.Reveal()) {
			t.Fatalf("secret leaked: %s", value)
		}
		if !strings.Contains(value, "REDACTED") {
			t.Fatal("missing redaction")
		}
	}
}
