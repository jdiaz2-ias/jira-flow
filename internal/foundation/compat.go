//go:build foundation

// Package foundation verifies pinned dependency compatibility without accessing
// a real keyring. Bubbles and keyring audit imports remain behind this tag.
package foundation

import (
	_ "charm.land/bubbles/v2/list"
	_ "github.com/zalando/go-keyring"
)
