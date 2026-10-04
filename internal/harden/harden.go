// Package harden applies process-level memory protections.
package harden

import (
	"errors"
	"strings"
)

// ErrNotLocked is returned by Apply when locking all memory was required
// but could not be done.
var ErrNotLocked = errors.New("could not lock process memory (mlockall)")

// Status reports which protections are active. Each field holds a short
// human-readable state for the TUI footer.
type Status struct {
	CoreDumps string // "off" when RLIMIT_CORE was set to 0
	Dumpable  string // "off" when PR_SET_DUMPABLE is 0 (no ptrace, no /proc/pid/mem)
	MlockAll  string // "on" or why it was skipped
	// Locked is true when mlockall succeeded: no page of the process can
	// be swapped out.
	Locked bool
}

func (s Status) String() string {
	parts := []string{
		"core dumps " + s.CoreDumps,
		"ptrace/dumpable " + s.Dumpable,
		"mlockall " + s.MlockAll,
	}
	return strings.Join(parts, " · ")
}
