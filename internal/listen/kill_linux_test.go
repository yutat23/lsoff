//go:build linux

package listen

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestProcessGone(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"ESRCH", unix.ESRCH, true},
		{"wrapped ESRCH", fmt.Errorf("pidfd_open: %w", unix.ESRCH), true},
		{"stat read ESRCH", &fs.PathError{Op: "read", Path: "/proc/42/stat", Err: syscall.ESRCH}, true},
		{"stat missing", &fs.PathError{Op: "open", Path: "/proc/42/stat", Err: syscall.ENOENT}, true},
		{"process done", os.ErrProcessDone, true},
		{"identity mismatch", errIdentityMismatch, false},
		{"no identity", errNoIdentity, false},
		{"EPERM", unix.EPERM, false},
		{"EACCES", unix.EACCES, false},
		{"stat EACCES", &fs.PathError{Op: "open", Path: "/proc/42/stat", Err: syscall.EACCES}, false},
		{"EINVAL", unix.EINVAL, false},
		{"ENOSYS", unix.ENOSYS, false},
		{"malformed stat", errors.New("malformed /proc/stat"), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := processGone(tt.err); got != tt.want {
				t.Errorf("processGone(%v) = %v, want %v", tt.err, got, tt.want)
			}
			wantErr := tt.err
			if tt.want {
				wantErr = nil
			}
			if got := ignoreGone(tt.err); got != wantErr {
				t.Errorf("ignoreGone(%v) = %v, want %v", tt.err, got, wantErr)
			}
		})
	}
}

// startSleeper starts a child that outlives the test unless it is killed. The
// returned wait reaps it once and may be called from any goroutine; cleanup
// kills and reaps the child if the test did not.
func startSleeper(t *testing.T) (*exec.Cmd, Ident, func() error) {
	t.Helper()
	cmd := exec.Command("sleep", "60")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	var once sync.Once
	var waitErr error
	wait := func() error {
		once.Do(func() { waitErr = cmd.Wait() })
		return waitErr
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = wait()
	})
	tok, err := procStartToken(cmd.Process.Pid)
	if err != nil {
		t.Fatal(err)
	}
	return cmd, Ident{PID: cmd.Process.Pid, Start: tok}, wait
}

// startReaper reaps the child in the background, as a real parent would, so
// Kill sees it disappear instead of lingering as a zombie.
func startReaper(wait func() error) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		_ = wait()
		close(done)
	}()
	return done
}

func requireSignaled(t *testing.T, cmd *exec.Cmd, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("child still running after Kill returned")
	}
	ws, ok := cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ok || !ws.Signaled() {
		t.Fatalf("child was not killed by a signal: %v", cmd.ProcessState)
	}
}

func TestKillTerminatesLiveProcess(t *testing.T) {
	cmd, id, wait := startSleeper(t)
	done := startReaper(wait)
	if err := Kill(id); err != nil {
		t.Fatalf("Kill: %v", err)
	}
	requireSignaled(t, cmd, done)
}

func TestKillByStartTokenTerminatesLiveProcess(t *testing.T) {
	cmd, id, wait := startSleeper(t)
	done := startReaper(wait)
	if err := killByStartToken(id); err != nil {
		t.Fatalf("killByStartToken: %v", err)
	}
	requireSignaled(t, cmd, done)
}

// Killing a process that already exited is not a failure, as on darwin: a row
// can go stale between listing and the kill.
func TestKillExitedProcessIsNoop(t *testing.T) {
	cmd, id, wait := startSleeper(t)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = wait() // reaped: /proc/<pid> is gone and pidfd_open fails with ESRCH

	if _, err := procStartToken(id.PID); err == nil {
		t.Skipf("pid %d was reused before the test could run", id.PID)
	} else if !processGone(err) {
		t.Fatalf("procStartToken of a reaped process = %v, want a not-exist or ESRCH error", err)
	}

	for name, kill := range map[string]func(Ident) error{
		"Kill":             Kill,
		"killByStartToken": killByStartToken,
	} {
		err := kill(id)
		if errors.Is(err, errIdentityMismatch) {
			t.Skipf("pid %d was reused before the test could run", id.PID)
		}
		if err != nil {
			t.Errorf("%s of exited process: %v", name, err)
		}
	}
}

// A zombie still has /proc/<pid>/stat with its original start time, and
// signals to it succeed, so Kill reaches the end of its wait and returns nil.
func TestKillZombieProcessIsNoop(t *testing.T) {
	cmd, id, _ := startSleeper(t)
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	// Block until the child is a zombie without reaping it.
	var info unix.Siginfo
	for {
		err := unix.Waitid(unix.P_PID, id.PID, &info, unix.WEXITED|unix.WNOWAIT, nil)
		if err == nil {
			break
		}
		if !errors.Is(err, unix.EINTR) {
			t.Fatalf("waitid: %v", err)
		}
	}
	if err := Kill(id); err != nil {
		t.Errorf("Kill of zombie process: %v", err)
	}
}

// A recycled PID must still be refused: "gone" is success, "someone else" is not.
func TestKillRefusesMismatchedStartToken(t *testing.T) {
	cmd, id, _ := startSleeper(t)
	stale := Ident{PID: id.PID, Start: id.Start + 1}
	for name, kill := range map[string]func(Ident) error{
		"Kill":             Kill,
		"killByStartToken": killByStartToken,
	} {
		if err := kill(stale); !errors.Is(err, errIdentityMismatch) {
			t.Errorf("%s with a stale start token = %v, want errIdentityMismatch", name, err)
		}
	}
	// Signal 0 also succeeds on a zombie, so ask waitid whether it exited.
	var info unix.Siginfo
	if err := unix.Waitid(unix.P_PID, cmd.Process.Pid, &info, unix.WEXITED|unix.WNOHANG|unix.WNOWAIT, nil); err != nil {
		t.Fatalf("waitid: %v", err)
	}
	if info.Signo != 0 {
		t.Error("process exited despite the identity mismatch")
	}
}

func TestKillRequiresIdentity(t *testing.T) {
	if err := Kill(Ident{PID: 12345, Start: 0}); !errors.Is(err, errNoIdentity) {
		t.Fatalf("Kill without a start token = %v, want errNoIdentity", err)
	}
}
