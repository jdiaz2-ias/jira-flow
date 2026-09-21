package cli

import "testing"

func TestUIRejectsNonInteractiveBeforeAccess(t *testing.T) {
	for _, args := range [][]string{{"ui"}, {"ui", "--no-input"}, {"ui", "--format=json"}, {"ui", "--format=plain"}} {
		deps, reader, _ := readingFixture(t)
		runReading(t, deps, args, 2)
		if reader.identityCalls != 0 || len(reader.queries) != 0 {
			t.Fatal("noninteractive UI contacted Jira")
		}
	}
}
