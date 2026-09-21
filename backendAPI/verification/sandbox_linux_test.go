//go:build linux

package verification

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func writeStaticExitLauncher(t *testing.T, exitCode int) (string, string) {
	t.Helper()
	cc, err := exec.LookPath("cc")
	if err != nil {
		t.Fatalf("C compiler unavailable for static exit launcher: %v", err)
	}
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "exit.c")
	path := filepath.Join(dir, "nsjail-exit")
	source := "int main(void) { return " + strconv.Itoa(exitCode) + "; }\n"
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatalf("write static exit launcher source: %v", err)
	}
	cmd := exec.Command(cc, "-static", "-O2", "-s", "-o", path, sourcePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build static exit launcher: %v: %s", err, output)
	}
	return path, artifactSHA256(t, path)
}

func TestSandboxLauncherSealsPolicyAndSurvivesSourceReplacement(t *testing.T) {
	launcherPath, launcherDigest := writeFakeSandboxLauncher(t)
	sandbox, err := newSandboxLauncher(launcherPath, launcherDigest)
	if err != nil {
		t.Fatalf("newSandboxLauncher: %v", err)
	}
	launcherFD := sandbox.execFile.Fd()
	policyFD := sandbox.policyFile.Fd()

	wantExecSeals := unix.F_SEAL_WRITE | unix.F_SEAL_GROW | unix.F_SEAL_SHRINK | unix.F_SEAL_EXEC | unix.F_SEAL_SEAL
	gotExecSeals, err := unix.FcntlInt(launcherFD, unix.F_GET_SEALS, 0)
	if err != nil || gotExecSeals&wantExecSeals != wantExecSeals {
		t.Fatalf("launcher seals = %#x, %v, want %#x", gotExecSeals, err, wantExecSeals)
	}
	gotPolicySeals, err := unix.FcntlInt(policyFD, unix.F_GET_SEALS, 0)
	if err != nil || gotPolicySeals&wantExecSeals != wantExecSeals {
		t.Fatalf("policy seals = %#x, %v, want %#x", gotPolicySeals, err, wantExecSeals)
	}
	if _, err := sandbox.policyFile.WriteAt([]byte("x"), 0); err == nil {
		t.Fatal("sealed sandbox policy accepted a write")
	}
	if _, err := sandbox.policyFile.Seek(0, io.SeekStart); err != nil {
		t.Fatalf("rewind sandbox policy: %v", err)
	}
	policyBytes, err := io.ReadAll(sandbox.policyFile)
	if err != nil || string(policyBytes) != string(sandboxPolicy) {
		t.Fatalf("sealed policy differs from embedded policy: error=%v", err)
	}

	if err := os.WriteFile(launcherPath, []byte("replaced after snapshot"), 0o700); err != nil {
		t.Fatalf("replace sandbox launcher source: %v", err)
	}
	compilerPath, compilerDigest := writeFakeNativeCompiler(t, "sealed-sandbox")
	compiler, err := newCompiler(
		CompilerSpec{
			Kind:    CompilerKindNative,
			BuildID: "sealed-sandbox",
			Bin:     compilerPath,
			SHA256:  compilerDigest,
			Default: true,
		},
		sandbox,
		make(chan struct{}, 1),
		2*time.Second,
		64*1024,
		defaultMaxStdoutBytes,
		defaultMaxStderrBytes,
	)
	if err != nil {
		t.Fatalf("compiler probe through sealed launcher after source replacement: %v", err)
	}
	if _, err := compiler.Compile(context.Background(), StandardJSONInput{Language: "Hyperion"}); err != nil {
		t.Fatalf("compile through sealed launcher after source replacement: %v", err)
	}
	if err := compiler.Close(); err != nil {
		t.Fatalf("close compiler: %v", err)
	}
	if err := sandbox.Close(); err != nil {
		t.Fatalf("close sandbox: %v", err)
	}
	for name, fd := range map[string]uintptr{"launcher": launcherFD, "policy": policyFD} {
		if _, err := unix.FcntlInt(fd, unix.F_GETFD, 0); !errors.Is(err, unix.EBADF) {
			t.Errorf("%s fd remains open after Close: %v", name, err)
		}
	}
	if err := sandbox.Close(); err != nil {
		t.Fatalf("idempotent sandbox close: %v", err)
	}
}

