package verification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Compiler wraps ONE pinned @theqrl/hypc build behind a one-shot subprocess
// (the WASM-via-Node runner hypc-runner.js, or the native hypc-native.sh
// wrapper). One Compiler == one build id.
//
// The sandboxing budget (per-compile timeout, stdin size cap, and the
// concurrency semaphore) is supplied by the owning Registry and SHARED
// across every build, so the total number of in-flight runner processes is
// bounded regardless of which build a request selects.
type Compiler struct {
	Language   string // always "Hyperion" today; carried explicitly for the API
	NodeBin    string // path to the interpreter (`node`, or `/bin/sh` for the native wrapper)
	RunnerPath string // absolute path to the runner (hypc-runner.js / hypc-native.sh)
	Bin        string // optional native hypc path, passed to the runner as HYPC_BIN
	BuildID    string // pinned version string the runner must report
	Default    bool   // true for the build used when the client omits compilerVersion

	Timeout  time.Duration // per-compile hard deadline (shared across builds)
	MaxStdin int           // standard-JSON payload cap in bytes (shared across builds)
	sem      chan struct{} // concurrency permit, SHARED across all builds
}

// ErrPayloadTooLarge is returned when the standard-JSON payload exceeds
// MaxStdin. The cap protects the runner from being asked to compile a
// payload so large that its mere parse exhausts heap.
var ErrPayloadTooLarge = errors.New("verification: standard-JSON payload exceeds MaxStdin")

// ErrCompileTimeout is returned when the per-call deadline fires.
var ErrCompileTimeout = errors.New("verification: compile timed out")

// newCompiler builds a single Compiler from a spec, wires in the shared
// limits, and gates construction on a successful version probe: the runner
// must report exactly spec.BuildID. A probe failure / mismatch is returned
// as an error so the Registry can skip just this build.
func newCompiler(spec CompilerSpec, sem chan struct{}, timeout time.Duration, maxStdin int) (*Compiler, error) {
	nodeBin := spec.NodeBin
	if nodeBin == "" {
		nodeBin = "node"
	}
	if spec.Runner == "" {
		return nil, errors.New("runner path is required")
	}
	if spec.BuildID == "" {
		return nil, errors.New("buildId is required")
	}

	c := &Compiler{
		Language:   "Hyperion",
		NodeBin:    nodeBin,
		RunnerPath: spec.Runner,
		Bin:        spec.Bin,
		BuildID:    spec.BuildID,
		Default:    spec.Default,
		Timeout:    timeout,
		MaxStdin:   maxStdin,
		sem:        sem,
	}

	got, err := c.probeVersion()
	if err != nil {
		return nil, fmt.Errorf("version probe failed: %w", err)
	}
	if got != spec.BuildID {
		return nil, fmt.Errorf("build mismatch: want %q, got %q", spec.BuildID, got)
	}
	return c, nil
}

// Compile invokes the runner with the supplied standard-JSON input and
// parses its JSON output. Compile errors (severity=="error") are
// returned in the StandardJSONOutput.Errors field, they are NOT a
// Go-level error. Only infrastructure failures (timeout, marshalling,
// runner non-zero exit) surface as error returns.
func (c *Compiler) Compile(ctx context.Context, input StandardJSONInput) (*StandardJSONOutput, error) {
	payload, err := json.Marshal(input)
	if err != nil {
		return nil, fmt.Errorf("marshal standard-JSON: %w", err)
	}
	if c.MaxStdin > 0 && len(payload) > c.MaxStdin {
		return nil, ErrPayloadTooLarge
	}

	// Acquire concurrency permit. Honour caller's context (so a request
	// timeout fires even while we're queued).
	select {
	case c.sem <- struct{}{}:
		defer func() { <-c.sem }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// Apply our per-call timeout on top of whatever the caller provided.
	subCtx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()

	cmd := exec.CommandContext(subCtx, c.NodeBin, c.RunnerPath)
	cmd.Stdin = bytes.NewReader(payload)
	// Scrub the env. Two pass-throughs:
	//   PATH     , node + shell builtins
	//   HYPC_BIN , lets the native-binary runner (hypc-native.sh) pin an
	//               absolute hypc path independent of PATH order. Ignored
	//               by the WASM-via-Node runner.
	cmd.Env = c.runnerEnv()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		if errors.Is(subCtx.Err(), context.DeadlineExceeded) {
			return nil, ErrCompileTimeout
		}
		return nil, fmt.Errorf("hypc-runner exec: %w (stderr: %s)", err, strings.TrimSpace(stderr.String()))
	}

	var out StandardJSONOutput
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return nil, fmt.Errorf("hypc-runner produced invalid JSON: %w", err)
	}
	return &out, nil
}

// probeVersion runs the runner with --version and returns the trimmed
// stdout. Used at startup to gate boot on a build-ID match.
func (c *Compiler) probeVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, c.NodeBin, c.RunnerPath, "--version")
	cmd.Env = c.runnerEnv()
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// runnerEnv builds the minimal env passed to this build's runner subprocess.
// Each build pins its own HYPC_BIN (so two native builds can point at
// different hypc binaries). When a build doesn't set Bin we fall back to a
// process-level HYPC_BIN, this preserves the pre-multi-version single-build
// deploy where HYPC_BIN was exported once in the environment.
func (c *Compiler) runnerEnv() []string {
	env := []string{"PATH=" + os.Getenv("PATH")}
	bin := c.Bin
	if bin == "" {
		bin = os.Getenv("HYPC_BIN")
	}
	if bin != "" {
		env = append(env, "HYPC_BIN="+bin)
	}
	return env
}

func envWithDefault(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

func envInt(k string, d int) int {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	var n int
	if _, err := fmt.Sscanf(v, "%d", &n); err != nil {
		return d
	}
	return n
}

func envDuration(k string, d time.Duration) time.Duration {
	v := os.Getenv(k)
	if v == "" {
		return d
	}
	dur, err := time.ParseDuration(v)
	if err != nil {
		return d
	}
	return dur
}
