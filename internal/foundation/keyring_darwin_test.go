//go:build foundation && darwin

package foundation

import (
	"crypto/rand"
	"encoding/hex"
	"os"
	"testing"

	"github.com/zalando/go-keyring"
)

// Opt-in only: touches a uniquely named synthetic item in the current keychain.
func TestMacKeychainRoundTrip(t *testing.T) {
	if os.Getenv("JFLOW_TEST_KEYCHAIN") != "1" {
		t.Skip("set JFLOW_TEST_KEYCHAIN=1 on a disposable/unlocked macOS keychain")
	}
	id := make([]byte, 16)
	if _, err := rand.Read(id); err != nil {
		t.Fatal(err)
	}
	service := "jflow-f0-test-" + hex.EncodeToString(id)
	secret := "synthetic 'quotes' \"unicode-á\"\nsecond-line"
	t.Cleanup(func() {
		if err := keyring.Delete(service, "fixture"); err != nil {
			t.Errorf("cleanup %s: %v", service, err)
		}
	})
	if err := keyring.Set(service, "fixture", secret); err != nil {
		t.Fatal(err)
	}
	got, err := keyring.Get(service, "fixture")
	if err != nil {
		t.Fatal(err)
	}
	if got != secret {
		t.Fatal("keychain round-trip mismatch")
	}
}
