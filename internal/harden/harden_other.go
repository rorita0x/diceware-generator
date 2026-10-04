//go:build !linux

package harden

import "fmt"

// Apply is a no-op outside Linux. memguard still protects the secrets.
func Apply(requireLock bool) (Status, error) {
	s := Status{CoreDumps: "unchanged", Dumpable: "unchanged", MlockAll: "unsupported"}
	if requireLock {
		return s, fmt.Errorf("%w: only supported on Linux", ErrNotLocked)
	}
	return s, nil
}
