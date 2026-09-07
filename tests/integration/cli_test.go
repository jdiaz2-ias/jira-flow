package integration

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestBinaryProcessContract(t *testing.T) {
	binary := filepath.Join(t.TempDir(), "jflow")
	build := exec.Command("go", "build", "-o", binary, "-ldflags=-X main.version=integration -X main.commit=fixture", "./cmd/jflow")
	build.Dir = "../.."
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	configPath := filepath.Join(t.TempDir(), "config.json")
	raw := `{"schema_version":1,"active_profile":"fixture","profiles":{"fixture":{"provider":"jira-cloud","site_url":"https://example.atlassian.net","auth":{"method":"api-token-unscoped","email":"fixture@example.com"}}}}`
	if err := os.WriteFile(configPath, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"version", "--format=json"}, 0},
		{[]string{"missing", "--format=json"}, 2},
		{[]string{"--format=json"}, 2},
		{[]string{"link", "app-123", "--format=json"}, 0},
		{[]string{"show", "../bad", "--format=json"}, 2},
		{[]string{"mine", "--page-size=0", "--format=json"}, 2},
		{[]string{"profile", "list", "--format=json"}, 0},
	} {
		cmd := exec.Command(binary, tc.args...)
		cmd.Env = append(os.Environ(), "TERM=dumb", "NO_COLOR=1", "JFLOW_CONFIG="+configPath, "JFLOW_PROFILE=fixture", "JFLOW_TOKEN=", "JFLOW_EMAIL=")
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		out, err := cmd.Output()
		code := 0
		if err != nil {
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatal(err)
			}
			code = exit.ExitCode()
		}
		if code != tc.code || !json.Valid(out) || stderr.Len() != 0 {
			t.Fatalf("args=%v code=%d stdout=%s stderr=%s", tc.args, code, out, stderr.String())
		}
		if tc.args[0] == "version" && !bytes.Contains(out, []byte(`"version":"integration"`)) {
			t.Fatalf("linker metadata missing: %s", out)
		}
	}
}

func TestCoreHasNoUIOrProviderDependencies(t *testing.T) {
	cmd := exec.Command("go", "list", "-deps", "./internal/domain", "./internal/ports", "./internal/app")
	cmd.Dir = "../.."
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, forbidden := range []string{"charm.land/", "github.com/spf13/", "/internal/provider/", "github.com/zalando/"} {
		if bytes.Contains(out, []byte(forbidden)) {
			t.Fatalf("core imports %s", forbidden)
		}
	}
}
