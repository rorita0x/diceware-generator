//go:build !linux

package harden

// Apply is a no-op outside Linux. memguard still protects the secrets.
func Apply() Status {
	return Status{CoreDumps: "unchanged", Dumpable: "unchanged", MlockAll: "unsupported"}
}
