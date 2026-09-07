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
	for _, tc := range []struct {
		args []string
		code int
	}{
		{[]string{"version", "--format=json"}, 0},
		{[]string{"missing", "--format=json"}, 2},
		{[]string{"--format=json"}, 2},
	} {
		cmd := exec.Command(binary, tc.args...)
		cmd.Env = append(os.Environ(), "TERM=dumb", "NO_COLOR=1")
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
		if tc.code == 0 && !bytes.Contains(out, []byte(`"version":"integration"`)) {
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
