package listen

import (
	"errors"
	"fmt"
	"os"
)

func checkKillPID(pid int) error {
	if pid <= 0 {
		return fmt.Errorf("invalid pid %d", pid)
	}
	if pid == 1 {
		return fmt.Errorf("refusing to kill pid 1")
	}
	if pid == os.Getpid() {
		return fmt.Errorf("refusing to kill self")
	}
	return nil
}

func requireIdent(id Ident) error {
	if err := checkKillPID(id.PID); err != nil {
		return err
	}
	if id.Start == 0 {
		return errNoIdentity
	}
	return nil
}

// KillAll terminates each process. Failures are joined; later PIDs are still attempted.
func KillAll(ids []Ident) error {
	var errs []error
	for _, id := range ids {
		if err := Kill(id); err != nil {
			errs = append(errs, fmt.Errorf("pid %d: %w", id.PID, err))
		}
	}
	return errors.Join(errs...)
}
