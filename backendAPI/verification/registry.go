package verification

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"time"
	"unicode"
)

// CompilerSpec is one entry in the HYPC_COMPILERS manifest. Enabled entries
// identify one pinned native hypc artifact. Legacy runner fields remain only
// for strict rejection and backward configuration diagnostics.
type CompilerSpec struct {
	Kind     string `json:"kind"`               // required: "native"; other kinds fail closed
	BuildID  string `json:"buildId"`            // required: version string the compiler must report
	Bin      string `json:"bin,omitempty"`      // native hypc path; copied into a sealed snapshot
	SHA256   string `json:"sha256,omitempty"`   // required for native; exact source artifact digest
	NodeBin  string `json:"nodeBin,omitempty"`  // legacy runner field; rejected for enabled native builds
	Runner   string `json:"runner,omitempty"`   // legacy runner field; rejected for enabled native builds
	Disabled bool   `json:"disabled,omitempty"` // retains unavailable historical identities without executing them
	Default  bool   `json:"default,omitempty"`  // exactly one enabled build must be the default
}

// Registry holds every enabled and probe-verified hypc build keyed by
// build id, plus the build to use when a request doesn't pick one. Exactly
// one build is always the default once construction succeeds.
type Registry struct {
	byID    map[string]*Compiler
	order   []*Compiler // stable display/iteration order (config order)
	def     *Compiler
	sandbox *sandboxLauncher
}

// NewRegistry builds the compiler registry from the environment.
//
// Preferred (multi-build): HYPC_COMPILERS holds either an inline JSON array
// of CompilerSpec, or a path to a file containing that array. File-backed
// manifests also require their exact digest in HYPC_COMPILERS_SHA256.
//
// Legacy (single-build, still supported): when HYPC_COMPILERS is unset, a
// one-entry native registry is synthesized from HYPC_BUILD_ID / HYPC_BIN /
// HYPC_SHA256. A legacy runner-only configuration is classified as npm and
// fails closed because its complete runtime provenance is unavailable.
//
// Shared limits (VERIFIER_MAX_CONCURRENCY / VERIFIER_COMPILE_TIMEOUT /
// VERIFIER_SOURCE_MAX_BYTES / VERIFIER_COMPILER_MAX_STDOUT_BYTES /
// VERIFIER_COMPILER_MAX_STDERR_BYTES) bound ALL builds together (one
// process-wide compile budget) regardless of which build a request selects.
// VERIFIER_SANDBOX_BIN and VERIFIER_SANDBOX_SHA256 identify the mandatory,
// fully static NsJail launcher shared by every enabled build.
//
// A non-default build whose sealed artifact cannot be invoked, reports the
// wrong version, or fails its binary hash check is skipped with a warning. The
// explicitly configured default must survive all provenance checks. This
// prevents an unavailable current compiler from silently promoting a
// historical build to the default.
func NewRegistry() (*Registry, error) {
	specs, err := loadCompilerSpecs()
	if err != nil {
		return nil, err
	}
	if len(specs) == 0 {
		return nil, errors.New("no hypc builds configured (set HYPC_COMPILERS, or HYPC_BUILD_ID+HYPC_BIN+HYPC_SHA256)")
	}
	for _, spec := range specs {
		if !spec.Disabled && spec.Kind != CompilerKindNative {
			return nil, fmt.Errorf(
				"compiler kind %q is unavailable without complete runtime provenance",
				spec.Kind,
			)
		}
	}

	maxConc := envInt("VERIFIER_MAX_CONCURRENCY", 2)
	if maxConc < 1 {
		maxConc = 1
	}
	timeout := envDuration("VERIFIER_COMPILE_TIMEOUT", 30*time.Second)
	maxStdin := normalizedPositiveLimit(
		envInt("VERIFIER_SOURCE_MAX_BYTES", defaultMaxStdinBytes),
		defaultMaxStdinBytes,
	)
	maxStdout := envInt("VERIFIER_COMPILER_MAX_STDOUT_BYTES", defaultMaxStdoutBytes)
	maxStderr := envInt("VERIFIER_COMPILER_MAX_STDERR_BYTES", defaultMaxStderrBytes)
	sem := make(chan struct{}, maxConc)

	sandbox, err := newSandboxLauncher(
		strings.TrimSpace(os.Getenv("VERIFIER_SANDBOX_BIN")),
		strings.TrimSpace(os.Getenv("VERIFIER_SANDBOX_SHA256")),
	)
	if err != nil {
		return nil, fmt.Errorf("verification sandbox unavailable: %w", err)
	}

	reg := &Registry{byID: map[string]*Compiler{}, sandbox: sandbox}
	ready := false
	defer func() {
		if !ready {
			reg.Close()
		}
	}()
	for _, spec := range specs {
		if spec.Disabled {
			log.Printf("verification.Registry: compiler build %q is retained as disabled", spec.BuildID)
			continue
		}
		c, err := newCompiler(spec, sandbox, sem, timeout, maxStdin, maxStdout, maxStderr)
		if err != nil {
			if spec.Default {
				return nil, fmt.Errorf("default hypc build %q is unusable: %w", spec.BuildID, err)
			}
			log.Printf("WARN: skipping hypc build %q: %v", spec.BuildID, err)
			continue
		}
		reg.byID[c.BuildID] = c
		reg.order = append(reg.order, c)
		if c.Default && reg.def == nil {
			reg.def = c
		}
	}

	if len(reg.order) == 0 {
		return nil, errors.New("no usable hypc builds (every enabled artifact failed provenance or version checks)")
	}

	// loadCompilerSpecs requires exactly one explicit default, and the loop
	// above fails registry construction if that build is unusable.
	if reg.def == nil {
		return nil, errors.New("configured default hypc build did not survive registry construction")
	}
	for _, c := range reg.order {
		c.Default = c == reg.def
	}

	log.Printf(
		"verification.Registry ready: %d build(s) %v, default=%s, sandbox=%s policy=%s, timeout=%s concurrency=%d stdout=%d stderr=%d",
		len(reg.order),
		reg.SupportedBuildIDs(),
		reg.def.BuildID,
		reg.sandbox.SHA256,
		reg.sandbox.PolicySHA256,
		timeout,
		maxConc,
		reg.def.MaxStdout,
		reg.def.MaxStderr,
	)
	ready = true
	return reg, nil
}

