//go:build linux

package verification

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"backendAPI/models"

	"golang.org/x/sys/unix"
)

const (
	currentQ128BuildID = "0.2.0-develop.2026.8.27+commit.6f862206.mod.Linux.g++"
	currentQ128SHA256  = "ac24ccbb53fa6200dc6ca9a3bb87aac5dbb8dd7a23a3f91c3a7563f6275882dd"
	currentQ128Dynamic = "fe8e2344dbd902d6fc8c8cbb24114378c2de3a996b0d58f642d303c8bf30e930"
	historicalD5Build  = "0.2.0-develop.2026.4.13+commit.d5d1b977.Linux.g++"
	historicalD5SHA256 = "7e6e9b1703321e19a51e5076dce85388845549d8415c94ce8e47ea1a45aad9b0"
)

func nativeRunner(t *testing.T) string {
	t.Helper()
	runner, err := filepath.Abs(filepath.Join("runner", "hypc-native.sh"))
	if err != nil {
		t.Fatalf("absolute native runner path: %v", err)
	}
	return runner
}

func writeFakeNativeCompiler(t *testing.T, buildID string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "hypc")
	sourcePath := filepath.Join(dir, "main.go")
	source := "package main\n" +
		"import (\"fmt\"; \"io\"; \"os\")\n" +
		"func main() {\n" +
		" if len(os.Args) == 2 && os.Args[1] == \"--version\" {\n" +
		"  fmt.Println(\"hypc, the hyperion compiler commandline interface\")\n" +
		"  fmt.Println(\"Version: \" + " + strconv.Quote(buildID) + ")\n" +
		"  return\n" +
		" }\n" +
		" if len(os.Args) == 2 && os.Args[1] == \"--standard-json\" {\n" +
		"  _, _ = io.Copy(io.Discard, os.Stdin)\n" +
		"  fmt.Print(`{\"contracts\":{}}`)\n" +
		"  return\n" +
		" }\n" +
		" os.Exit(2)\n" +
		"}\n"
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatalf("write fake compiler source: %v", err)
	}
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-buildid=", "-o", path, sourcePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build fake compiler: %v: %s", err, output)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fake compiler: %v", err)
	}
	digest := sha256.Sum256(data)
	return path, hex.EncodeToString(digest[:])
}

func writeBlockingFakeNativeCompiler(t *testing.T, buildID, markerPath string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "hypc")
	sourcePath := filepath.Join(dir, "main.go")
	source := "package main\n" +
		"import (\"fmt\"; \"io\"; \"os\"; \"time\")\n" +
		"func main() {\n" +
		" if len(os.Args) == 2 && os.Args[1] == \"--version\" {\n" +
		"  fmt.Println(\"Version: \" + " + strconv.Quote(buildID) + ")\n" +
		"  return\n" +
		" }\n" +
		" if len(os.Args) == 2 && os.Args[1] == \"--standard-json\" {\n" +
		"  _ = os.WriteFile(" + strconv.Quote(markerPath) + ", []byte(\"started\"), 0600)\n" +
		"  time.Sleep(250 * time.Millisecond)\n" +
		"  _, _ = io.Copy(io.Discard, os.Stdin)\n" +
		"  fmt.Print(`{\"contracts\":{}}`)\n" +
		"  return\n" +
		" }\n" +
		" os.Exit(2)\n" +
		"}\n"
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatalf("write blocking compiler source: %v", err)
	}
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-buildid=", "-o", path, sourcePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build blocking compiler: %v: %s", err, output)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read blocking compiler: %v", err)
	}
	digest := sha256.Sum256(data)
	return path, hex.EncodeToString(digest[:])
}

