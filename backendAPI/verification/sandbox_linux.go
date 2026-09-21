//go:build linux

package verification

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"golang.org/x/sys/unix"
)

const (
	sandboxSnapshotExecArg = "/proc/self/fd/3"
	compilerSnapshotArg    = "/proc/self/fd/4"
	sandboxPolicyArg       = "/proc/self/fd/5"
)

type sandboxLauncher struct {
	SHA256       string
	PolicySHA256 string

	mu         sync.RWMutex
	execFile   *os.File
	policyFile *os.File
}

func newSandboxLauncher(path, expectedSHA256 string) (*sandboxLauncher, error) {
	if path == "" {
		return nil, errors.New("VERIFIER_SANDBOX_BIN is required")
	}
	if !filepath.IsAbs(path) {
		return nil, errors.New("VERIFIER_SANDBOX_BIN must be an absolute path")
	}
	digest, err := normalizeSHA256(expectedSHA256)
	if err != nil {
		return nil, fmt.Errorf("VERIFIER_SANDBOX_SHA256: %w", err)
	}
	if digest == "" {
		return nil, errors.New("VERIFIER_SANDBOX_SHA256 is required")
	}
	policyDigest, err := sandboxPolicySHA256()
	if err != nil {
		return nil, err
	}

	execFile, err := snapshotSandboxLauncher(path, digest)
	if err != nil {
		return nil, err
	}
	keepExec := false
	defer func() {
		if !keepExec {
			_ = execFile.Close()
		}
	}()

	policyFile, err := sealSandboxPolicy()
	if err != nil {
		return nil, err
	}
	keepPolicy := false
	defer func() {
		if !keepPolicy {
			_ = policyFile.Close()
		}
	}()

	launcher := &sandboxLauncher{
		SHA256:       digest,
		PolicySHA256: policyDigest,
		execFile:     execFile,
		policyFile:   policyFile,
	}

	keepExec = true
	keepPolicy = true
	return launcher, nil
}

func sealSandboxPolicy() (*os.File, error) {
	return sealSandboxPolicyBytes(sandboxPolicy)
}

func sealSandboxPolicyBytes(policy []byte) (*os.File, error) {
	flags := unix.MFD_CLOEXEC | unix.MFD_ALLOW_SEALING
	fd, err := unix.MemfdCreate("zondscan-nsjail-policy", flags)
	if err != nil {
		return nil, fmt.Errorf("create sandbox policy memfd: %w", err)
	}
	policyFile := os.NewFile(uintptr(fd), "zondscan-nsjail-policy")
	if policyFile == nil {
		_ = unix.Close(fd)
		return nil, errors.New("wrap sandbox policy memfd")
	}
	keepOpen := false
	defer func() {
		if !keepOpen {
			_ = policyFile.Close()
		}
	}()

	written, err := policyFile.Write(policy)
	if err != nil {
		return nil, fmt.Errorf("write sandbox policy snapshot: %w", err)
	}
	if written != len(policy) {
		return nil, fmt.Errorf("short sandbox policy snapshot write: %d of %d", written, len(policy))
	}
	if err := unix.Fchmod(fd, 0o400); err != nil {
		return nil, fmt.Errorf("make sandbox policy read-only: %w", err)
	}
	if _, err := policyFile.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("rewind sandbox policy: %w", err)
	}
	seals := unix.F_SEAL_WRITE | unix.F_SEAL_GROW | unix.F_SEAL_SHRINK | unix.F_SEAL_EXEC | unix.F_SEAL_SEAL
	if _, err := unix.FcntlInt(uintptr(fd), unix.F_ADD_SEALS, seals); err != nil {
		return nil, fmt.Errorf("seal sandbox policy: %w", err)
	}
	gotSeals, err := unix.FcntlInt(uintptr(fd), unix.F_GET_SEALS, 0)
	if err != nil {
		return nil, fmt.Errorf("read sandbox policy seals: %w", err)
	}
	if gotSeals&seals != seals {
		return nil, fmt.Errorf("sandbox policy seal mismatch: want %#x, got %#x", seals, gotSeals)
	}

	keepOpen = true
	return policyFile, nil
}

func (s *sandboxLauncher) run(
	ctx context.Context,
	compilerFile *os.File,
	args []string,
	stdin *bytes.Reader,
	maxStdout int,
	maxStderr int,
) ([]byte, []byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.execFile == nil || s.policyFile == nil {
		return nil, nil, errors.New("sandbox launcher is closed")
	}
	if compilerFile == nil {
		return nil, nil, errors.New("compiler snapshot is closed")
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	commandArgs := []string{
		"--config", sandboxPolicyArg,
		"--",
		compilerSnapshotArg,
	}
	commandArgs = append(commandArgs, args...)
	cmd := exec.CommandContext(runCtx, sandboxSnapshotExecArg, commandArgs...)
	configureCompilerProcess(cmd)
	cmd.ExtraFiles = []*os.File{s.execFile, compilerFile, s.policyFile}
	cmd.Env = []string{}
	if stdin != nil {
		cmd.Stdin = stdin
	}
	stdout := newBoundedBuffer(maxStdout, cancel)
	stderr := newBoundedBuffer(maxStderr, cancel)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	if stdout.Overflowed() {
		return stdout.Bytes(), stderr.Bytes(), ErrCompilerOutputTooLarge
	}
	if stderr.Overflowed() {
		return stdout.Bytes(), stderr.Bytes(), ErrCompilerStderrTooLarge
	}
	return stdout.Bytes(), stderr.Bytes(), err
}

func (s *sandboxLauncher) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	var firstErr error
	if s.execFile != nil {
		if err := s.execFile.Close(); err != nil {
			firstErr = err
		}
		s.execFile = nil
	}
	if s.policyFile != nil {
		if err := s.policyFile.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		s.policyFile = nil
	}
	return firstErr
}
