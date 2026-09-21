//go:build linux

package verification

import (
	"context"
	"os/exec"
	"syscall"
	"testing"
)

func TestCompilerProcessUsesIsolatedGroupAndParentDeathSignal(t *testing.T) {
	cmd := exec.CommandContext(context.Background(), "/bin/true")
	configureCompilerProcess(cmd)
	if cmd.SysProcAttr == nil || !cmd.SysProcAttr.Setpgid {
		t.Fatal("compiler process does not create a dedicated process group")
	}
	if cmd.SysProcAttr.Pdeathsig != syscall.SIGKILL {
		t.Fatalf("compiler parent-death signal = %v, want SIGKILL", cmd.SysProcAttr.Pdeathsig)
	}
	if cmd.Cancel == nil {
		t.Fatal("compiler process has no process-group cancellation function")
	}
	if cmd.WaitDelay != compilerWaitDelay {
		t.Fatalf("compiler WaitDelay = %s, want %s", cmd.WaitDelay, compilerWaitDelay)
	}
}