// loadCompilerSpecs reads the HYPC_COMPILERS manifest (inline JSON or a file
// path), falling back to the legacy single-build env vars when unset.
func loadCompilerSpecs() ([]CompilerSpec, error) {
	raw := strings.TrimSpace(os.Getenv("HYPC_COMPILERS"))
	manifestSHA256 := strings.TrimSpace(os.Getenv("HYPC_COMPILERS_SHA256"))
	if raw == "" {
		if manifestSHA256 != "" {
			return nil, errors.New("HYPC_COMPILERS_SHA256 requires a file-backed HYPC_COMPILERS manifest")
		}
		return legacyCompilerSpecs(), nil
	}

	data := []byte(raw)
	// Anything that doesn't start with a JSON array is treated as a path.
	if !strings.HasPrefix(raw, "[") {
		expectedSHA256, err := normalizeSHA256(manifestSHA256)
		if err != nil {
			return nil, fmt.Errorf("HYPC_COMPILERS_SHA256: %w", err)
		}
		if expectedSHA256 == "" {
			return nil, errors.New("HYPC_COMPILERS_SHA256 is required for a file-backed HYPC_COMPILERS manifest")
		}
		b, err := readCompilerManifest(raw, expectedSHA256)
		if err != nil {
			return nil, fmt.Errorf("read HYPC_COMPILERS file %q: %w", raw, err)
		}
		data = b
	} else if manifestSHA256 != "" {
		return nil, errors.New("HYPC_COMPILERS_SHA256 is only valid with a file-backed HYPC_COMPILERS manifest")
	}
	if err := rejectDuplicateJSONMembers(data); err != nil {
		return nil, fmt.Errorf("parse HYPC_COMPILERS: %w", err)
	}

	var specs []CompilerSpec
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&specs); err != nil {
		return nil, fmt.Errorf("parse HYPC_COMPILERS: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return nil, errors.New("parse HYPC_COMPILERS: trailing content after compiler array")
	}
	seenBuildIDs := make(map[string]struct{}, len(specs))
	for i := range specs {
		if specs[i].Kind == "" {
			return nil, fmt.Errorf("HYPC_COMPILERS entry %d missing required \"kind\"", i)
		}
		if specs[i].BuildID == "" {
			return nil, fmt.Errorf("HYPC_COMPILERS entry %d missing required \"buildId\"", i)
		}
		if _, duplicate := seenBuildIDs[specs[i].BuildID]; duplicate {
			return nil, fmt.Errorf("HYPC_COMPILERS contains duplicate buildId %q", specs[i].BuildID)
		}
		seenBuildIDs[specs[i].BuildID] = struct{}{}
		if specs[i].Disabled && specs[i].Default {
			return nil, fmt.Errorf("HYPC_COMPILERS build %q cannot be both disabled and default", specs[i].BuildID)
		}
	}
	if len(specs) > 0 {
		defaultCount := 0
		for i := range specs {
			if specs[i].Default {
				defaultCount++
			}
		}
		if defaultCount != 1 {
			return nil, fmt.Errorf("HYPC_COMPILERS must mark exactly one default build, got %d", defaultCount)
		}
	}
	return specs, nil
}

const maxCompilerManifestBytes int64 = 1 << 20