func writeResourceFakeNativeCompiler(t *testing.T, buildID, behavior, markerPath string) (string, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "hypc")
	sourcePath := filepath.Join(dir, "main.go")
	source := "package main\n" +
		"import (\"fmt\"; \"io\"; \"os\"; \"os/exec\"; \"strconv\"; \"strings\"; \"time\")\n" +
		"func main() {\n" +
		" if len(os.Args) >= 2 && os.Args[1] == \"--child\" {\n" +
		"  _ = os.WriteFile(os.Args[2], []byte(strconv.Itoa(os.Getpid())), 0600)\n" +
		"  time.Sleep(30 * time.Second)\n" +
		"  return\n" +
		" }\n" +
		" if len(os.Args) == 2 && os.Args[1] == \"--version\" {\n" +
		"  fmt.Println(\"Version: \" + " + strconv.Quote(buildID) + ")\n" +
		"  return\n" +
		" }\n" +
		" if len(os.Args) == 2 && os.Args[1] == \"--standard-json\" {\n" +
		"  _, _ = io.Copy(io.Discard, os.Stdin)\n" +
		"  switch " + strconv.Quote(behavior) + " {\n" +
		"  case \"stdout\": fmt.Print(strings.Repeat(\"x\", 1 << 20))\n" +
		"  case \"stderr\": fmt.Fprint(os.Stderr, strings.Repeat(\"e\", 1 << 20))\n" +
		"  case \"descendant\":\n" +
		"   child := exec.Command(os.Args[0], \"--child\", " + strconv.Quote(markerPath) + ")\n" +
		"   _ = child.Start()\n" +
		"   time.Sleep(30 * time.Second)\n" +
		"  default: fmt.Print(`{\"contracts\":{}}`)\n" +
		"  }\n" +
		"  return\n" +
		" }\n" +
		" os.Exit(2)\n" +
		"}\n"
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatalf("write resource compiler source: %v", err)
	}
	cmd := exec.Command("go", "build", "-trimpath", "-ldflags=-buildid=", "-o", path, sourcePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build resource compiler: %v: %s", err, output)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read resource compiler: %v", err)
	}
	digest := sha256.Sum256(data)
	return path, hex.EncodeToString(digest[:])
}

func writeDynamicNativeCompiler(t *testing.T) string {
	t.Helper()
	cc, err := exec.LookPath("cc")
	if err != nil {
		t.Skipf("C compiler unavailable for dynamic ELF fixture: %v", err)
	}
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "main.c")
	path := filepath.Join(dir, "hypc-dynamic")
	source := "#include <stdio.h>\nint main(void) { puts(\"dynamic\"); return 0; }\n"
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatalf("write dynamic compiler source: %v", err)
	}
	cmd := exec.Command(cc, "-o", path, sourcePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("build dynamic ELF fixture: %v: %s", err, output)
	}
	return path
}

func writeFakeSandboxLauncher(t *testing.T) (string, string) {
	t.Helper()
	cc, err := exec.LookPath("cc")
	if err != nil {
		t.Fatalf("C compiler unavailable for static sandbox test launcher: %v", err)
	}
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "launcher.c")
	path := filepath.Join(dir, "nsjail-test-launcher")
	source := `#define _GNU_SOURCE
#include <errno.h>
#include <fcntl.h>
#include <stdio.h>
#include <sys/syscall.h>
#include <unistd.h>
extern char **environ;
int main(int argc, char **argv) {
  int separator = -1;
  for (int i = 1; i < argc; i++) {
    if (argv[i][0] == '-' && argv[i][1] == '-' && argv[i][2] == '\0') {
      separator = i;
      break;
    }
  }
  if (separator < 0 || separator + 1 >= argc) return 64;
  syscall(SYS_execveat, 4, "", &argv[separator + 1], environ, AT_EMPTY_PATH);
  perror("execveat compiler test snapshot");
  return errno == 0 ? 65 : errno;
}
`
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatalf("write static sandbox test launcher source: %v", err)
	}
	cmd := exec.Command(cc, "-static", "-O2", "-s", "-o", path, sourcePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build static sandbox test launcher: %v: %s", err, output)
	}
	return path, artifactSHA256(t, path)
}

func fakeSandboxForTest(t *testing.T) *sandboxLauncher {
	t.Helper()
	path, digest := writeFakeSandboxLauncher(t)
	sandbox, err := newSandboxLauncher(path, digest)
	if err != nil {
		t.Fatalf("new fake sandbox launcher: %v", err)
	}
	t.Cleanup(func() {
		if err := sandbox.Close(); err != nil {
			t.Errorf("close fake sandbox launcher: %v", err)
		}
	})
	return sandbox
}

func setFakeSandboxEnv(t *testing.T) {
	t.Helper()
	path, digest := writeFakeSandboxLauncher(t)
	t.Setenv("VERIFIER_SANDBOX_BIN", path)
	t.Setenv("VERIFIER_SANDBOX_SHA256", digest)
}

