//go:build foundation

// Package foundation verifies F0 dependency compatibility without shipping a TUI
// or accessing a real keyring. The tag keeps future adapters out of the CLI binary.
package foundation

import (
	_ "charm.land/bubbles/v2/list"
	_ "charm.land/bubbletea/v2"
	_ "charm.land/lipgloss/v2"
	_ "github.com/zalando/go-keyring"
	_ "golang.org/x/term"
)
