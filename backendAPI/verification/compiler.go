package verification

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Compiler wraps the @theqrl/hypc WASM via a one-shot Node subprocess
// (backendAPI/verification/runner/hypc-runner.js). The Go layer holds the
// sandboxing budget — timeout, concurrency cap, stdin size cap, scratch
// env — so the runner can stay tiny.
type Compiler struct {
	NodeBin    string        // path to the `node` binary
	RunnerPath string        // absolute path to hypc-runner.js
	BuildID    string        // pinned version string (e.g. "0.0.2+commit.3e18e55d.Emscripten.clang")
	Timeout    time.Duration // per-compile hard deadline
	MaxStdin   int           // bytes — reject standard-JSON inputs larger than this

	sem  chan struct{}
	once sync.Once
}

// ErrPayloadTooLarge is returned when the standard-JSON payload exceeds
// MaxStdin. The cap protects the runner from being asked to compile a
// payload so large that its mere parse exhausts heap.
var ErrPayloadTooLarge = errors.New("verification: standard-JSON payload exceeds MaxStdin")

// ErrCompileTimeout is returned when the per-call deadline fires.
var ErrCompileTimeout = errors.New("verification: compile timed out")

// NewCompiler constructs a Compiler from environment variables:
//
//	HYPC_NODE_BIN           path to the node binary           (default "node")
//	HYPC_RUNNER             abs path to hypc-runner.js        (required)
//	HYPC_BUILD_ID           pinned version string             (required)
//	VERIFIER_MAX_CONCURRENCY in-process concurrent compiles   (default 2)
//	VERIFIER_COMPILE_TIMEOUT per-compile hard timeout         (default 30s)
//	VERIFIER_SOURCE_MAX_BYTES standard-JSON payload cap       (default 256KiB)
//
// The Compiler fails fast at startup if HYPC_RUNNER / HYPC_BUILD_ID are
// missing or if the runner's reported version doesn't match HYPC_BUILD_ID.
func NewCompiler() (*Compiler, error) {
	nodeBin := envWithDefault("HYPC_NODE_BIN", "node")
	runner := os.Getenv("HYPC_RUNNER")
	if runner == "" {
		return nil, errors.New("HYPC_RUNNER env var is required (abs path to hypc-runner.js)")
	}
	buildID := os.Getenv("HYPC_BUILD_ID")
	if buildID == "" {
		return nil, errors.New("HYPC_BUILD_ID env var is required (pinned hypc version string)")
	}
	maxConc := envInt("VERIFIER_MAX_CONCURRENCY", 2)
	if maxConc < 1 {
		maxConc = 1
	}
	timeout := envDuration("VERIFIER_COMPILE_TIMEOUT", 30*time.Second)
	maxStdin := envInt("VERIFIER_SOURCE_MAX_BYTES", 256*1024)

	c := &Compiler{
		NodeBin:    nodeBin,
		RunnerPath: runner,
		BuildID:    buildID,
		Timeout:    timeout,
		MaxStdin:   maxStdin,
		sem:        make(chan struct{}, maxConc),
	}

	// Validate the runner can be invoked + reports the expected build.
	got, err := c.probeVersion()
	if err != nil {
		return nil, fmt.Errorf("hypc-runner version probe failed: %w", err)
	}
	if got != buildID {
		return nil, fmt.Errorf("hypc-runner build mismatch: want %q, got %q", buildID, got)
	}
	log.Printf("verification.Compiler ready: runner=%s build=%s timeout=%s concurrency=%d", runner, buildID, timeout, maxConc)
	return c, nil
}

// CompilerInfo returns the pinned build ID. Used by GET /contract/compiler-info.
func (c *Compiler) CompilerInfo() CompilerInfoResponse {
	return CompilerInfoResponse{Language: "Hyperion", BuildID: c.BuildID}
}

// Compile invokes the runner with the supplied standard-JSON input and
// parses its JSON output. Compile errors (severity=="error") are
// returned in the StandardJSONOutput.Errors field — they are NOT a
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
	// Scrub the env — the runner only needs PATH to find node-bundled
	// shared libs. Anything else is signalling we don't want leaking.
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}

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
	cmd.Env = []string{"PATH=" + os.Getenv("PATH")}
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
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