func readCompilerManifest(path, expectedSHA256 string) ([]byte, error) {
	f, openInfo, err := openSecureRegularFile(path, "manifest")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if err := validateManifestFileMode(openInfo); err != nil {
		return nil, err
	}
	if openInfo.Size() > maxCompilerManifestBytes {
		return nil, fmt.Errorf("manifest exceeds %d bytes", maxCompilerManifestBytes)
	}

	data, err := io.ReadAll(io.LimitReader(f, maxCompilerManifestBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxCompilerManifestBytes {
		return nil, fmt.Errorf("manifest exceeds %d bytes", maxCompilerManifestBytes)
	}
	afterInfo, err := f.Stat()
	if err != nil {
		return nil, err
	}
	if int64(len(data)) != openInfo.Size() || afterInfo.Size() != openInfo.Size() ||
		!afterInfo.ModTime().Equal(openInfo.ModTime()) {
		return nil, errors.New("manifest changed while reading")
	}
	digest := sha256.Sum256(data)
	actualSHA256 := hex.EncodeToString(digest[:])
	if actualSHA256 != expectedSHA256 {
		return nil, fmt.Errorf("manifest SHA-256 mismatch: want %s, got %s", expectedSHA256, actualSHA256)
	}
	return data, nil
}

func validateManifestFileMode(info os.FileInfo) error {
	if !info.Mode().IsRegular() {
		return errors.New("manifest must be a regular file")
	}
	if info.Mode().Perm()&0o022 != 0 {
		return errors.New("manifest must not be group or world writable")
	}
	return nil
}

func rejectDuplicateJSONMembers(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	if err := walkJSONValue(decoder); err != nil {
		return err
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		if err == nil {
			return errors.New("trailing JSON content")
		}
		return err
	}
	return nil
}

func walkJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := make(map[string]string)
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return errors.New("JSON object key is not a string")
			}
			foldedKey := foldJSONMemberName(key)
			if firstKey, duplicate := seen[foldedKey]; duplicate {
				return fmt.Errorf(
					"duplicate JSON member %q conflicts with %q under case-insensitive matching",
					key,
					firstKey,
				)
			}
			seen[foldedKey] = key
			if err := walkJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim('}') {
			return errors.New("unterminated JSON object")
		}
	case '[':
		for decoder.More() {
			if err := walkJSONValue(decoder); err != nil {
				return err
			}
		}
		end, err := decoder.Token()
		if err != nil {
			return err
		}
		if end != json.Delim(']') {
			return errors.New("unterminated JSON array")
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
	return nil
}

// encoding/json matches struct fields and JSON tags case-insensitively. Fold
// member names with the same Unicode simple-fold rule before duplicate checks
// so inputs such as "kind" plus "Kind" cannot overwrite an earlier value.
func foldJSONMemberName(name string) string {
	var folded strings.Builder
	folded.Grow(len(name))
	for _, r := range name {
		if 'a' <= r && r <= 'z' {
			r -= 'a' - 'A'
		} else if r > unicode.MaxASCII {
			r = foldJSONRune(r)
		}
		folded.WriteRune(r)
	}
	return folded.String()
}

func foldJSONRune(r rune) rune {
	for {
		next := unicode.SimpleFold(r)
		if next <= r {
			return next
		}
		r = next
	}
}

// legacyCompilerSpecs reproduces the single-build configuration from the
// pre-multi-version env vars. Returns nil when neither is set so the caller
// can report "no builds configured".
func legacyCompilerSpecs() []CompilerSpec {
	buildID := os.Getenv("HYPC_BUILD_ID")
	if buildID == "" {
		return nil
	}
	bin := os.Getenv("HYPC_BIN")
	if bin != "" {
		return []CompilerSpec{{
			Kind:    CompilerKindNative,
			BuildID: buildID,
			Bin:     bin,
			SHA256:  os.Getenv("HYPC_SHA256"),
			Default: true,
		}}
	}
	return []CompilerSpec{{
		Kind:    CompilerKindNPM,
		BuildID: buildID,
		NodeBin: os.Getenv("HYPC_NODE_BIN"),
		Runner:  os.Getenv("HYPC_RUNNER"),
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
		builds[i] = CompilerBuild{
			BuildID:    c.BuildID,
			Language:   c.Language,
			Kind:       c.Kind,
			SHA256:     c.SHA256,
			Provenance: c.ProvenanceRecord(),
			Default:    c.Default,
		}
	}
	return CompilerInfoResponse{
		Language:   r.def.Language,
		BuildID:    r.def.BuildID,
		Kind:       r.def.Kind,
		SHA256:     r.def.SHA256,
		Provenance: r.def.ProvenanceRecord(),
		Default:    r.def.BuildID,
		Compilers:  builds,
	}
}

// Close releases every sealed executable snapshot owned by the registry.
// It is idempotent and waits for in-flight compiler subprocesses to finish.
func (r *Registry) Close() error {
	var firstErr error
	for _, c := range r.order {
		if err := c.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if r.sandbox != nil {
		if err := r.sandbox.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
		r.sandbox = nil
	}
	return firstErr
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
