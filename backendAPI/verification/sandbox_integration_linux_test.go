//go:build linux

package verification

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

const realSandboxTestEnv = "VERIFIER_REAL_SANDBOX_TEST"

func realSandboxForIntegrationTest(t *testing.T) *sandboxLauncher {
	t.Helper()
	if os.Getenv(realSandboxTestEnv) != "1" {
		t.Skipf("set %s=1 to run real NsJail integration tests", realSandboxTestEnv)
	}
	path := strings.TrimSpace(os.Getenv("VERIFIER_SANDBOX_BIN"))
	digest := strings.TrimSpace(os.Getenv("VERIFIER_SANDBOX_SHA256"))
	if path == "" || digest == "" {
		t.Fatalf("%s=1 requires VERIFIER_SANDBOX_BIN and VERIFIER_SANDBOX_SHA256", realSandboxTestEnv)
	}
	sandbox, err := newSandboxLauncher(path, digest)
	if err != nil {
		t.Fatalf("load real NsJail sandbox: %v", err)
	}
	t.Cleanup(func() {
		if err := sandbox.Close(); err != nil {
			t.Errorf("close real NsJail sandbox: %v", err)
		}
	})
	return sandbox
}

func writeSandboxProbeCompiler(t *testing.T, behavior, markerPath string) (string, string, string) {
	t.Helper()
	cc, err := exec.LookPath("cc")
	if err != nil {
		t.Fatalf("C compiler unavailable for sandbox probe: %v", err)
	}
	buildID := "zondscan-sandbox-probe-" + behavior
	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "probe.c")
	binaryPath := filepath.Join(dir, "hypc-probe")
	source := fmt.Sprintf(`#define _GNU_SOURCE
#include <errno.h>
#include <fcntl.h>
#include <stdio.h>
#include <string.h>
#include <sys/resource.h>
#include <sys/socket.h>
#include <sys/syscall.h>
#include <sys/utsname.h>
#include <unistd.h>

extern char **environ;

static int drain_stdin(void) {
  char buf[4096];
  ssize_t got;
  do {
    got = read(STDIN_FILENO, buf, sizeof(buf));
  } while (got > 0);
  return got < 0 ? 70 : 0;
}

static int check_limit(int resource, rlim_t want) {
  struct rlimit limit;
  if (getrlimit(resource, &limit) != 0) return 71;
  return limit.rlim_cur == want && limit.rlim_max == want ? 0 : 72;
}

int main(int argc, char **argv) {
  if (argc == 2 && strcmp(argv[1], "--version") == 0) {
    puts("Version: %s");
    return 0;
  }
  if (argc != 2 || strcmp(argv[1], "--standard-json") != 0) return 64;
  if (drain_stdin() != 0) return 70;

  if (strcmp(%s, "isolation") == 0) {
    struct utsname identity;
    char target[16];
    if (environ != NULL && environ[0] != NULL) return 73;
    if (uname(&identity) != 0 || strcmp(identity.nodename, "zondscan-hypc") != 0) return 74;
    errno = 0;
    if (readlinkat(AT_FDCWD, "/etc/passwd", target, sizeof(target)) != -1 || errno != ENOENT) return 75;
    if (check_limit(RLIMIT_AS, 2048ULL * 1024ULL * 1024ULL) != 0) return 76;
    if (check_limit(RLIMIT_CPU, 30) != 0) return 77;
    if (check_limit(RLIMIT_CORE, 0) != 0) return 78;
    if (check_limit(RLIMIT_FSIZE, 0) != 0) return 79;
    if (check_limit(RLIMIT_NOFILE, 16) != 0) return 80;
    if (check_limit(RLIMIT_NPROC, 1) != 0) return 81;
    puts("{\"contracts\":{}}");
    return 0;
  }
  if (strcmp(%s, "host-read") == 0) {
    return syscall(SYS_openat, AT_FDCWD, "/etc/passwd", O_RDONLY, 0) >= 0 ? 82 : 83;
  }
  if (strcmp(%s, "host-write") == 0) {
    return syscall(SYS_openat, AT_FDCWD, %s, O_WRONLY | O_CREAT | O_TRUNC, 0600) >= 0 ? 84 : 85;
  }
  if (strcmp(%s, "tcp-socket") == 0) {
    return syscall(SYS_socket, AF_INET, SOCK_STREAM, 0) >= 0 ? 86 : 87;
  }
  if (strcmp(%s, "unix-socket") == 0) {
    return syscall(SYS_socket, AF_UNIX, SOCK_STREAM, 0) >= 0 ? 88 : 89;
  }
  if (strcmp(%s, "process") == 0) {
    return syscall(SYS_fork) >= 0 ? 90 : 91;
  }
  return 92;
}
`, buildID, strconv.Quote(behavior), strconv.Quote(behavior), strconv.Quote(behavior), strconv.Quote(markerPath), strconv.Quote(behavior), strconv.Quote(behavior), strconv.Quote(behavior))
	if err := os.WriteFile(sourcePath, []byte(source), 0o600); err != nil {
		t.Fatalf("write sandbox probe source: %v", err)
	}
	cmd := exec.Command(cc, "-static", "-O2", "-s", "-o", binaryPath, sourcePath)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build sandbox probe: %v: %s", err, output)
	}
	return binaryPath, artifactSHA256(t, binaryPath), buildID
}

