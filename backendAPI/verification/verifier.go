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

	"go.mongodb.org/mongo-driver/bson"
)

// Verifier ties Compiler + on-chain code + byte match + the M1 storage
// layer (MarkContractVerified + UpdateVerificationJob) into a single
// background runner. One Verifier per process.
type Verifier struct {
	Compiler *Compiler
}

// RunAsync runs the verification flow in the background and updates the
// job document along the way. Returns immediately so the HTTP handler
// can hand the jobId back to the client without blocking.
//
// Status transitions:
//
//	pending → compiling → success    (mark contract verified, store ResultRef)
//	pending → compiling → failed     (record error message)
//
// The supplied ctx should typically be context.Background() so the job
// survives the client connection being torn down mid-compile.
func (v *Verifier) RunAsync(jobID string, req VerifyRequest) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), v.Compiler.Timeout+15*time.Second)
		defer cancel()
		v.run(ctx, jobID, req)
	}()
}

func (v *Verifier) run(ctx context.Context, jobID string, req VerifyRequest) {
	if err := db.UpdateVerificationJob(jobID, bson.M{"status": models.VerificationJobCompiling}); err != nil {
		// Status transition failures aren't fatal — the rest of the run
		// continues and the client will see the eventual terminal state.
		// But we log so persistent mongo issues don't go unnoticed.
		log.Printf("verifier: failed to mark job %s compiling: %v", jobID, err)
	}

	input := wrapStandardJSON(req)

	out, err := v.Compiler.Compile(ctx, input)
	if err != nil {
		failJob(jobID, fmt.Sprintf("compile invocation failed: %s", err.Error()))
		return
	}
	if fatal := out.FatalErrors(); len(fatal) > 0 {
		failJob(jobID, formatCompileErrors(fatal))
		return
	}

	contract, ok := findContract(out, req.ContractName)
	if !ok {
		failJob(jobID, fmt.Sprintf("contract %q not found in compiler output", req.ContractName))
		return
	}

	onchain, err := FetchOnChainCode(ctx, req.Address)
	if err != nil {
		failJob(jobID, fmt.Sprintf("fetch on-chain code: %s", err.Error()))
		return
	}

	dbc := contract.DeployedBytecode()
	match, err := Match(dbc.Object, onchain, dbc.ImmutableReferences)
	if err != nil {
		failJob(jobID, fmt.Sprintf("match: %s", err.Error()))
		return
	}
	if match.NotFound {
		failJob(jobID, "no contract code at address (qrl_getCode returned empty)")
		return
	}
	if !match.Matched {
		failJob(jobID, fmt.Sprintf("bytecode mismatch (compiled=%d B, on-chain=%d B, diff@byte=%d)",
			match.CompiledLen, match.OnChainLen, match.DiffByteOffset))
		return
	}

	abiBytes, _ := json.Marshal(contract.ABI)
	result := models.VerificationResult{
		SourceCode:           req.SourceCode,
		Abi:                  string(abiBytes),
		ContractName:         req.ContractName,
		CompilerVersion:      v.Compiler.BuildID,
		OptimizationEnabled:  req.OptimizerEnabled,
		OptimizationRuns:     req.OptimizerRuns,
		EvmVersion:           req.EvmVersion,
		ConstructorArguments: req.ConstructorArguments,
		Libraries:            req.Libraries,
		License:              req.License,
		VerificationMethod:   "full-source",
	}
	if err := db.MarkContractVerified(req.Address, result); err != nil {
		failJob(jobID, fmt.Sprintf("write verification: %s", err.Error()))
		return
	}

	if err := db.UpdateVerificationJob(jobID, bson.M{
		"status": models.VerificationJobSuccess,
		"error":  "",
		"result": models.VerificationJobResultRef{
			Abi:        string(abiBytes),
			VerifiedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}); err != nil {
		log.Printf("verifier: failed to mark job %s success: %v", jobID, err)
	}
}

func failJob(jobID, msg string) {
	if err := db.UpdateVerificationJob(jobID, bson.M{
		"status": models.VerificationJobFailed,
		"error":  msg,
	}); err != nil {
		log.Printf("verifier: failed to mark job %s failed (%q): %v", jobID, msg, err)
	}
}

// wrapStandardJSON builds the Hyperion standard-JSON input from the user-
// submitted single-file request. The primary source is keyed by
// `<ContractName>.hyp`; any caller-supplied imports are passed through
// verbatim so the runner's import resolver can find them.
func wrapStandardJSON(req VerifyRequest) StandardJSONInput {
	sources := map[string]StandardJSONSource{
		req.ContractName + ".hyp": {Content: req.SourceCode},
	}
	for path, content := range req.Imports {
		sources[normalizeImportPath(path)] = StandardJSONSource{Content: content}
	}

	// hypc 0.2.x treats "no optimizer block" and "optimizer: {enabled:false}"
	// as different inputs — the latter still applies a baseline optimization
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
	}
}

// normalizeImportPath strips a leading "./" from a user-supplied import
// key so it lines up with whatever Hyperion's resolver looks up. Users
// who literally write `import "./Context.hyp"` in their source might
// reasonably key the imports map either way; we accept both.
func normalizeImportPath(p string) string {
	if strings.HasPrefix(p, "./") {
		return p[2:]
	}
	return p
}

// findContract walks the compiler output for a contract with the supplied
// name. Hypc nests contracts under <sourceUnit>.<contractName> so we scan
// every source unit for a matching name.
func findContract(out *StandardJSONOutput, name string) (*CompiledContract, bool) {
	for _, unit := range out.Contracts {
		if c, ok := unit[name]; ok {
			return &c, true
		}
	}
	return nil, false
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

// AlreadyVerified returns true when the contract document at `address`
// already carries a verified=true marker, so duplicate submissions can
// be made idempotent.
func AlreadyVerified(address string) (bool, error) {
	c, err := db.ReturnContractCode(address)
	if err != nil {
		return false, err
	}
	return c.Verified, nil
}

// HasPendingJob returns true when a job for `address` is currently
// pending or compiling. Used to reject duplicate submissions.
func HasPendingJob(address string) (bool, error) {
	// Walk through (limited) — only a tiny number of jobs per contract
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
// verified — callers should treat this as success (idempotency).
var ErrAlreadyVerified = errors.New("verification: address already verified")

// ErrDuplicateJob is returned by Enqueue when a pending job already exists
// for the same address.
var ErrDuplicateJob = errors.New("verification: another job is already in-flight for this address")
