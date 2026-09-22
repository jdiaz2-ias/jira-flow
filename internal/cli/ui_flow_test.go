package cli

import (
	"context"
	"io"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"jira-flow.local/jflow/internal/config"
	"jira-flow.local/jflow/internal/ports"
	"jira-flow.local/jflow/internal/tui"
)

func TestUIRootAndExplicitEntryShareWorkflow(t *testing.T) {
	for _, args := range [][]string{{"--theme=mono"}, {"ui", "--theme=mono", "--no-record"}} {
		deps, f := workflowCLI(t)
		deps.Interactive = func() bool { return true }
		deps.IssueReader = func(config.Profile, ports.Secret) (ports.IssueReader, error) { return &cliReader{}, nil }
		ran := false
		deps.RunUI = func(_ context.Context, m *tui.Model, _ io.Reader, _ io.Writer) error {
			ran = true
			m.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
			m.Update(m.Init()())
			_, prepare := m.Update(tea.KeyPressMsg{Code: 's', Text: "s"})
			m.Update(prepare())
			if f.writes != 0 {
				t.Fatal("preparation mutated")
			}
			_, apply := m.Update(tea.KeyPressMsg{Code: 'y', Text: "y"})
			m.Update(apply())
			if f.writes != 1 {
				t.Fatal("single confirmed write missing")
			}
			return nil
		}
		runReading(t, deps, args, 0)
		if !ran {
			t.Fatal("UI not entered")
		}
	}
}
func TestAccessibleUIUsesPlainReaderWithoutWorkflow(t *testing.T) {
	deps, reader, _ := readingFixture(t)
	deps.RunUI = func(context.Context, *tui.Model, io.Reader, io.Writer) error {
		t.Fatal("full screen opened")
		return nil
	}
	_, out := runReading(t, deps, []string{"ui", "--accessible", "--no-input"}, 0)
	if !strings.Contains(out, "APP-1") || len(reader.queries) == 0 {
		t.Fatal("missing accessible listing")
	}
}
func TestUIEnvironmentGuards(t *testing.T) {
	for _, envVar := range []string{"TERM", "JFLOW_NO_INPUT"} {
		deps, reader, _ := readingFixture(t)
		original := deps.Env
		deps.Env = func(k string) string {
			if k == envVar {
				if k == "TERM" {
					return "dumb"
				}
				return "true"
			}
			return original(k)
		}
		deps.Interactive = func() bool { return true }
		runReading(t, deps, []string{"ui"}, 2)
		if len(reader.queries) != 0 || reader.identityCalls != 0 {
			t.Fatal("guard contacted Jira")
		}
	}
}
