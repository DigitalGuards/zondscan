package verification

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"
	"time"
)

// CompilerSpec is one entry in the HYPC_COMPILERS manifest. It describes how
// to invoke a single hypc build; the runner contract is shared with the
// legacy single-build env vars (see backendAPI/verification/runner/).
type CompilerSpec struct {
	BuildID string `json:"buildId"`           // required: version string the runner must report
	NodeBin string `json:"nodeBin,omitempty"` // interpreter; default "node"
	Runner  string `json:"runner"`            // required: abs path to hypc-runner.js / hypc-native.sh
	Bin     string `json:"bin,omitempty"`     // optional: native hypc path → HYPC_BIN for this build
	Default bool   `json:"default,omitempty"` // optional: the build used when a request omits compilerVersion
}

// Registry holds every configured (and probe-verified) hypc build keyed by
// build id, plus the build to use when a request doesn't pick one. Exactly
// one build is always the default once construction succeeds.
type Registry struct {
	byID  map[string]*Compiler
	order []*Compiler // stable display/iteration order (config order)
	def   *Compiler
}

// NewRegistry builds the compiler registry from the environment.
//
// Preferred (multi-build): HYPC_COMPILERS holds either an inline JSON array
// of CompilerSpec, or a path to a file containing that array.
//
// Legacy (single-build, still supported): when HYPC_COMPILERS is unset, a
// one-entry registry is synthesised from HYPC_RUNNER / HYPC_BUILD_ID /
// HYPC_NODE_BIN / HYPC_BIN exactly as the pre-multi-version backend did, so
// existing deploys keep working untouched.
//
// Shared limits — VERIFIER_MAX_CONCURRENCY / VERIFIER_COMPILE_TIMEOUT /
// VERIFIER_SOURCE_MAX_BYTES — bound ALL builds together (one process-wide
// compile budget) regardless of which build a request selects.
//
// A build whose runner can't be invoked, or reports the wrong version, is
// skipped with a warning rather than disabling the whole feature. Only when
// zero builds survive does NewRegistry return an error (verification then
// boots disabled and the routes answer 503).
func NewRegistry() (*Registry, error) {
	specs, err := loadCompilerSpecs()
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return nil, errors.New("no hypc builds configured (set HYPC_COMPILERS, or HYPC_RUNNER+HYPC_BUILD_ID)")
	}

	maxConc := envInt("VERIFIER_MAX_CONCURRENCY", 2)
	if maxConc < 1 {
		maxConc = 1
	}
	timeout := envDuration("VERIFIER_COMPILE_TIMEOUT", 30*time.Second)
	maxStdin := envInt("VERIFIER_SOURCE_MAX_BYTES", 256*1024)
	sem := make(chan struct{}, maxConc)

	reg := &Registry{byID: map[string]*Compiler{}}
	for _, spec := range specs {
		if _, dup := reg.byID[spec.BuildID]; dup {
			log.Printf("WARN: duplicate hypc build %q in HYPC_COMPILERS, keeping the first", spec.BuildID)
			continue
		}
		c, err := newCompiler(spec, sem, timeout, maxStdin)
		if err != nil {
			log.Printf("WARN: skipping hypc build %q (runner=%s): %v", spec.BuildID, spec.Runner, err)
			continue
		}
		reg.byID[c.BuildID] = c
		reg.order = append(reg.order, c)
		if c.Default && reg.def == nil {
			reg.def = c
		}
	}

	if len(reg.order) == 0 {
		return nil, errors.New("no usable hypc builds (every configured runner failed its version probe)")
	}

	// Guarantee exactly one default and keep the Default flags consistent
	// with reality (an explicit default may have been the build that got
	// skipped above).
	if reg.def == nil {
		reg.def = reg.order[0]
	}
	for _, c := range reg.order {
		c.Default = c == reg.def
	}

	log.Printf("verification.Registry ready: %d build(s) %v, default=%s, timeout=%s concurrency=%d",
		len(reg.order), reg.SupportedBuildIDs(), reg.def.BuildID, timeout, maxConc)
	return reg, nil
}

// loadCompilerSpecs reads the HYPC_COMPILERS manifest (inline JSON or a file
// path), falling back to the legacy single-build env vars when unset.
func loadCompilerSpecs() ([]CompilerSpec, error) {
	raw := strings.TrimSpace(os.Getenv("HYPC_COMPILERS"))
	if raw == "" {
		return legacyCompilerSpecs(), nil
	}

	data := []byte(raw)
	// Anything that doesn't start with a JSON array is treated as a path.
	if !strings.HasPrefix(raw, "[") {
		b, err := os.ReadFile(raw)
		if err != nil {
			return nil, fmt.Errorf("read HYPC_COMPILERS file %q: %w", raw, err)
		}
		data = b
	}

	var specs []CompilerSpec
	if err := json.Unmarshal(data, &specs); err != nil {
		return nil, fmt.Errorf("parse HYPC_COMPILERS: %w", err)
	}
	for i := range specs {
		if specs[i].Runner == "" {
			return nil, fmt.Errorf("HYPC_COMPILERS entry %d missing required \"runner\"", i)
		}
		if specs[i].BuildID == "" {
			return nil, fmt.Errorf("HYPC_COMPILERS entry %d missing required \"buildId\"", i)
		}
	}
	return specs, nil
}

// legacyCompilerSpecs reproduces the single-build configuration from the
// pre-multi-version env vars. Returns nil when neither is set so the caller
// can report "no builds configured".
func legacyCompilerSpecs() []CompilerSpec {
	runner := os.Getenv("HYPC_RUNNER")
	buildID := os.Getenv("HYPC_BUILD_ID")
	if runner == "" || buildID == "" {
		return nil
	}
	return []CompilerSpec{{
		BuildID: buildID,
		NodeBin: envWithDefault("HYPC_NODE_BIN", "node"),
		Runner:  runner,
		Bin:     os.Getenv("HYPC_BIN"),
		Default: true,
	}}
}

// Resolve returns the compiler for the requested build id. An empty id
// selects the default build. The bool is false only when a non-empty id
// matches no configured build.
func (r *Registry) Resolve(buildID string) (*Compiler, bool) {
	if buildID == "" {
		return r.def, true
	}
	c, ok := r.byID[buildID]
	return c, ok
}

// Default returns the build used when a submission omits compilerVersion.
func (r *Registry) Default() *Compiler { return r.def }

// Info is the body of GET /contract/compiler-info: every selectable build
// plus which one is the default. The top-level Language/BuildID mirror the
// default build for backwards compatibility with single-build clients.
func (r *Registry) Info() CompilerInfoResponse {
	builds := make([]CompilerBuild, len(r.order))
	for i, c := range r.order {
		builds[i] = CompilerBuild{BuildID: c.BuildID, Language: c.Language, Default: c.Default}
	}
	return CompilerInfoResponse{
		Language:  r.def.Language,
		BuildID:   r.def.BuildID,
		Default:   r.def.BuildID,
		Compilers: builds,
	}
}

// SupportedBuildIDs returns every configured build id in display order.
// Used to populate the 400 response when a client requests an unknown build.
func (r *Registry) SupportedBuildIDs() []string {
	ids := make([]string, len(r.order))
	for i, c := range r.order {
		ids[i] = c.BuildID
	}
	return ids
}