func TestSandboxLauncherRejectsUnpinnedOrDynamicArtifacts(t *testing.T) {
	staticPath, staticDigest := writeFakeSandboxLauncher(t)
	dynamicPath := writeDynamicNativeCompiler(t)
	dynamicDigest := artifactSHA256(t, dynamicPath)
	symlinkPath := filepath.Join(t.TempDir(), "nsjail-link")
	if err := os.Symlink(staticPath, symlinkPath); err != nil {
		t.Fatalf("create sandbox launcher symlink: %v", err)
	}
	writablePath, writableDigest := writeFakeSandboxLauncher(t)
	if err := os.Chmod(writablePath, 0o775); err != nil {
		t.Fatalf("make sandbox launcher group-writable: %v", err)
	}

	cases := []struct {
		name   string
		path   string
		digest string
		want   string
	}{
		{name: "missing path", digest: staticDigest, want: "VERIFIER_SANDBOX_BIN is required"},
		{name: "relative path", path: "nsjail", digest: staticDigest, want: "absolute path"},
		{name: "missing digest", path: staticPath, want: "VERIFIER_SANDBOX_SHA256 is required"},
		{name: "malformed digest", path: staticPath, digest: "bad", want: "64 hexadecimal"},
		{name: "wrong digest", path: staticPath, digest: strings.Repeat("0", 64), want: "SHA-256 mismatch"},
		{name: "dynamic launcher", path: dynamicPath, digest: dynamicDigest, want: "statically linked"},
		{name: "symlink launcher", path: symlinkPath, digest: staticDigest, want: "must not contain symlinks"},
		{name: "writable launcher", path: writablePath, digest: writableDigest, want: "must not be group or world writable"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			sandbox, err := newSandboxLauncher(tc.path, tc.digest)
			if sandbox != nil {
				_ = sandbox.Close()
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("newSandboxLauncher error = %v, want substring %q", err, tc.want)
			}
		})
	}
}

func TestRegistryRequiresSandboxAndFailsClosedOnProbeError(t *testing.T) {
	compilerPath, compilerDigest := writeFakeNativeCompiler(t, "sandbox-required")
	manifest, err := json.Marshal([]CompilerSpec{{
		Kind:    CompilerKindNative,
		BuildID: "sandbox-required",
		Bin:     compilerPath,
		SHA256:  compilerDigest,
		Default: true,
	}})
	if err != nil {
		t.Fatalf("marshal compiler manifest: %v", err)
	}

	t.Run("missing sandbox configuration", func(t *testing.T) {
		clearVerifierEnv(t)
		t.Setenv("HYPC_COMPILERS", string(manifest))
		if _, err := NewRegistry(); err == nil || !strings.Contains(err.Error(), "VERIFIER_SANDBOX_BIN is required") {
			t.Fatalf("NewRegistry error = %v, want missing sandbox failure", err)
		}
	})

	t.Run("launcher probe error", func(t *testing.T) {
		clearVerifierEnv(t)
		exitPath, exitDigest := writeStaticExitLauncher(t, 77)
		t.Setenv("VERIFIER_SANDBOX_BIN", exitPath)
		t.Setenv("VERIFIER_SANDBOX_SHA256", exitDigest)
		t.Setenv("HYPC_COMPILERS", string(manifest))
		if _, err := NewRegistry(); err == nil ||
			!strings.Contains(err.Error(), "default hypc build") ||
			!strings.Contains(err.Error(), "version probe failed") {
			t.Fatalf("NewRegistry error = %v, want fail-closed sandbox probe error", err)
		}
	})
}

func TestSandboxPolicyDigestUsesExactEmbeddedBytes(t *testing.T) {
	digest := sha256.Sum256(sandboxPolicy)
	if got := hex.EncodeToString(digest[:]); got != sandboxPolicyExpectedSHA256 {
		t.Fatalf("embedded policy digest = %q, want %q", got, sandboxPolicyExpectedSHA256)
	}
}