func withoutPTInterp(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read dynamic ELF fixture: %v", err)
	}
	if len(data) < 64 || data[4] != byte(2) || data[5] != byte(1) {
		t.Fatal("dynamic ELF fixture is not little-endian ELF64")
	}
	programOffset := binary.LittleEndian.Uint64(data[32:40])
	programSize := uint64(binary.LittleEndian.Uint16(data[54:56]))
	programCount := uint64(binary.LittleEndian.Uint16(data[56:58]))
	found := false
	for i := uint64(0); i < programCount; i++ {
		offset := programOffset + i*programSize
		if offset+4 > uint64(len(data)) {
			t.Fatal("dynamic ELF program header exceeds fixture size")
		}
		if binary.LittleEndian.Uint32(data[offset:offset+4]) == uint32(3) {
			binary.LittleEndian.PutUint32(data[offset:offset+4], uint32(0))
			found = true
		}
	}
	if !found {
		t.Fatal("dynamic ELF fixture has no PT_INTERP program header")
	}
	outPath := filepath.Join(t.TempDir(), "hypc-needed-only")
	if err := os.WriteFile(outPath, data, 0o700); err != nil {
		t.Fatalf("write DT_NEEDED-only fixture: %v", err)
	}
	return outPath
}

func artifactSHA256(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read artifact: %v", err)
	}
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func compilerForTest(t *testing.T, spec CompilerSpec) *Compiler {
	return compilerForTestWithLimits(
		t,
		spec,
		5*time.Second,
		defaultMaxStdoutBytes,
		defaultMaxStderrBytes,
	)
}

func compilerForTestWithLimits(
	t *testing.T,
	spec CompilerSpec,
	timeout time.Duration,
	maxStdout int,
	maxStderr int,
) *Compiler {
	t.Helper()
	c, err := newCompiler(
		spec,
		fakeSandboxForTest(t),
		make(chan struct{}, 1),
		timeout,
		64*1024,
		maxStdout,
		maxStderr,
	)
	if err != nil {
		t.Fatalf("newCompiler: %v", err)
	}
	t.Cleanup(func() {
		if err := c.Close(); err != nil {
			t.Errorf("close compiler snapshot: %v", err)
		}
	})
	return c
}

func TestCompilerOutputCapsCancelExecution(t *testing.T) {
	cases := []struct {
		name     string
		behavior string
		want     error
	}{
		{name: "stdout", behavior: "stdout", want: ErrCompilerOutputTooLarge},
		{name: "stderr", behavior: "stderr", want: ErrCompilerStderrTooLarge},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bin, digest := writeResourceFakeNativeCompiler(t, "output-cap", tc.behavior, "")
			c := compilerForTestWithLimits(t, CompilerSpec{
				Kind:    CompilerKindNative,
				BuildID: "output-cap",
				Bin:     bin,
				SHA256:  digest,
				Default: true,
			}, 5*time.Second, 1024, 512)

			_, err := c.Compile(context.Background(), StandardJSONInput{Language: "Hyperion"})
			if !errors.Is(err, tc.want) {
				t.Fatalf("Compile error = %v, want %v", err, tc.want)
			}
		})
	}
}

