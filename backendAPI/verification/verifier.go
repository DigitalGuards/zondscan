package verification

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"backendAPI/db"
	"backendAPI/models"
	"backendAPI/sourcebundle"
)

// Verifier ties the compiler Registry + on-chain code + byte match + the M1
// storage layer (MarkContractVerified + UpdateVerificationJob) into a single
// background runner. One Verifier per process.
type Verifier struct {
	Registry *Registry
}

// RunAsync runs the verification flow in the background and updates the
// job document along the way. Returns immediately so the HTTP handler
// can hand the jobId back to the client without blocking.
//
// req.CompilerVersion selects which configured build to compile with; an
// empty value uses the registry default. (The routes layer already validated
// it, so an unknown id here only happens for a direct/internal caller; we
// fail the job rather than leave it stuck in 'pending'.)
//
// Status transitions:
//
//	pending → compiling → success    (mark contract verified, store ResultRef)
//	pending → compiling → failed     (record error message)
//
// The supplied ctx should typically be context.Background() so the job
// survives the client connection being torn down mid-compile.
func (v *Verifier) RunAsync(jobID string, req VerifyRequest, target models.VerificationTarget) {
	canonicalReq, err := CanonicalizeVerifyRequest(req)
	if err != nil {
		failJob(jobID, target, fmt.Sprintf("invalid verification request: %s", err.Error()))
		return
	}
	req = canonicalReq

	comp, ok := v.Registry.Resolve(req.CompilerVersion)
	if !ok {
		failJob(jobID, target, fmt.Sprintf("unsupported compilerVersion %q", req.CompilerVersion))
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), comp.Timeout+15*time.Second)
		defer cancel()
		v.run(ctx, jobID, req, target, comp)
	}()
}

func (v *Verifier) run(
	ctx context.Context,
	jobID string,
	req VerifyRequest,
	target models.VerificationTarget,
	comp *Compiler,
) {
	if err := db.ClaimVerificationJob(jobID, target); err != nil {
		log.Printf("verifier: job %s lost its verification fence before compile: %v", jobID, err)
		return
	}

	input, err := wrapStandardJSON(req)
	if err != nil {
		failJob(jobID, target, fmt.Sprintf("invalid verification request: %s", err.Error()))
		return
	}

	out, err := comp.Compile(ctx, input)
	if err != nil {
		failJob(jobID, target, fmt.Sprintf("compile invocation failed: %s", err.Error()))
		return
	}
	if fatal := out.FatalErrors(); len(fatal) > 0 {
		failJob(jobID, target, formatCompileErrors(fatal))
		return
	}

	primarySource := primarySourcePath(req.ContractName)
	contract, ok := findContract(out, primarySource, req.ContractName)
	if !ok {
		failJob(jobID, target, fmt.Sprintf(
			"contract %q not found in primary compiler source %q",
			req.ContractName,
			primarySource,
		))
		return
	}

	onchain, err := FetchOnChainCode(ctx, req.Address)
	if err != nil {
		failJob(jobID, target, fmt.Sprintf("fetch on-chain code: %s", err.Error()))
		return
	}
	onchainDigest, err := RuntimeCodeSHA256(onchain)
	if err != nil || onchainDigest != target.DeployedCodeSHA256 {
		failJob(jobID, target, "verification target changed before bytecode match")
		return
	}

	dbc := contract.DeployedBytecode()
	match, err := Match(dbc.Object, onchain, dbc.ImmutableReferences)
	if err != nil {
		failJob(jobID, target, fmt.Sprintf("match: %s", err.Error()))
		return
	}
	if match.NotFound {
		failJob(jobID, target, "no contract code at address (qrl_getCode returned empty)")
		return
	}
	if !match.Matched {
		failJob(jobID, target, fmt.Sprintf("bytecode mismatch (compiled=%d B, on-chain=%d B, diff@byte=%d)",
			match.CompiledLen, match.OnChainLen, match.DiffByteOffset))
		return
	}

	abiBytes, _ := json.Marshal(contract.ABI)
	result := models.VerificationResult{
		SourceCode:           req.SourceCode,
		Abi:                  string(abiBytes),
		ContractName:         req.ContractName,
		CompilerVersion:      comp.BuildID,
		CompilerProvenance:   comp.ProvenanceRecord(),
		OptimizationEnabled:  req.OptimizerEnabled,
		OptimizationRuns:     req.OptimizerRuns,
		EvmVersion:           req.EvmVersion,
		ConstructorArguments: req.ConstructorArguments,
		Libraries:            req.Libraries,
		Imports:              req.Imports,
		License:              req.License,
		VerificationMethod:   "full-source",
	}
	if err := RevalidateVerificationTarget(ctx, target); err != nil {
		failJob(jobID, target, fmt.Sprintf("revalidate verification target: %s", err.Error()))
		return
	}
	if _, err := db.MarkContractVerified(jobID, target, result); err != nil {
		failJob(jobID, target, fmt.Sprintf("write verification: %s", err.Error()))
	}
}

