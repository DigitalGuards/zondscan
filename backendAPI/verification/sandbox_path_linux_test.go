//go:build linux

package verification

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

type sandboxPathTestInfo struct {
	os.FileInfo
	mode os.FileMode
	size int64
	sys  any
}

func (i *sandboxPathTestInfo) Mode() os.FileMode { return i.mode }
func (i *sandboxPathTestInfo) IsDir() bool       { return i.mode.IsDir() }
func (i *sandboxPathTestInfo) Size() int64       { return i.size }
func (i *sandboxPathTestInfo) Sys() any          { return i.sys }

func rootOwnedSandboxTestInfo(info os.FileInfo) *sandboxPathTestInfo {
	stat := *info.Sys().(*syscall.Stat_t)
	stat.Uid = 0
	mode := info.Mode()
	if info.IsDir() {
		mode = os.ModeDir | 0o755
	}
	return &sandboxPathTestInfo{FileInfo: info, mode: mode, size: info.Size(), sys: &stat}
}

// Only metadata is simulated. Opens, ELF parsing, hashing, and execution use
// real files, including the production openat2 symlink restrictions.
func rootOwnedSandboxTestOps() sandboxPathOps {
	return sandboxPathOps{
		lstat: func(path string) (os.FileInfo, error) {
			info, err := os.Lstat(path)
			if err != nil {
				return nil, err
			}
			return rootOwnedSandboxTestInfo(info), nil
		},
		open: openReadOnlyNoFollow,
		stat: func(f *os.File) (os.FileInfo, error) {
			info, err := f.Stat()
			if err != nil {
				return nil, err
			}
			return rootOwnedSandboxTestInfo(info), nil
		},
	}
}

func newPathSandboxForTest(t *testing.T, path, digest string, ops sandboxPathOps) *sandboxLauncher {
	t.Helper()
	sandbox, err := newSandboxLauncherWithPathOps(path, digest, sandboxExecModePath, ops)
	if err != nil {
		t.Fatalf("new path sandbox: %v", err)
	}
	t.Cleanup(func() { _ = sandbox.Close() })
	return sandbox
}

func TestPathSandboxRejectsUnsafeFileAndEveryParent(t *testing.T) {
	path, digest := writeStaticExitLauncher(t, 0)
	compilerFile, err := snapshotExecutable(path, digest)
	if err != nil {
		t.Fatal(err)
	}
	defer compilerFile.Close()
	var paths []string
	for component := path; ; component = filepath.Dir(component) {
		paths = append(paths, component)
		if component == "/" {
			break
		}
	}
	for _, component := range paths {
		for _, tc := range []struct {
			name   string
			mutate func(*sandboxPathTestInfo)
			want   string
		}{
			{"owner", func(i *sandboxPathTestInfo) { i.sys.(*syscall.Stat_t).Uid = 1000 }, "owned by uid 0"},
			{"group writable", func(i *sandboxPathTestInfo) { i.mode |= 0o020 }, "group or world writable"},
			{"world writable", func(i *sandboxPathTestInfo) { i.mode |= 0o002 }, "group or world writable"},
			{"sticky writable", func(i *sandboxPathTestInfo) { i.mode |= os.ModeSticky | 0o022 }, "group or world writable"},
			{"symlink", func(i *sandboxPathTestInfo) { i.mode = os.ModeSymlink | 0o777 }, "must not contain symlinks"},
			{"missing ownership", func(i *sandboxPathTestInfo) { i.sys = nil }, "ownership is unavailable"},
			{"nil ownership", func(i *sandboxPathTestInfo) { i.sys = (*syscall.Stat_t)(nil) }, "ownership is unavailable"},
			{"wrong ownership type", func(i *sandboxPathTestInfo) { i.sys = struct{}{} }, "ownership is unavailable"},
			{"invalid type", func(i *sandboxPathTestInfo) {
				if component == path {
					i.mode = os.ModeDir | 0o755
				} else {
					i.mode = 0o755
				}
			}, "must be a"},
		} {
			t.Run(component+"/"+tc.name, func(t *testing.T) {
				ops := rootOwnedSandboxTestOps()
				lstat := ops.lstat
				ops.lstat = func(p string) (os.FileInfo, error) {
					info, err := lstat(p)
					if err == nil && p == component {
						tc.mutate(info.(*sandboxPathTestInfo))
					}
					return info, err
				}
				sandbox, err := newSandboxLauncherWithPathOps(path, digest, sandboxExecModePath, ops)
				if sandbox != nil {
					_ = sandbox.Close()
					t.Fatal("unsafe path produced a launcher")
				}
				if err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("startup error = %v, want %q", err, tc.want)
				}
				sandbox = newPathSandboxForTest(t, path, digest, rootOwnedSandboxTestOps())
				sandbox.pathOps = ops
				for attempt := 0; attempt < 2; attempt++ {
					stdout, stderr, err := sandbox.run(context.Background(), compilerFile, nil, nil, 1024, 1024)
					if err == nil || !strings.Contains(err.Error(), "before launch") || !strings.Contains(err.Error(), tc.want) {
						t.Fatalf("launch %d error = %v, want re-verification failure %q", attempt, err, tc.want)
					}
					if len(stdout) != 0 || len(stderr) != 0 {
						t.Fatal("rejected launcher produced output")
					}
				}
			})
		}
	}
}

