package integration

import (
	"context"
	_ "jira-flow.local/jflow/internal/cli"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestInteractiveKeyboardJourney(t *testing.T) {
	// Record fixture reads in the Go test cache; the CLI import also tracks
	// changes in code compiled by the child go-build command.
	for _, path := range []string{"testdata/tui/main.go", "testdata/tui/terminal_journey.py"} {
		if _, err := os.ReadFile(path); err != nil {
			t.Fatal(err)
		}
	}
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("python3 is required for the Linux/macOS PTY acceptance harness")
	}
	binary := filepath.Join(t.TempDir(), "tui-fixture")
	build := exec.Command("go", "build", "-race", "-o", binary, "./tests/integration/testdata/tui")
	build.Dir = "../.."
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("fixture build: %v\n%s", err, output)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, python, "testdata/tui/terminal_journey.py", binary)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("PTY journey: %v\n%s", err, output)
	} else {
		t.Log(string(output))
	}
}
