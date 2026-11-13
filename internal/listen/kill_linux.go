//go:build linux

package listen

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const killWait = 2 * time.Second

// Kill sends SIGTERM via pidfd, then SIGKILL if the same process is still alive.
//
// A process that is already gone counts as killed, as on darwin: a row can go
// stale between listing and the kill (an auto-restarting dev server), and the
// user's goal is met either way. A PID now owned by someone else still fails.
func Kill(id Ident) error {
	if err := requireIdent(id); err != nil {
		return err
	}
	fd, err := unix.PidfdOpen(id.PID, 0)
	if err != nil {
		if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EOPNOTSUPP) {
			return killByStartToken(id)
		}
		return ignoreGone(err)
	}
	defer unix.Close(fd)

	// The pidfd pins the process, not the PID number: if it was reaped after
	// PidfdOpen, /proc/<pid> is either missing (gone) or someone else's
	// (mismatch). Signals below go through fd, so they cannot hit the new owner.
	if err := verifyStart(id); err != nil {
		return ignoreGone(err)
	}
	if err := unix.PidfdSendSignal(fd, unix.SIGTERM, nil, 0); err != nil {
		return ignoreGone(err)
	}
	deadline := time.Now().Add(killWait)
	for time.Now().Before(deadline) {
		if err := unix.PidfdSendSignal(fd, 0, nil, 0); err != nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return ignoreGone(unix.PidfdSendSignal(fd, unix.SIGKILL, nil, 0))
}

func killByStartToken(id Ident) error {
	if err := verifyStart(id); err != nil {
		return ignoreGone(err)
	}
	p, err := os.FindProcess(id.PID)
	if err != nil {
		return err
	}
	if err := p.Signal(syscall.SIGTERM); err != nil {
		return ignoreGone(err)
	}
	deadline := time.Now().Add(killWait)
	for time.Now().Before(deadline) {
		if err := verifyStart(id); err != nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := verifyStart(id); err != nil {
		return nil
	}
	// The process can still exit between the check above and this signal.
	return ignoreGone(p.Kill())
}

func verifyStart(id Ident) error {
	cur, err := procStartToken(id.PID)
	if err != nil {
		return err
	}
	if cur != id.Start {
		return errIdentityMismatch
	}
	return nil
}

// processGone reports whether err only says that the target process no
// longer exists. Each case is a "no such process" spelling on some kill path:
//   - ESRCH: pidfd_open, pidfd_send_signal and kill for a reaped process, and
//     a read of /proc/<pid>/stat whose task exited after the open.
//   - fs.ErrNotExist (ENOENT): /proc/<pid> vanished when the process was
//     reaped. hidepid could also hide it, but then no start token would have
//     been captured and requireIdent refuses first.
//   - os.ErrProcessDone: os.Process reports ESRCH this way.
//
// errIdentityMismatch, EPERM/EACCES and everything else are real failures.
func processGone(err error) bool {
	return errors.Is(err, unix.ESRCH) ||
		errors.Is(err, fs.ErrNotExist) ||
		errors.Is(err, os.ErrProcessDone)
}

func ignoreGone(err error) error {
	if processGone(err) {
		return nil
	}
	return err
}
