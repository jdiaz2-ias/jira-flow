package secretstore

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"
	"time"

	"jira-flow.local/jflow/internal/ports"
)

func TestNativeKeychainRoundTrip(t *testing.T) {
	if os.Getenv("JFLOW_TEST_KEYCHAIN") != "1" {
		t.Skip("opt-in: synthetic Keychain item")
	}
	b := make([]byte, 16)
	if _, e := rand.Read(b); e != nil {
		t.Fatal(e)
	}
	ref := ports.CredentialRef(hex.EncodeToString(b))
	s := New()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if e := s.Delete(ctx, ref); e != nil {
			t.Error(e)
		}
	})
	token := ports.NewSecret(`synthetic-'quotes'-"double"-á-\-fixture`)
	if e := s.Set(ctx, ref, token); e != nil {
		t.Fatal(e)
	}
	got, e := s.Get(ctx, ref)
	if e != nil || got.Reveal() != token.Reveal() {
		t.Fatal("round trip mismatch", e)
	}
	if e = s.Delete(ctx, ref); e != nil {
		t.Fatal(e)
	}
	if _, e = s.Get(ctx, ref); e == nil {
		t.Fatal("secret retained after delete")
	}
}
