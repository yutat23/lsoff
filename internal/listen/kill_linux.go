//go:build linux

package listen

import (
	"errors"
	"os"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

const killWait = 2 * time.Second

// Kill sends SIGTERM via pidfd, then SIGKILL if the same process is still alive.
func Kill(id Ident) error {
	if err := requireIdent(id); err != nil {
		return err
	}
	fd, err := unix.PidfdOpen(id.PID, 0)
	if err != nil {
		if errors.Is(err, unix.ENOSYS) || errors.Is(err, unix.EOPNOTSUPP) {
			return killByStartToken(id)
		}
		return err
	}
	defer unix.Close(fd)

	if err := verifyStart(id); err != nil {
		return err
	}
	if err := unix.PidfdSendSignal(fd, unix.SIGTERM, nil, 0); err != nil {
		if errors.Is(err, unix.ESRCH) {
			return nil
		}
		return err
	}
	deadline := time.Now().Add(killWait)
	for time.Now().Before(deadline) {
		if err := unix.PidfdSendSignal(fd, 0, nil, 0); err != nil {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	if err := unix.PidfdSendSignal(fd, unix.SIGKILL, nil, 0); err != nil && !errors.Is(err, unix.ESRCH) {
		return err
	}
	return nil
}

func killByStartToken(id Ident) error {
	if err := verifyStart(id); err != nil {
		return err
	}
	p, err := os.FindProcess(id.PID)
	if err != nil {
		return err
	}
	if err := p.Signal(syscall.SIGTERM); err != nil {
		if errors.Is(err, os.ErrProcessDone) || errors.Is(err, syscall.ESRCH) {
			return nil
		}
		return err
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
	return p.Kill()
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
