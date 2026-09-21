//go:build linux

package verification

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestSecureArtifactOpenPromptlyRejectsFIFOAndDevice(t *testing.T) {
	fifoPath := filepath.Join(t.TempDir(), "artifact.fifo")
	if err := unix.Mkfifo(fifoPath, 0o600); err != nil {
		t.Fatalf("create FIFO: %v", err)
	}

	cases := []struct {
		name string
		open func() error
	}{
		{
			name: "manifest FIFO",
			open: func() error {
				_, err := readCompilerManifest(fifoPath, strings.Repeat("0", 64))
				return err
			},
		},
		{
			name: "compiler FIFO",
			open: func() error {
				f, err := snapshotExecutable(fifoPath, strings.Repeat("0", 64))
				if f != nil {
					_ = f.Close()
				}
				return err
			},
		},
		{
			name: "sandbox FIFO",
			open: func() error {
				f, err := snapshotSandboxLauncher(fifoPath, strings.Repeat("0", 64))
				if f != nil {
					_ = f.Close()
				}
				return err
			},
		},
		{
			name: "manifest device",
			open: func() error {
				_, err := readCompilerManifest("/dev/null", strings.Repeat("0", 64))
				return err
			},
		},
		{
			name: "compiler device",
			open: func() error {
				f, err := snapshotExecutable("/dev/null", strings.Repeat("0", 64))
				if f != nil {
					_ = f.Close()
				}
				return err
			},
		},
		{
			name: "sandbox device",
			open: func() error {
				f, err := snapshotSandboxLauncher("/dev/null", strings.Repeat("0", 64))
				if f != nil {
					_ = f.Close()
				}
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			done := make(chan error, 1)
			go func() { done <- tc.open() }()

			select {
			case err := <-done:
				if err == nil || !strings.Contains(err.Error(), "regular file") {
					t.Fatalf("openSecureRegularFile error = %v, want regular-file rejection", err)
				}
			case <-time.After(500 * time.Millisecond):
				t.Fatal("secure artifact opener blocked on a FIFO or device")
			}
		})
	}
}
