//go:build linux

package harden

import (
	"golang.org/x/sys/unix"
)

// Apply disables core dumps, marks the process non-dumpable (which also
// blocks ptrace attach and /proc/<pid>/mem from other non-root processes of
// the same user) and, if the memlock limit allows it, locks all memory so
// nothing gets swapped out.
//
// mlockall is only attempted with an unlimited RLIMIT_MEMLOCK: the Go
// runtime reserves far more virtual memory than the usual 8 MiB limit, and
// MCL_FUTURE would make later heap growth fail. memguard still mlocks the
// buffers that hold secrets, independent of this.
func Apply() Status {
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

	var lim unix.Rlimit
	switch err := unix.Getrlimit(unix.RLIMIT_MEMLOCK, &lim); {
	case err != nil:
		s.MlockAll = "skipped (" + err.Error() + ")"
	case lim.Cur != unix.RLIM_INFINITY:
		s.MlockAll = "skipped (memlock limit; secrets still locked)"
	default:
		if err := unix.Mlockall(unix.MCL_CURRENT | unix.MCL_FUTURE); err != nil {
			s.MlockAll = "failed (" + err.Error() + ")"
		} else {
			s.MlockAll = "on"
		}
	}
	return s
}