func TestCompilerTimeoutKillsDescendantProcessGroup(t *testing.T) {
	markerPath := filepath.Join(t.TempDir(), "child-pid")
	bin, digest := writeResourceFakeNativeCompiler(t, "descendant", "descendant", markerPath)
	c := compilerForTestWithLimits(t, CompilerSpec{
		Kind:    CompilerKindNative,
		BuildID: "descendant",
		Bin:     bin,
		SHA256:  digest,
		Default: true,
	}, 500*time.Millisecond, 1024, 1024)

	_, err := c.Compile(context.Background(), StandardJSONInput{Language: "Hyperion"})
	if !errors.Is(err, ErrCompileTimeout) {
		t.Fatalf("Compile error = %v, want ErrCompileTimeout", err)
	}
	pidBytes, err := os.ReadFile(markerPath)
	if err != nil {
		t.Fatalf("read descendant PID marker: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
	if err != nil {
		t.Fatalf("parse descendant PID %q: %v", pidBytes, err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		err := unix.Kill(pid, 0)
		if errors.Is(err, unix.ESRCH) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("descendant process %d survived compiler process-group cancellation: %v", pid, err)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestNativeCompilerExecutesSealedSnapshot(t *testing.T) {
	bin, digest := writeFakeNativeCompiler(t, "test-q128")
	c := compilerForTest(t, CompilerSpec{
		Kind:    CompilerKindNative,
		BuildID: "test-q128",
		Bin:     bin,
		SHA256:  strings.ToUpper(digest),
		Default: true,
	})
	if c.SHA256 != digest {
		t.Fatalf("normalized digest = %q, want %q", c.SHA256, digest)
	}
	if c.Provenance.ExecutionDigest == "" || c.Provenance.Components[0].SHA256 != digest {
		t.Fatalf("compiler provenance = %+v", c.Provenance)
	}
	if _, err := c.execFile.WriteAt([]byte{0}, 0); err == nil {
		t.Fatal("sealed compiler snapshot unexpectedly accepted a write")
	}
	info, err := c.execFile.Stat()
	if err != nil {
		t.Fatalf("stat executable snapshot: %v", err)
	}
	if info.Mode().Perm() != 0o500 {
		t.Fatalf("executable snapshot mode = %#o, want 0500", info.Mode().Perm())
	}
	wantSeals := unix.F_SEAL_WRITE | unix.F_SEAL_GROW | unix.F_SEAL_SHRINK | unix.F_SEAL_EXEC | unix.F_SEAL_SEAL
	gotSeals, err := unix.FcntlInt(c.execFile.Fd(), unix.F_GET_SEALS, 0)
	if err != nil {
		t.Fatalf("read executable snapshot seals: %v", err)
	}
	if gotSeals&wantSeals != wantSeals {
		t.Fatalf("executable snapshot seals = %#x, want %#x", gotSeals, wantSeals)
	}

	if _, err := c.Compile(context.Background(), StandardJSONInput{Language: "Hyperion"}); err != nil {
		t.Fatalf("compile with pinned fake compiler: %v", err)
	}

	replacement, _ := writeFakeNativeCompiler(t, "replacement")
	if err := os.Rename(replacement, bin); err != nil {
		t.Fatalf("atomically replace source compiler: %v", err)
	}
	if got, err := c.probeVersion(); err != nil || got != "test-q128" {
		t.Fatalf("version after source replacement = %q, error = %v", got, err)
	}
	if _, err := c.Compile(context.Background(), StandardJSONInput{Language: "Hyperion"}); err != nil {
		t.Fatalf("compile after source replacement: %v", err)
	}
}

func TestNativeCompilerRejectsDynamicELFDependencies(t *testing.T) {
	dynamicPath := writeDynamicNativeCompiler(t)
	t.Run("PT_INTERP", func(t *testing.T) {
		_, err := snapshotExecutable(dynamicPath, artifactSHA256(t, dynamicPath))
		if err == nil || !strings.Contains(err.Error(), "PT_INTERP") {
			t.Fatalf("snapshotExecutable error = %v, want PT_INTERP rejection", err)
		}
	})

	t.Run("DT_NEEDED", func(t *testing.T) {
		neededPath := withoutPTInterp(t, dynamicPath)
		_, err := snapshotExecutable(neededPath, artifactSHA256(t, neededPath))
		if err == nil || !strings.Contains(err.Error(), "DT_NEEDED") {
			t.Fatalf("snapshotExecutable error = %v, want DT_NEEDED rejection", err)
		}
	})
}

func TestNativeCompilerRejectsNonLinuxOrNonExecutableELF(t *testing.T) {
	path, _ := writeFakeNativeCompiler(t, "elf-identity")
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read static ELF fixture: %v", err)
	}
	if len(original) < 18 || original[4] != byte(2) || original[5] != byte(1) {
		t.Fatal("static ELF fixture is not little-endian ELF64")
	}

	freeBSDPath := filepath.Join(t.TempDir(), "hypc-freebsd-abi")
	freeBSD := append([]byte(nil), original...)
	freeBSD[7] = byte(9)
	if err := os.WriteFile(freeBSDPath, freeBSD, 0o700); err != nil {
		t.Fatalf("write non-Linux ABI fixture: %v", err)
	}
	if snapshot, err := snapshotExecutable(freeBSDPath, artifactSHA256(t, freeBSDPath)); err == nil ||
		!strings.Contains(err.Error(), "Linux ELF ABI") {
		if snapshot != nil {
			_ = snapshot.Close()
		}
		t.Fatalf("non-Linux ELF error = %v, want Linux ABI rejection", err)
	}

	relocatablePath := filepath.Join(t.TempDir(), "hypc-relocatable")
	relocatable := append([]byte(nil), original...)
	relocatable[16] = byte(1)
	relocatable[17] = byte(0)
	if err := os.WriteFile(relocatablePath, relocatable, 0o700); err != nil {
		t.Fatalf("write relocatable ELF fixture: %v", err)
	}
	if snapshot, err := snapshotExecutable(relocatablePath, artifactSHA256(t, relocatablePath)); err == nil ||
		!strings.Contains(err.Error(), "executable ELF") {
		if snapshot != nil {
			_ = snapshot.Close()
		}
		t.Fatalf("relocatable ELF error = %v, want executable-type rejection", err)
	}
}

func TestCompilerCloseWaitsForCompileAndReleasesSnapshot(t *testing.T) {
	markerPath := filepath.Join(t.TempDir(), "compile-started")
	bin, digest := writeBlockingFakeNativeCompiler(t, "blocking-q128", markerPath)
	c := compilerForTest(t, CompilerSpec{
		Kind:    CompilerKindNative,
		BuildID: "blocking-q128",
		Bin:     bin,
		SHA256:  digest,
		Default: true,
	})
	fd := c.execFile.Fd()
	compileDone := make(chan error, 1)
	go func() {
		_, err := c.Compile(context.Background(), StandardJSONInput{Language: "Hyperion"})
		compileDone <- err
	}()

	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(markerPath); err == nil {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("compiler subprocess did not reach its blocking section")
		}
		time.Sleep(5 * time.Millisecond)
	}

	closeDone := make(chan error, 1)
	go func() { closeDone <- c.Close() }()
	select {
	case err := <-closeDone:
		t.Fatalf("Close returned before the in-flight compile finished: %v", err)
	case <-time.After(25 * time.Millisecond):
	}
	if err := <-compileDone; err != nil {
		t.Fatalf("blocking compile: %v", err)
	}
	if err := <-closeDone; err != nil {
		t.Fatalf("close compiler: %v", err)
	}
	if _, err := unix.FcntlInt(fd, unix.F_GETFD, 0); !errors.Is(err, unix.EBADF) {
		t.Fatalf("snapshot fd remains open after Close: %v", err)
	}
	if _, err := c.probeVersion(); err == nil || !strings.Contains(err.Error(), "snapshot is closed") {
		t.Fatalf("probe after Close error = %v", err)
	}
	if err := c.Close(); err != nil {
		t.Fatalf("idempotent Close: %v", err)
	}
}

func TestNativeCompilerRejectsUnpinnedOrMismatchedBinary(t *testing.T) {
	sandbox := fakeSandboxForTest(t)
	bin, digest := writeFakeNativeCompiler(t, "test-native")
	writableBin, writableDigest := writeFakeNativeCompiler(t, "writable-native")
	if err := os.Chmod(writableBin, 0o775); err != nil {
		t.Fatalf("make compiler group writable: %v", err)
	}
	nonELF := filepath.Join(t.TempDir(), "hypc-script")
	if err := os.WriteFile(nonELF, []byte("#!/bin/sh\nexit 0\n"), 0o700); err != nil {
		t.Fatalf("write non-ELF compiler: %v", err)
	}
	nonELFBytes, err := os.ReadFile(nonELF)
	if err != nil {
		t.Fatalf("read non-ELF compiler: %v", err)
	}
	nonELFDigest := sha256.Sum256(nonELFBytes)
	symlinkPath := filepath.Join(t.TempDir(), "hypc-link")
	if err := os.Symlink(bin, symlinkPath); err != nil {
		t.Fatalf("create compiler symlink: %v", err)
	}
	cases := []struct {
		name string
		spec CompilerSpec
		want string
	}{
		{
			name: "missing SHA-256",
			spec: CompilerSpec{Kind: CompilerKindNative, BuildID: "test-native", Bin: bin},
			want: "sha256 is required",
		},
		{
			name: "wrong SHA-256",
			spec: CompilerSpec{Kind: CompilerKindNative, BuildID: "test-native", Bin: bin,
				SHA256: strings.Repeat("0", 64)},
			want: "SHA-256 mismatch",
		},
		{
			name: "invalid SHA-256",
			spec: CompilerSpec{Kind: CompilerKindNative, BuildID: "test-native", Bin: bin,
				SHA256: "not-a-digest"},
			want: "64 hexadecimal characters",
		},
		{
			name: "relative compiler",
			spec: CompilerSpec{Kind: CompilerKindNative, BuildID: "test-native", Bin: "hypc",
				SHA256: digest},
			want: "absolute path",
		},
		{
			name: "missing compiler",
			spec: CompilerSpec{Kind: CompilerKindNative, BuildID: "test-native", SHA256: digest},
			want: "bin is required",
		},
		{
			name: "legacy runner fields",
			spec: CompilerSpec{Kind: CompilerKindNative, BuildID: "test-native", NodeBin: "/bin/sh", Runner: "/runner",
				Bin: bin, SHA256: digest},
			want: "must not configure runner or nodeBin",
		},
		{
			name: "compiler symlink",
			spec: CompilerSpec{Kind: CompilerKindNative, BuildID: "test-native", Bin: symlinkPath, SHA256: digest},
			want: "must not contain symlinks",
		},
		{
			name: "group writable compiler",
			spec: CompilerSpec{Kind: CompilerKindNative, BuildID: "writable-native", Bin: writableBin,
				SHA256: writableDigest},
			want: "must not be group or world writable",
		},
		{
			name: "non-regular compiler",
			spec: CompilerSpec{Kind: CompilerKindNative, BuildID: "test-native", Bin: t.TempDir(), SHA256: digest},
			want: "must be a regular file",
		},
		{
			name: "non-ELF compiler",
			spec: CompilerSpec{Kind: CompilerKindNative, BuildID: "test-native", Bin: nonELF,
				SHA256: hex.EncodeToString(nonELFDigest[:])},
			want: "must be an ELF executable",
		},
		{
			name: "unpinned npm runtime",
			spec: CompilerSpec{Kind: CompilerKindNPM, BuildID: localBuildID, Runner: nativeRunner(t)},
			want: "unavailable without complete runtime provenance",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := newCompiler(
				tc.spec,
				sandbox,
				make(chan struct{}, 1),
				time.Second,
				1024,
				defaultMaxStdoutBytes,
				defaultMaxStderrBytes,
			)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("newCompiler error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestNativeRunnerRejectsPATHFallbackAndBareBin(t *testing.T) {
	bin, _ := writeFakeNativeCompiler(t, "path-fallback")
	pathDir := filepath.Dir(bin)
	runner := nativeRunner(t)

	t.Run("missing HYPC_BIN", func(t *testing.T) {
		cmd := exec.Command("/bin/sh", runner, "--version")
		cmd.Env = []string{"PATH=" + pathDir}
		output, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(output), "HYPC_BIN is required") {
			t.Fatalf("runner output = %q, error = %v", output, err)
		}
	})

	t.Run("bare HYPC_BIN", func(t *testing.T) {
		cmd := exec.Command("/bin/sh", runner, "--version")
		cmd.Env = []string{"PATH=" + pathDir, "HYPC_BIN=hypc"}
		output, err := cmd.CombinedOutput()
		if err == nil || !strings.Contains(string(output), "must be an absolute path") {
			t.Fatalf("runner output = %q, error = %v", output, err)
		}
	})
}

func TestRegistryKeepsHistoricalBuildExplicitAndFailsClosedOnDefault(t *testing.T) {
	currentBin, currentDigest := writeFakeNativeCompiler(t, "current-q128")
	historicalBin, historicalDigest := writeFakeNativeCompiler(t, "historical")

	setManifest := func(t *testing.T, specs []CompilerSpec) {
		t.Helper()
		clearVerifierEnv(t)
		setFakeSandboxEnv(t)
		encoded, err := json.Marshal(specs)
		if err != nil {
			t.Fatalf("marshal manifest: %v", err)
		}
		t.Setenv("HYPC_COMPILERS", string(encoded))
	}

	t.Run("historical remains selectable", func(t *testing.T) {
		setManifest(t, []CompilerSpec{
			{Kind: CompilerKindNative, BuildID: "current-q128", Bin: currentBin, SHA256: currentDigest, Default: true},
			{Kind: CompilerKindNative, BuildID: "historical", Bin: historicalBin, SHA256: historicalDigest},
			{Kind: CompilerKindNPM, BuildID: localBuildID, Disabled: true},
		})
		reg, err := NewRegistry()
		if err != nil {
			t.Fatalf("NewRegistry: %v", err)
		}
		t.Cleanup(func() { _ = reg.Close() })
		if got, ok := reg.Resolve(""); !ok || got.BuildID != "current-q128" {
			t.Fatalf("default resolve = %+v, ok=%v", got, ok)
		}
		if got, ok := reg.Resolve("historical"); !ok || got.BuildID != "historical" {
			t.Fatalf("historical resolve = %+v, ok=%v", got, ok)
		}
		if _, ok := reg.Resolve("unknown"); ok {
			t.Fatal("unknown compiler should not resolve")
		}
		info := reg.Info()
		if info.BuildID != "current-q128" || info.Kind != CompilerKindNative || info.SHA256 != currentDigest ||
			info.Provenance == nil || len(info.Compilers) != 2 ||
			info.Compilers[1].SHA256 != historicalDigest {
			t.Fatalf("compiler info = %+v", info)
		}
	})

	t.Run("failed default does not promote historical", func(t *testing.T) {
		setManifest(t, []CompilerSpec{
			{Kind: CompilerKindNative, BuildID: "wrong-current-id", Bin: currentBin, SHA256: currentDigest, Default: true},
			{Kind: CompilerKindNative, BuildID: "historical", Bin: historicalBin, SHA256: historicalDigest},
		})
		if _, err := NewRegistry(); err == nil || !strings.Contains(err.Error(), "default hypc build") {
			t.Fatalf("NewRegistry error = %v, want failed default", err)
		}
	})
}

func TestRegistryAppliesConfiguredOutputLimits(t *testing.T) {
	bin, digest := writeFakeNativeCompiler(t, "output-limits")
	clearVerifierEnv(t)
	setFakeSandboxEnv(t)
	encoded, err := json.Marshal([]CompilerSpec{{
		Kind:    CompilerKindNative,
		BuildID: "output-limits",
		Bin:     bin,
		SHA256:  digest,
		Default: true,
	}})
	if err != nil {
		t.Fatalf("marshal compiler manifest: %v", err)
	}
	t.Setenv("HYPC_COMPILERS", string(encoded))
	t.Setenv("VERIFIER_COMPILER_MAX_STDOUT_BYTES", "4096")
	t.Setenv("VERIFIER_COMPILER_MAX_STDERR_BYTES", "2048")

	reg, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	if reg.Default().MaxStdout != 4096 || reg.Default().MaxStderr != 2048 {
		t.Fatalf(
			"compiler output limits = stdout %d stderr %d",
			reg.Default().MaxStdout,
			reg.Default().MaxStderr,
		)
	}
}

func TestCompilerInfoJSONExposesPinnedProvenance(t *testing.T) {
	digest := strings.Repeat("a", 64)
	nsjailDigest := strings.Repeat("b", 64)
	compiler := &Compiler{
		Language: "Hyperion",
		Kind:     CompilerKindNative,
		SHA256:   digest,
		BuildID:  "current-q128",
		Default:  true,
		Provenance: nativeCompilerProvenance(
			"current-q128",
			digest,
			nsjailDigest,
			sandboxPolicyExpectedSHA256,
		),
	}
	reg := &Registry{
		byID:  map[string]*Compiler{compiler.BuildID: compiler},
		order: []*Compiler{compiler},
		def:   compiler,
	}
	encoded, err := json.Marshal(reg.Info())
	if err != nil {
		t.Fatalf("encode compiler info: %v", err)
	}
	var body map[string]any
	if err := json.Unmarshal(encoded, &body); err != nil {
		t.Fatalf("decode compiler info: %v", err)
	}
	if body["kind"] != CompilerKindNative || body["sha256"] != digest {
		t.Fatalf("compiler info identity = %s", encoded)
	}
	provenance, ok := body["provenance"].(map[string]any)
	if !ok || provenance["buildId"] != compiler.BuildID || provenance["executionDigest"] == "" {
		t.Fatalf("compiler info provenance = %s", encoded)
	}
	builds, ok := body["compilers"].([]any)
	if !ok || len(builds) != 1 {
		t.Fatalf("compiler info builds = %s", encoded)
	}
	build, ok := builds[0].(map[string]any)
	if !ok || build["provenance"] == nil || build["sha256"] != digest {
		t.Fatalf("compiler build provenance = %s", encoded)
	}
}

func TestNativeCompilerProvenanceUsesCanonicalExecutionDigest(t *testing.T) {
	buildID := "current-q128"
	binaryDigest := strings.Repeat("b", 64)
	nsjailDigest := strings.Repeat("c", 64)
	policyDigest := strings.Repeat("d", 64)
	want := models.NativeSandboxCompilerExecutionDigestV2(
		buildID,
		binaryDigest,
		nsjailDigest,
		policyDigest,
	)
	provenance := nativeCompilerProvenance(buildID, binaryDigest, nsjailDigest, policyDigest)
	if provenance.ExecutionDigest != want {
		t.Fatalf("execution digest = %q, want %q", provenance.ExecutionDigest, want)
	}
	if provenance.Schema != compilerProvenanceV2 || provenance.Kind != CompilerKindNative ||
		provenance.BuildID != buildID || len(provenance.Components) != 3 ||
		provenance.Components[0].Name != "hypc" || provenance.Components[0].SHA256 != binaryDigest ||
		provenance.Components[1].Name != "nsjail" || provenance.Components[1].SHA256 != nsjailDigest ||
		provenance.Components[2].Name != "policy" || provenance.Components[2].SHA256 != policyDigest {
		t.Fatalf("compiler provenance = %+v", provenance)
	}
}

func TestCurrentQ128DynamicCompilerFailsClosed(t *testing.T) {
	central, err := filepath.Abs(filepath.Join("..", "..", "..", "hyperion", "build", "hypc", "hypc"))
	if err != nil {
		t.Fatalf("absolute central compiler path: %v", err)
	}
	if _, err := os.Stat(central); err != nil {
		t.Skipf("central Hyperion compiler is not built: %v", err)
	}

	data, err := os.ReadFile(central)
	if err != nil {
		t.Fatalf("read central compiler: %v", err)
	}
	digest := sha256.Sum256(data)
	actualDigest := hex.EncodeToString(digest[:])
	if actualDigest != currentQ128Dynamic {
		t.Skipf("central compiler digest %s is no longer the audited dynamic artifact", actualDigest)
	}

	_, err = newCompiler(CompilerSpec{
		Kind:    CompilerKindNative,
		BuildID: currentQ128BuildID,
		Bin:     central,
		SHA256:  actualDigest,
		Default: true,
	}, fakeSandboxForTest(t), make(chan struct{}, 1), time.Second, 1024, defaultMaxStdoutBytes, defaultMaxStderrBytes)
	if err == nil || !strings.Contains(err.Error(), "statically linked") {
		t.Fatalf("dynamic current compiler error = %v, want static-link rejection", err)
	}
}

func TestCurrentQ128IdentityRejectsDynamicHistoricalSystemCompiler(t *testing.T) {
	const systemCompiler = "/usr/local/bin/hypc"
	data, err := os.ReadFile(systemCompiler)
	if err != nil {
		t.Skipf("system compiler is absent: %v", err)
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) == currentQ128SHA256 {
		t.Skip("system compiler already matches the current Q128 compiler")
	}

	_, err = newCompiler(CompilerSpec{
		Kind:    CompilerKindNative,
		BuildID: currentQ128BuildID,
		Bin:     systemCompiler,
		SHA256:  currentQ128SHA256,
		Default: true,
	}, fakeSandboxForTest(t), make(chan struct{}, 1), time.Second, 1024, defaultMaxStdoutBytes, defaultMaxStderrBytes)
	if err == nil || !strings.Contains(err.Error(), "statically linked") {
		t.Fatalf("historical system compiler error = %v, want static-link rejection", err)
	}
}

func TestCompilerExamplePreservesCurrentAndHistoricalIdentities(t *testing.T) {
	clearVerifierEnv(t)
	path, err := filepath.Abs(filepath.Join("runner", "compilers.example.json"))
	if err != nil {
		t.Fatalf("absolute example manifest path: %v", err)
	}
	t.Setenv("HYPC_COMPILERS", path)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read example manifest: %v", err)
	}
	t.Setenv("HYPC_COMPILERS_SHA256", compilerManifestSHA256(data))
	specs, err := loadCompilerSpecs()
	if err != nil {
		t.Fatalf("load example manifest: %v", err)
	}
	if len(specs) != 3 {
		t.Fatalf("example compiler count = %d, want 3", len(specs))
	}
	if specs[0].Kind != CompilerKindNative || specs[0].BuildID != currentQ128BuildID ||
		specs[0].SHA256 != currentQ128SHA256 || !specs[0].Default {
		t.Fatalf("current example entry = %+v", specs[0])
	}
	if specs[1].Kind != CompilerKindNative || specs[1].BuildID != historicalD5Build ||
		specs[1].SHA256 != historicalD5SHA256 || !specs[1].Disabled || specs[1].Default {
		t.Fatalf("d5 historical entry = %+v", specs[1])
	}
	if specs[2].Kind != CompilerKindNPM || specs[2].BuildID != localBuildID || !specs[2].Disabled || specs[2].Default {
		t.Fatalf("npm historical entry = %+v", specs[2])
	}
}

func TestIgnoredLocalCompilerManifest(t *testing.T) {
	path, err := filepath.Abs(filepath.Join("runner", "compilers.json"))
	if err != nil {
		t.Fatalf("absolute local manifest path: %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Skipf("ignored local compiler manifest is absent: %v", err)
	}

	clearVerifierEnv(t)
	setFakeSandboxEnv(t)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read local compiler manifest: %v", err)
	}
	setFileCompilerManifest(t, path, data)
	reg, err := NewRegistry()
	if err != nil {
		if strings.Contains(err.Error(), "SHA-256 mismatch") || strings.Contains(err.Error(), "statically linked") {
			t.Logf("ignored local manifest awaits static compiler artifacts: %v", err)
			return
		}
		t.Fatalf("local compiler registry: %v", err)
	}
	t.Cleanup(func() { _ = reg.Close() })
	if got := reg.Default(); got.BuildID != currentQ128BuildID || got.SHA256 != currentQ128SHA256 {
		t.Fatalf("local default compiler = %+v", got)
	}
	if got, ok := reg.Resolve(historicalD5Build); !ok || got.SHA256 != historicalD5SHA256 {
		t.Fatalf("local historical compiler = %+v, ok=%v", got, ok)
	}
}
