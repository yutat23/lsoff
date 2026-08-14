//go:build windows

package listen

import "golang.org/x/sys/windows"

// Kill terminates the process via TerminateProcess after verifying CreationTime.
func Kill(id Ident) error {
	if err := requireIdent(id); err != nil {
		return err
	}
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE|windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(id.PID))
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)

	cur, err := processStartToken(h)
	if err != nil {
		return err
	}
	if cur != id.Start {
		return errIdentityMismatch
	}
	return windows.TerminateProcess(h, 1)
}
