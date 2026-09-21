package verification

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"backendAPI/models"
)

const (
	CompilerKindNative    = "native"
	CompilerKindNPM       = "npm"
	compilerProvenanceV2  = models.CompilerProvenanceSchemaV2
	defaultMaxStdinBytes  = 256 << 10
	defaultMaxStdoutBytes = 16 << 20
	defaultMaxStderrBytes = 256 << 10
)

// Compiler wraps one pinned native hypc build behind a sealed executable
// snapshot. One Compiler equals one build id and one execution digest.
//
// The sandboxing budget (per-compile timeout, stdin size cap, and the
// concurrency semaphore) is supplied by the owning Registry and SHARED
// across every build, so the total number of in-flight compiler processes is
// bounded regardless of which build a request selects.
type Compiler struct {
	Language   string                    // always "Hyperion" today; carried explicitly for the API
	Kind       string                    // execution kind; only native is currently enabled
	SHA256     string                    // exact native hypc binary digest
	BuildID    string                    // pinned version string the snapshot must report
	Default    bool                      // true for the build used when the client omits compilerVersion
	Provenance models.CompilerProvenance // canonical artifact identity persisted with verification results

	Timeout   time.Duration // per-compile hard deadline (shared across builds)
	MaxStdin  int           // standard-JSON payload cap in bytes (shared across builds)
	MaxStdout int           // compiler JSON output cap in bytes (shared across builds)
	MaxStderr int           // compiler diagnostic output cap in bytes (shared across builds)
	sem       chan struct{} // concurrency permit, SHARED across all builds

	execMu   sync.RWMutex
	execFile *os.File // sealed memfd retained for registry lifetime
	sandbox  *sandboxLauncher
}

// ErrPayloadTooLarge is returned when the standard-JSON payload exceeds
// MaxStdin. The cap protects the compiler from being asked to compile a
// payload so large that its mere parse exhausts heap.
var ErrPayloadTooLarge = errors.New("verification: standard-JSON payload exceeds MaxStdin")

// ErrCompileTimeout is returned when the per-call deadline fires.
var ErrCompileTimeout = errors.New("verification: compile timed out")

// ErrCompilerOutputTooLarge is returned when compiler stdout exceeds the
// configured JSON output cap. The process group is killed immediately.
var ErrCompilerOutputTooLarge = errors.New("verification: compiler stdout exceeds MaxStdout")

// ErrCompilerStderrTooLarge is returned when compiler stderr exceeds the
// configured diagnostic output cap. The process group is killed immediately.
var ErrCompilerStderrTooLarge = errors.New("verification: compiler stderr exceeds MaxStderr")

// newCompiler builds a single Compiler from a spec, wires in the shared
// limits, and gates construction on a successful version probe: the compiler
// must report exactly spec.BuildID. A probe failure or mismatch is returned
// to the Registry, which rejects a failed default and may skip a failed
// historical build.
func newCompiler(
	spec CompilerSpec,
	sandbox *sandboxLauncher,
	sem chan struct{},
	timeout time.Duration,
	maxStdin int,
	maxStdout int,
	maxStderr int,
) (*Compiler, error) {
	if sandbox == nil {
		return nil, errors.New("sealed NsJail sandbox launcher is required")
	}
	if spec.BuildID == "" {
		return nil, errors.New("buildId is required")
	}
	if spec.Kind != CompilerKindNative {
		return nil, fmt.Errorf("compiler kind %q is unavailable without complete runtime provenance", spec.Kind)
	}
	if spec.Runner != "" || spec.NodeBin != "" {
		return nil, errors.New("native compiler entries must not configure runner or nodeBin")
	}
	expectedSHA256, err := normalizeSHA256(spec.SHA256)
	if err != nil {
		return nil, err
	}
	if spec.Bin == "" {
		return nil, errors.New("native compiler bin is required")
	}
	if !filepath.IsAbs(spec.Bin) {
		return nil, errors.New("native compiler bin must be an absolute path")
	}
	if expectedSHA256 == "" {
		return nil, errors.New("sha256 is required for a native compiler")
	}

	execFile, err := snapshotExecutable(spec.Bin, expectedSHA256)
	if err != nil {
		return nil, err
	}

	c := &Compiler{
		Language: "Hyperion",
		Kind:     spec.Kind,
		SHA256:   expectedSHA256,
		BuildID:  spec.BuildID,
		Default:  spec.Default,
		Provenance: nativeCompilerProvenance(
			spec.BuildID,
			expectedSHA256,
			sandbox.SHA256,
			sandbox.PolicySHA256,
		),
		Timeout:   timeout,
		MaxStdin:  maxStdin,
		MaxStdout: normalizedPositiveLimit(maxStdout, defaultMaxStdoutBytes),
		MaxStderr: normalizedPositiveLimit(maxStderr, defaultMaxStderrBytes),
		sem:       sem,
		execFile:  execFile,
		sandbox:   sandbox,
	}

	got, err := c.probeVersion()
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("version probe failed: %w", err)
	}
	if got != spec.BuildID {
		c.Close()
		return nil, fmt.Errorf("build mismatch: want %q, got %q", spec.BuildID, got)
	}
	return c, nil
}

