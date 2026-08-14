//go:build darwin

package listen

import (
	"errors"
	"time"

	"golang.org/x/sys/unix"
)

const killWait = 2 * time.Second

// Kill sends SIGTERM, then SIGKILL only if the process start time still matches.
//
// macOS has no pidfd. verifyStart and unix.Kill are separate syscalls, so a
// process can theoretically exit and its PID be reused in that window. Using
// pbi_start_tvsec/usec and killing without os.FindProcess shrinks the window;
// it does not close it. See README "Limitations".
func Kill(id Ident) error {
	if err := requireIdent(id); err != nil {
		return err
	}
	if err := signalIfSame(id, unix.SIGTERM); err != nil {
		if errors.Is(err, unix.ESRCH) {
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
	if err := signalIfSame(id, unix.SIGKILL); err != nil && !errors.Is(err, unix.ESRCH) {
		return err
	}
	return nil
}

func signalIfSame(id Ident, sig unix.Signal) error {
	if err := verifyStart(id); err != nil {
		return err
	}
	return unix.Kill(id.PID, sig)
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
