//go:build !linux

package verification

import (
	"os/exec"
	"time"
)

const compilerWaitDelay = 2 * time.Second

func configureCompilerProcess(cmd *exec.Cmd) {
	cmd.WaitDelay = compilerWaitDelay
}
