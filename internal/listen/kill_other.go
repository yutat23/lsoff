//go:build unix && !linux && !darwin

package listen

import (
	"errors"
	"os"
	"syscall"
	"time"
)

const killWait = 2 * time.Second

// Kill sends SIGTERM then SIGKILL, re-checking the start token before SIGKILL.
func Kill(id Ident) error {
	if err := requireIdent(id); err != nil {
		return err
	}
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