func TestPathSandboxRejectsInvalidConfigurationAndArtifacts(t *testing.T) {
	path, digest := writeStaticExitLauncher(t, 0)
	for _, tc := range []struct {
		name, path, digest, mode, want string
	}{
		{"missing path", "", digest, "path", "VERIFIER_SANDBOX_BIN is required"},
		{"relative path", "nsjail", digest, "path", "absolute path"},
		{"missing digest", path, "", "path", "VERIFIER_SANDBOX_SHA256 is required"},
		{"malformed digest", path, "bad", "path", "64 hexadecimal"},
		{"wrong digest", path, strings.Repeat("0", 64), "path", "SHA-256 mismatch"},
		{"invalid mode", path, digest, "auto", "VERIFIER_SANDBOX_EXEC_MODE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sandbox, err := newSandboxLauncherWithPathOps(tc.path, tc.digest, tc.mode, rootOwnedSandboxTestOps())
			if sandbox != nil {
				_ = sandbox.Close()
				t.Fatal("invalid configuration produced a launcher")
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name   string
		mutate func(*sandboxPathTestInfo)
		want   string
	}{
		{"FIFO", func(i *sandboxPathTestInfo) { i.mode = os.ModeNamedPipe | 0o700 }, "regular file"},
		{"device", func(i *sandboxPathTestInfo) { i.mode = os.ModeDevice | 0o700 }, "regular file"},
		{"socket", func(i *sandboxPathTestInfo) { i.mode = os.ModeSocket | 0o700 }, "regular file"},
		{"non executable", func(i *sandboxPathTestInfo) { i.mode = 0o644 }, "not executable"},
		{"empty", func(i *sandboxPathTestInfo) { i.size = 0 }, "outside the allowed range"},
		{"oversized", func(i *sandboxPathTestInfo) { i.size = maxNativeExecutableBytes + 1 }, "outside the allowed range"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ops := rootOwnedSandboxTestOps()
			lstat := ops.lstat
			ops.lstat = func(p string) (os.FileInfo, error) {
				info, err := lstat(p)
				if err == nil && p == path {
					tc.mutate(info.(*sandboxPathTestInfo))
				}
				return info, err
			}
			f, err := ops.verify(path, digest)
			if f != nil {
				_ = f.Close()
				t.Fatal("invalid artifact passed verification")
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestPathSandboxUsesStaticELFChecks(t *testing.T) {
	path, _ := writeStaticExitLauncher(t, 0)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	dynamic := writeDynamicNativeCompiler(t)
	for _, tc := range []struct {
		name, path, want string
		mutate           func([]byte) []byte
	}{
		{name: "PT_INTERP", path: dynamic, want: "PT_INTERP"},
		{name: "DT_NEEDED", path: withoutPTInterp(t, dynamic), want: "DT_NEEDED"},
		{name: "non ELF", mutate: func(_ []byte) []byte { return []byte("#!/bin/sh\nexit 0\n") }, want: "ELF"},
		{name: "truncated ELF", mutate: func(b []byte) []byte { return b[:4] }, want: "ELF"},
		{name: "wrong architecture", mutate: func(b []byte) []byte { b[18] = 183; return b }, want: "Linux x86-64"},
		{name: "wrong ABI", mutate: func(b []byte) []byte { b[7] = 9; return b }, want: "Linux ELF ABI"},
		{name: "relocatable ELF", mutate: func(b []byte) []byte { b[16] = 1; return b }, want: "executable ELF"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			artifact := tc.path
			if tc.mutate != nil {
				artifact = filepath.Join(t.TempDir(), "nsjail-invalid")
				if err := os.WriteFile(artifact, tc.mutate(append([]byte(nil), original...)), 0o755); err != nil {
					t.Fatal(err)
				}
			}
			f, err := rootOwnedSandboxTestOps().verify(artifact, artifactSHA256(t, artifact))
			if f != nil {
				_ = f.Close()
				t.Fatal("invalid ELF passed verification")
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestPathSandboxRejectsSymlinksIncludingUncleanPaths(t *testing.T) {
	path, digest := writeStaticExitLauncher(t, 0)
	dir := filepath.Dir(path)
	link := filepath.Join(dir, "link")
	if err := os.Symlink(dir, link); err != nil {
		t.Fatal(err)
	}
	fileLink := filepath.Join(dir, "nsjail-link")
	if err := os.Symlink(path, fileLink); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{fileLink, link + "/" + filepath.Base(path), link + "/../" + filepath.Base(dir) + "/" + filepath.Base(path)} {
		f, err := rootOwnedSandboxTestOps().verify(p, digest)
		if f != nil {
			_ = f.Close()
			t.Fatal("symlink passed verification")
		}
		if err == nil || !strings.Contains(err.Error(), "must not contain symlinks") {
			t.Fatalf("verify(%q) = %v, want symlink rejection", p, err)
		}
	}
	// Confirm the real kernel opener also refuses a link introduced after lstat.
	ops := rootOwnedSandboxTestOps()
	lstat := ops.lstat
	ops.lstat = func(p string) (os.FileInfo, error) {
		if p == fileLink {
			return lstat(path)
		}
		return lstat(p)
	}
	if f, err := ops.verify(fileLink, digest); err == nil {
		_ = f.Close()
		t.Fatal("open followed a symlink introduced after lstat")
	}
}

func TestPathSandboxFailsClosedOnFilesystemErrorsAndChanges(t *testing.T) {
	path, digest := writeStaticExitLauncher(t, 0)
	for _, phase := range []string{"lstat", "open", "fstat", "post-hash fstat", "post-hash lstat"} {
		t.Run(phase, func(t *testing.T) {
			ops := rootOwnedSandboxTestOps()
			lstat, open, stat := ops.lstat, ops.open, ops.stat
			var opened *os.File
			lstats, stats := 0, 0
			ops.lstat = func(p string) (os.FileInfo, error) {
				if p == path {
					lstats++
					if phase == "lstat" || (phase == "post-hash lstat" && lstats == 2) {
						return nil, os.ErrPermission
					}
				}
				return lstat(p)
			}
			ops.open = func(p string) (*os.File, error) {
				if phase == "open" {
					return nil, os.ErrPermission
				}
				var err error
				opened, err = open(p)
				return opened, err
			}
			ops.stat = func(f *os.File) (os.FileInfo, error) {
				stats++
				if phase == "fstat" || (phase == "post-hash fstat" && stats == 2) {
					return nil, os.ErrPermission
				}
				return stat(f)
			}
			if f, err := ops.verify(path, digest); !errors.Is(err, os.ErrPermission) {
				if f != nil {
					_ = f.Close()
				}
				t.Fatalf("error = %v, want permission failure", err)
			}
			if opened != nil {
				if _, err := opened.Stat(); !errors.Is(err, os.ErrClosed) {
					t.Fatalf("failed verification leaked descriptor: %v", err)
				}
			}
		})
	}
	for _, phase := range []string{"open", "hash", "path"} {
		for _, change := range []string{"owner", "mode", "inode", "size", "mtime", "ctime"} {
			t.Run(phase+"/"+change, func(t *testing.T) {
				ops := rootOwnedSandboxTestOps()
				mutate := func(info os.FileInfo) {
					i := info.(*sandboxPathTestInfo)
					s := i.sys.(*syscall.Stat_t)
					switch change {
					case "owner":
						s.Uid = 1000
					case "mode":
						i.mode |= 0o002
					case "inode":
						s.Ino++
					case "size":
						i.size++
					case "mtime":
						s.Mtim.Nsec++
					case "ctime":
						s.Ctim.Nsec++
					}
				}
				lstat, stat := ops.lstat, ops.stat
				lstats, stats := 0, 0
				ops.lstat = func(p string) (os.FileInfo, error) {
					info, err := lstat(p)
					if err == nil && p == path {
						lstats++
						if phase == "path" && lstats == 2 {
							mutate(info)
						}
					}
					return info, err
				}
				ops.stat = func(f *os.File) (os.FileInfo, error) {
					info, err := stat(f)
					stats++
					if err == nil && ((phase == "open" && stats == 1) || (phase == "hash" && stats == 2)) {
						mutate(info)
					}
					return info, err
				}
				if f, err := ops.verify(path, digest); err == nil {
					_ = f.Close()
					t.Fatal("file changed during verification without failure")
				}
			})
		}
	}
}

func writePathCheckingSandboxLauncher(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "launcher.c")
	path := filepath.Join(dir, "nsjail-test-launcher")
	source := `#define _GNU_SOURCE
#include <fcntl.h>
#include <string.h>
#include <sys/syscall.h>
#include <unistd.h>
extern char **environ;
int main(int argc, char **argv) {
  char executable[4096];
  ssize_t n = readlink("/proc/self/exe", executable, sizeof(executable) - 1);
  if (n < 0) return 60;
  executable[n] = '\0';
  if (strcmp(executable, argv[0]) != 0) return 61;
  /* One compiler argument for --version, two for a standard-JSON compile. */
  if ((argc != 6 && argc != 7) || strcmp(argv[1], "--config") != 0 ||
      strcmp(argv[2], "/proc/self/fd/5") != 0 || strcmp(argv[3], "--") != 0 ||
      strcmp(argv[4], "/proc/self/fd/4") != 0) return 62;
  if (environ != NULL && environ[0] != NULL) return 63;
  int seals = F_SEAL_WRITE | F_SEAL_GROW | F_SEAL_SHRINK | F_SEAL_SEAL | 0x20;
  for (int fd = 4; fd <= 5; fd++) {
    int actual = fcntl(fd, F_GET_SEALS);
    if (actual < 0 || (actual & seals) != seals) return 64;
  }
  syscall(SYS_execveat, 4, "", &argv[4], environ, AT_EMPTY_PATH);
  return 65;
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("cc", "-static", "-O2", "-s", "-o", path, sourcePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build path-checking launcher: %v: %s", err, output)
	}
	return path, artifactSHA256(t, path)
}

func TestPathSandboxExecutesPathWithSealedInputsAndReverifiesEveryLaunch(t *testing.T) {
	path, digest := writePathCheckingSandboxLauncher(t)
	ops := rootOwnedSandboxTestOps()
	open := ops.open
	opens := 0
	var opened []*os.File
	ops.open = func(p string) (*os.File, error) {
		opens++
		f, err := open(p)
		if err == nil {
			opened = append(opened, f)
		}
		return f, err
	}
	sandbox := newPathSandboxForTest(t, path, digest, ops)
	if sandbox.execFile != nil || sandbox.execPath != path || opens != 1 {
		t.Fatal("path launcher did not verify once at initialization")
	}
	compilerPath, compilerDigest := writeFakeNativeCompiler(t, "path-sandbox")
	compiler, err := newCompiler(CompilerSpec{
		Kind: CompilerKindNative, BuildID: "path-sandbox", Bin: compilerPath, SHA256: compilerDigest, Default: true,
	}, sandbox, make(chan struct{}, 1), 2*time.Second, 64*1024, defaultMaxStdoutBytes, defaultMaxStderrBytes)
	if err != nil {
		t.Fatalf("probe compiler through path launcher: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })
	if opens != 2 {
		t.Fatalf("opens after version probe = %d, want 2", opens)
	}
	if err := os.WriteFile(compilerPath, []byte("replaced after snapshot"), 0o700); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if _, err := compiler.Compile(context.Background(), StandardJSONInput{Language: "Hyperion"}); err != nil {
			t.Fatalf("compile %d: %v", i, err)
		}
	}
	if opens != 4 {
		t.Fatalf("opens after compiles = %d, want 4", opens)
	}
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, replacement := range []bool{false, true} {
		t.Run(fmt.Sprintf("replace-inode=%t", replacement), func(t *testing.T) {
			target := path
			if replacement {
				target += ".replacement"
			}
			if err := os.WriteFile(target, append(append([]byte(nil), original...), 'x'), 0o755); err != nil {
				t.Fatal(err)
			}
			if replacement {
				if err := os.Rename(target, path); err != nil {
					t.Fatal(err)
				}
			}
			for attempt := 0; attempt < 2; attempt++ {
				if _, err := compiler.Compile(context.Background(), StandardJSONInput{Language: "Hyperion"}); err == nil ||
					!strings.Contains(err.Error(), "before launch") || !strings.Contains(err.Error(), "SHA-256 mismatch") {
					t.Fatalf("changed launcher error = %v, want pre-launch digest failure", err)
				}
			}
			if err := os.WriteFile(path, original, 0o755); err != nil {
				t.Fatal(err)
			}
			if _, err := compiler.Compile(context.Background(), StandardJSONInput{Language: "Hyperion"}); err != nil {
				t.Fatalf("restored pinned launcher: %v", err)
			}
		})
	}
	policyFD := sandbox.policyFile.Fd()
	if err := sandbox.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := unix.FcntlInt(policyFD, unix.F_GETFD, 0); !errors.Is(err, unix.EBADF) {
		t.Fatalf("policy descriptor leaked after Close: %v", err)
	}
	if _, err := compiler.Compile(context.Background(), StandardJSONInput{Language: "Hyperion"}); err == nil ||
		!strings.Contains(err.Error(), "sandbox launcher is closed") {
		t.Fatalf("compile after close = %v, want closed error", err)
	}
	if err := sandbox.Close(); err != nil {
		t.Fatalf("idempotent close: %v", err)
	}
	for i, f := range opened {
		if _, err := f.Stat(); !errors.Is(err, os.ErrClosed) {
			t.Errorf("verified launcher descriptor %d remains open: %v", i, err)
		}
	}
}
