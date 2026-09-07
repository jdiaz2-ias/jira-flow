package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"jira-flow.local/jflow/internal/app"
)

func TestCommandContract(t *testing.T) {
	info := app.VersionInfo{Version: "test", Commit: "abc123", GoVersion: "go-test", OS: "linux", Arch: "amd64"}
	for _, tc := range []struct {
		name     string
		args     []string
		code     int
		json     bool
		contains string
	}{
		{"help", []string{"--help"}, 0, false, "version"},
		{"version", []string{"version"}, 0, false, "jflow test"},
		{"json", []string{"version", "--format", "json"}, 0, true, "go_version"},
		{"json-before", []string{"--format=json", "version"}, 0, true, "abc123"},
		{"json-help", []string{"--help", "--format=json"}, 0, true, "help"},
		{"help-version", []string{"help", "version", "--format=json"}, 0, true, "help"},
		{"empty", nil, 2, false, ""},
		{"empty-json", []string{"--format=json"}, 2, true, "invalid_input"},
		{"unknown", []string{"mine", "--format=json"}, 2, true, "invalid_input"},
		{"unknown-flag", []string{"version", "--no-such-flag", "--format=json"}, 2, true, "invalid_input"},
		{"unknown-help", []string{"help", "missing", "--format=json"}, 2, true, "invalid_input"},
		{"parse-stops-early", []string{"--format=plain", "version", "--bad", "--format=json"}, 2, true, "invalid_input"},
		{"extra-arg", []string{"version", "extra", "--format=json"}, 2, true, "invalid_input"},
		{"bad-format", []string{"version", "--format=xml"}, 2, false, ""},
		{"bad-help-format", []string{"--help", "--format=xml"}, 2, false, ""},
		{"last-format", []string{"version", "--format=plain", "--format=json"}, 0, true, "go_version"},
		{"terminator", []string{"version", "--", "--format=json"}, 2, false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, diagnostic bytes.Buffer
			code := Run(context.Background(), tc.args, strings.NewReader(""), &out, &diagnostic, info)
			if code != tc.code {
				t.Fatalf("code=%d want=%d out=%s stderr=%s", code, tc.code, out.String(), diagnostic.String())
			}
			if !strings.Contains(out.String(), tc.contains) {
				t.Errorf("missing %q: %s", tc.contains, out.String())
			}
			if tc.json {
				dec := json.NewDecoder(&out)
				var envelope map[string]any
				if err := dec.Decode(&envelope); err != nil {
					t.Fatal(err)
				}
				if envelope["schema_version"] != float64(1) || envelope["ok"] != (tc.code == 0) {
					t.Fatalf("invalid envelope: %#v", envelope)
				}
				if err := dec.Decode(new(any)); !errors.Is(err, io.EOF) {
					t.Fatalf("expected exactly one JSON document: %v", err)
				}
				if diagnostic.Len() != 0 {
					t.Fatalf("unexpected stderr: %s", diagnostic.String())
				}
			} else if tc.code != 0 && diagnostic.Len() == 0 {
				t.Error("expected diagnostic")
			}
		})
	}
}

func TestCanceledCommand(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var out bytes.Buffer
	if code := Run(ctx, []string{"version", "--format=json"}, strings.NewReader(""), &out, io.Discard, app.VersionInfo{}); code != 130 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(out.String(), `"code":"canceled"`) {
		t.Fatal(out.String())
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestWriteFailureIsNotSuccess(t *testing.T) {
	for _, args := range [][]string{{"version"}, {"version", "--format=json"}, {"--help"}} {
		if code := Run(context.Background(), args, strings.NewReader(""), brokenWriter{}, io.Discard, app.VersionInfo{}); code != 1 {
			t.Fatalf("%v: code=%d", args, code)
		}
	}
}

func TestParseErrorsDoNotEchoArbitraryInput(t *testing.T) {
	var out, diagnostic bytes.Buffer
	Run(context.Background(), []string{"private-synthetic-input", "--format=json"}, strings.NewReader(""), &out, &diagnostic, app.VersionInfo{})
	if strings.Contains(out.String()+diagnostic.String(), "private-synthetic-input") {
		t.Fatal("raw argument was exposed")
	}
}