func compilerWithRealSandbox(
	t *testing.T,
	sandbox *sandboxLauncher,
	path string,
	digest string,
	buildID string,
) *Compiler {
	t.Helper()
	compiler, err := newCompiler(
		CompilerSpec{
			Kind:    CompilerKindNative,
			BuildID: buildID,
			Bin:     path,
			SHA256:  digest,
			Default: true,
		},
		sandbox,
		make(chan struct{}, 1),
		5*time.Second,
		64*1024,
		defaultMaxStdoutBytes,
		defaultMaxStderrBytes,
	)
	if err != nil {
		t.Fatalf("construct compiler through real NsJail: %v", err)
	}
	t.Cleanup(func() {
		if err := compiler.Close(); err != nil {
			t.Errorf("close sandbox probe compiler: %v", err)
		}
	})
	return compiler
}

func TestRealSandboxIsolationAndMandatoryLimits(t *testing.T) {
	sandbox := realSandboxForIntegrationTest(t)
	path, digest, buildID := writeSandboxProbeCompiler(t, "isolation", "")
	compiler := compilerWithRealSandbox(t, sandbox, path, digest, buildID)
	if _, err := compiler.Compile(context.Background(), StandardJSONInput{Language: "Hyperion"}); err != nil {
		t.Fatalf("sandbox isolation probe failed: %v", err)
	}
}

func TestRealSandboxRejectsHostAndKernelAttackSurface(t *testing.T) {
	sandbox := realSandboxForIntegrationTest(t)
	markerPath := filepath.Join(t.TempDir(), "host-write-marker")
	for _, behavior := range []string{"host-read", "host-write", "tcp-socket", "unix-socket", "process"} {
		t.Run(behavior, func(t *testing.T) {
			path, digest, buildID := writeSandboxProbeCompiler(t, behavior, markerPath)
			compiler := compilerWithRealSandbox(t, sandbox, path, digest, buildID)
			_, err := compiler.Compile(context.Background(), StandardJSONInput{Language: "Hyperion"})
			if err == nil || !strings.Contains(err.Error(), "exit status 159") {
				t.Fatalf("hostile %s error = %v, want seccomp SIGSYS exit 159", behavior, err)
			}
		})
	}
	if _, err := os.Stat(markerPath); !os.IsNotExist(err) {
		t.Fatalf("host write marker exists after sandbox probe: %v", err)
	}
}