func failJob(jobID string, target models.VerificationTarget, msg string) {
	if err := db.FailVerificationJob(jobID, target, msg); err != nil &&
		!errors.Is(err, db.ErrVerificationTargetChanged) {
		log.Printf("verifier: failed to mark job %s failed (%q): %v", jobID, msg, err)
	}
}

// wrapStandardJSON builds the Hyperion standard-JSON input from the user-
// submitted single-file request. The primary source is keyed by
// `<ContractName>.hyp`; canonical imports retain their exact source content
// under the same clean paths persisted with the verification record.
func wrapStandardJSON(req VerifyRequest) (StandardJSONInput, error) {
	canonicalReq, err := CanonicalizeVerifyRequest(req)
	if err != nil {
		return StandardJSONInput{}, err
	}
	req = canonicalReq

	sources := map[string]StandardJSONSource{
		primarySourcePath(req.ContractName): {Content: req.SourceCode},
	}
	for path, content := range req.Imports {
		sources[path] = StandardJSONSource{Content: content}
	}

	// hypc 0.2.x treats "no optimizer block" and "optimizer: {enabled:false}"
	// as different inputs, the latter still applies a baseline optimization
	// pass that shrinks output by ~100 bytes. Since the standard CLI flow
	// (`hypc --bin` with no flags) omits the optimizer entirely, we mirror
	// that for the "optimizer off" path. Explicit enable still includes the
	// block with the requested runs.
	settings := StandardJSONSettings{
		EVMVersion: req.EvmVersion,
		OutputSelection: map[string]map[string][]string{
			"*": {
				"*": {
					"abi",
					// Request both naming conventions so we work against
					// hypc 0.0.2 (zvm) and 0.2.x+ (qrvm). Unknown keys are
					// ignored by the compiler.
					"zvm.deployedBytecode.object",
					"zvm.deployedBytecode.immutableReferences",
					"qrvm.deployedBytecode.object",
					"qrvm.deployedBytecode.immutableReferences",
					"metadata",
				},
			},
		},
	}
	if req.OptimizerEnabled {
		runs := req.OptimizerRuns
		if runs <= 0 {
			runs = 200
		}
		settings.Optimizer = &Optimizer{Enabled: true, Runs: runs}
	}
	if len(req.Libraries) > 0 {
		// Libraries map is keyed by file → name → address. Best-effort:
		// hoist into a single sentinel "<ContractName>.hyp" entry so users
		// don't have to know the multi-file shape.
		settings.Libraries = map[string]map[string]string{
			req.ContractName + ".hyp": req.Libraries,
		}
	}

	return StandardJSONInput{
		Language: "Hyperion",
		Sources:  sources,
		Settings: settings,
	}, nil
}

// findContract selects a contract only from the named primary source unit.
// Imported units may declare the same contract name, so a cross-unit scan
// would make source selection depend on randomized Go map iteration.
func findContract(out *StandardJSONOutput, sourceUnit, name string) (*CompiledContract, bool) {
	unit, ok := out.Contracts[sourceUnit]
	if !ok {
		return nil, false
	}
	contract, ok := unit[name]
	if !ok {
		return nil, false
	}
	return &contract, true
}

func formatCompileErrors(es []CompilerError) string {
	parts := []string{}
	for _, e := range es {
		if e.FormattedMessage != "" {
			parts = append(parts, e.FormattedMessage)
		} else {
			parts = append(parts, e.Message)
		}
	}
	msg := strings.Join(parts, "\n---\n")
	if len(msg) > 8000 {
		msg = msg[:8000] + "…(truncated)"
	}
	return msg
}

// AlreadyVerified returns true for valid digest-backed and legacy verified
// records so duplicate submissions remain idempotent. Invalid recorded
// provenance requires controlled operator repair and is never overwritten by
// the public submission path.
func AlreadyVerified(address string) (bool, error) {
	c, err := db.ReturnContractCode(address)
	if err != nil {
		return false, err
	}
	if !c.Verified {
		return false, nil
	}
	if sourcebundle.ClassifyStoredVerification(c) == models.CompilerProvenanceInvalidRecorded {
		return false, ErrInvalidStoredVerification
	}
	return true, nil
}

// HasPendingJob returns true when a job for `address` is currently
// pending or compiling. Used to reject duplicate submissions.
func HasPendingJob(address string) (bool, error) {
	// Walk through (limited), only a tiny number of jobs per contract
	// are expected in practice.
	for _, status := range []models.VerificationJobStatus{models.VerificationJobPending, models.VerificationJobCompiling} {
		jobs, err := db.FindVerificationJobsByAddress(address, status, 1)
		if err != nil {
			return false, err
		}
		if len(jobs) > 0 {
			return true, nil
		}
	}
	return false, nil
}

// ErrAlreadyVerified is returned by Enqueue when the contract is already
// verified, callers should treat this as success (idempotency).
var ErrAlreadyVerified = errors.New("verification: address already verified")

// ErrInvalidStoredVerification protects write-once verification records from
// unauthenticated replacement when their persisted provenance is malformed.
var ErrInvalidStoredVerification = errors.New("stored verification record is invalid; operator repair required")

// ErrDuplicateJob is returned by Enqueue when a pending job already exists
// for the same address.
var ErrDuplicateJob = errors.New("verification: another job is already in-flight for this address")