// Compile invokes the sealed compiler with the supplied standard-JSON input and
// parses its JSON output. Compile errors (severity=="error") are
// returned in the StandardJSONOutput.Errors field, they are NOT a
// Go-level error. Only infrastructure failures (timeout, marshalling,
// compiler non-zero exit) surface as error returns.
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
	stdout, stderr, err := c.runSnapshot(subCtx, []string{"--standard-json"}, bytes.NewReader(payload))
	if err != nil {
		if errors.Is(err, ErrCompilerOutputTooLarge) || errors.Is(err, ErrCompilerStderrTooLarge) {
			return nil, err
		}
		if errors.Is(subCtx.Err(), context.DeadlineExceeded) {
			return nil, ErrCompileTimeout
		}
		return nil, fmt.Errorf("hypc exec: %w (stderr: %s)", err, strings.TrimSpace(string(stderr)))
	}

	var out StandardJSONOutput
	if err := json.Unmarshal(stdout, &out); err != nil {
		return nil, fmt.Errorf("hypc produced invalid JSON: %w", err)
	}
	return &out, nil
}

// probeVersion runs the sealed compiler with --version and returns its exact
// stdout. Used at startup to gate boot on a build-ID match.
func (c *Compiler) probeVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, stderr, err := c.runSnapshot(ctx, []string{"--version"}, nil)
	if err != nil {
		return "", fmt.Errorf("exec sealed compiler: %w (stderr: %s)", err, strings.TrimSpace(string(stderr)))
	}
	for _, line := range strings.Split(string(out), "\n") {
		if version, ok := strings.CutPrefix(strings.TrimSpace(line), "Version:"); ok {
			version = strings.TrimSpace(version)
			if version != "" {
				return version, nil
			}
		}
	}
	return "", errors.New("sealed compiler version output has no Version field")
}

func (c *Compiler) runSnapshot(ctx context.Context, args []string, stdin *bytes.Reader) ([]byte, []byte, error) {
	c.execMu.RLock()
	defer c.execMu.RUnlock()
	if c.execFile == nil {
		return nil, nil, errors.New("compiler snapshot is closed")
	}

	if c.sandbox == nil {
		return nil, nil, errors.New("sandbox launcher is unavailable")
	}
	return c.sandbox.run(ctx, c.execFile, args, stdin, c.MaxStdout, c.MaxStderr)
}

func normalizedPositiveLimit(configured, fallback int) int {
	if configured < 1 {
		return fallback
	}
	return configured
}

func normalizeSHA256(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	normalized := strings.ToLower(strings.TrimSpace(value))
	digest, err := hex.DecodeString(normalized)
	if err != nil || len(digest) != sha256.Size {
		return "", errors.New("sha256 must be exactly 64 hexadecimal characters")
	}
	return normalized, nil
}

func nativeCompilerProvenance(
	buildID string,
	binarySHA256 string,
	nsjailSHA256 string,
	policySHA256 string,
) models.CompilerProvenance {
	return models.CompilerProvenance{
		Schema:  compilerProvenanceV2,
		Kind:    CompilerKindNative,
		BuildID: buildID,
		ExecutionDigest: models.NativeSandboxCompilerExecutionDigestV2(
			buildID,
			binarySHA256,
			nsjailSHA256,
			policySHA256,
		),
		Components: []models.CompilerProvenanceComponent{
			{Name: "hypc", SHA256: binarySHA256},
			{Name: "nsjail", SHA256: nsjailSHA256},
			{Name: "policy", SHA256: policySHA256},
		},
	}
}

func (c *Compiler) ProvenanceRecord() *models.CompilerProvenance {
	copyRecord := c.Provenance
	copyRecord.Components = append([]models.CompilerProvenanceComponent(nil), c.Provenance.Components...)
	return &copyRecord
}

func (c *Compiler) Close() error {
	c.execMu.Lock()
	defer c.execMu.Unlock()
	if c.execFile == nil {
		return nil
	}
	err := c.execFile.Close()
	c.execFile = nil
	return err
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
