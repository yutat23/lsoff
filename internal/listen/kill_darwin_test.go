//go:build darwin

package listen

import (
	"errors"
	"os/exec"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

// Killing a process that already exited is not a failure: the goal is that the
// process is gone, and it is. Linux treats ESRCH that way; darwin must match.
func TestKillExitedProcessIsNoop(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	tok, err := procStartToken(pid)
	if err != nil {
		t.Fatal(err)
	}
	id := Ident{PID: pid, Start: tok}
	if err := Kill(id); err != nil {
		t.Fatalf("first kill: %v", err)
	}
	_ = cmd.Wait()
	time.Sleep(100 * time.Millisecond)
	if err := Kill(id); err != nil {
		t.Errorf("kill of exited process: %v", err)
	}
}

func TestProcStartTokenMissingProcessIsESRCH(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait()
	time.Sleep(100 * time.Millisecond)

	_, err := procStartToken(pid)
	if err == nil {
		t.Fatalf("procStartToken(%d) succeeded for an exited process", pid)
	}
	if !errors.Is(err, unix.ESRCH) {
		t.Fatalf("procStartToken(%d) = %v, want an ESRCH error", pid, err)
	}
}

// A recycled PID must still be refused: "gone" is success, "someone else" is not.
func TestKillRefusesMismatchedStartToken(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	tok, err := procStartToken(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	err = Kill(Ident{PID: cmd.Process.Pid, Start: tok + 1})
	if !errors.Is(err, errIdentityMismatch) {
		t.Fatalf("Kill with a stale start token = %v, want errIdentityMismatch", err)
	}
	if cmd.Process.Signal(unix.Signal(0)) != nil {
		t.Error("process was killed despite the identity mismatch")
	}
}

func TestKillRequiresIdentity(t *testing.T) {
	if err := Kill(Ident{PID: 12345, Start: 0}); !errors.Is(err, errNoIdentity) {
		t.Fatalf("Kill without a start token = %v, want errNoIdentity", err)
	}
}
