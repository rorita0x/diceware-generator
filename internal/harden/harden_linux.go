//go:build linux

package harden

import (
	"fmt"

	"golang.org/x/sys/unix"
)

// Apply disables core dumps, marks the process non-dumpable (which also
// blocks ptrace attach and /proc/<pid>/mem from other non-root processes of
// the same user) and locks all memory so no page gets swapped out.
//
// mlockall needs either CAP_IPC_LOCK or an unlimited RLIMIT_MEMLOCK: the
// kernel compares the process's whole virtual size with the limit, and the
// Go runtime reserves far more address space (often >1 GiB) than the usual
// 8 MiB limit. Without either, it is skipped, because under a finite limit
// MCL_FUTURE would make later heap growth fail. memguard still mlocks the
// buffers that hold secrets, independent of this. The capability is kept
// for the whole run for the same reason: every new mapping is locked too.
//
// If requireLock is true and memory could not be locked, Apply returns an
// error wrapping ErrNotLocked.
func Apply(requireLock bool) (Status, error) {
	var s Status

	if err := unix.Setrlimit(unix.RLIMIT_CORE, &unix.Rlimit{Cur: 0, Max: 0}); err != nil {
		s.CoreDumps = "ON (" + err.Error() + ")"
	} else {
		s.CoreDumps = "off"
	}

	if err := unix.Prctl(unix.PR_SET_DUMPABLE, 0, 0, 0, 0); err != nil {
		s.Dumpable = "ON (" + err.Error() + ")"
	} else {
		s.Dumpable = "off"
	}

	reason := ""
	switch ok, why := canLockAll(); {
	case !ok:
		reason = why
	default:
		// MCL_ONFAULT locks pages as they are touched instead of faulting in
		// all reserved address space up front.
		if err := unix.Mlockall(unix.MCL_CURRENT | unix.MCL_FUTURE | unix.MCL_ONFAULT); err != nil {
			reason = "failed: " + err.Error()
		}
	}
	if reason == "" {
		s.MlockAll, s.Locked = "on", true
	} else {
		s.MlockAll = "skipped (" + reason + ")"
	}

	if requireLock && !s.Locked {
		return s, fmt.Errorf("%w: %s", ErrNotLocked, reason)
	}
	return s, nil
}

// canLockAll reports whether mlockall can be used safely, and if not, why.
func canLockAll() (bool, string) {
	if hasCapIPCLock() {
		return true, ""
	}
	var lim unix.Rlimit
	if err := unix.Getrlimit(unix.RLIMIT_MEMLOCK, &lim); err != nil {
		return false, "getrlimit: " + err.Error()
	}
	if lim.Cur == unix.RLIM_INFINITY {
		return true, ""
	}
	return false, "needs CAP_IPC_LOCK"
}

// hasCapIPCLock reports whether CAP_IPC_LOCK is in the effective set.
func hasCapIPCLock() bool {
	hdr := unix.CapUserHeader{Version: unix.LINUX_CAPABILITY_VERSION_3}
	var data [2]unix.CapUserData // version 3 uses two 32-bit words
	if err := unix.Capget(&hdr, &data[0]); err != nil {
		return false
	}
	return data[unix.CAP_IPC_LOCK/32].Effective&(1<<(unix.CAP_IPC_LOCK%32)) != 0
}