func TestRealSandboxFailsClosedOnInvalidPolicy(t *testing.T) {
	sandbox := realSandboxForIntegrationTest(t)
	invalidPolicy, err := sealSandboxPolicyBytes([]byte("seccomp_string: \"BROKEN POLICY\"\n"))
	if err != nil {
		t.Fatalf("seal invalid policy fixture: %v", err)
	}
	if err := sandbox.policyFile.Close(); err != nil {
		t.Fatalf("close valid policy fixture: %v", err)
	}
	sandbox.policyFile = invalidPolicy

	path, digest, buildID := writeSandboxProbeCompiler(t, "isolation", "")
	compiler, err := newCompiler(
		CompilerSpec{
			Kind:    CompilerKindNative,
			BuildID: buildID,
			Bin:     path,
			SHA256:  digest,
			Default: true,
		},
		sandbox,
		make(chan struct{}, 1),
		5*time.Second,
		64*1024,
		defaultMaxStdoutBytes,
		defaultMaxStderrBytes,
	)
	if compiler != nil {
		_ = compiler.Close()
	}
	if err == nil ||
		!strings.Contains(err.Error(), "version probe failed") ||
		!strings.Contains(err.Error(), "exit status 255") {
		t.Fatalf("invalid NsJail policy error = %v, want fail-closed probe exit 255", err)
	}
}

func TestRealStaticHyperionCompilesThroughSandbox(t *testing.T) {
	sandbox := realSandboxForIntegrationTest(t)
	path := strings.TrimSpace(os.Getenv("VERIFIER_TEST_HYPC_BIN"))
	digest := strings.TrimSpace(os.Getenv("VERIFIER_TEST_HYPC_SHA256"))
	buildID := strings.TrimSpace(os.Getenv("VERIFIER_TEST_HYPC_BUILD_ID"))
	if path == "" || digest == "" || buildID == "" {
		t.Skip("set VERIFIER_TEST_HYPC_BIN, VERIFIER_TEST_HYPC_SHA256, and VERIFIER_TEST_HYPC_BUILD_ID for the real Hyperion probe")
	}
	compiler := compilerWithRealSandbox(t, sandbox, path, digest, buildID)
	input := StandardJSONInput{
		Language: "Hyperion",
		Sources: map[string]StandardJSONSource{
			"Counter.hyp": {Content: "contract Counter { uint256 public value; function set(uint256 next) public { value = next; } }"},
		},
		Settings: StandardJSONSettings{
			Optimizer: &Optimizer{Enabled: true, Runs: 200},
			OutputSelection: map[string]map[string][]string{
				"*": {"*": {"abi", "qrvm.bytecode.object", "qrvm.deployedBytecode.object"}},
			},
		},
	}
	output, err := compiler.Compile(context.Background(), input)
	if err != nil {
		t.Fatalf("compile representative Hyperion contract through NsJail: %v", err)
	}
	contracts := output.Contracts["Counter.hyp"]
	if len(contracts) == 0 {
		t.Fatalf("representative Hyperion output has no Counter.hyp contracts: %#v", output.Errors)
	}
	payload, err := json.Marshal(input)
	if err != nil {
		t.Fatalf("marshal representative Hyperion input: %v", err)
	}
	direct := exec.Command(path, "--standard-json")
	direct.Env = []string{}
	direct.Stdin = bytes.NewReader(payload)
	directOutput, err := direct.Output()
	if err != nil {
		t.Fatalf("compile representative Hyperion contract directly: %v", err)
	}
	sandboxOutput, stderr, err := compiler.runSnapshot(
		context.Background(),
		[]string{"--standard-json"},
		bytes.NewReader(payload),
	)
	if err != nil {
		t.Fatalf("compile representative Hyperion contract for parity: %v (stderr: %s)", err, stderr)
	}
	if !bytes.Equal(sandboxOutput, directOutput) {
		t.Fatalf("sandboxed Hyperion output differs from direct output: sandbox=%d bytes direct=%d bytes", len(sandboxOutput), len(directOutput))
	}
}
