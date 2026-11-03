package verification

import (
	"encoding/json"

	"backendAPI/models"
)

// VerifyRequest is the JSON body for POST /contract/verify.
//
// The primary contract source goes in `sourceCode`. By default it is compiled
// as the source unit `<contractName>.hyp` at the root of the virtual source
// tree. A project laid out in directories sets `primarySourcePath` to the
// repo-relative path the deployer compiled (e.g. "contracts/core/Token.hyp").
// Every other source goes into the `imports` map keyed by its repo-relative
// path in the same tree (e.g. "contracts/lib/Math.hyp"), so a relative import
// such as "../lib/Math.hyp" resolves exactly as it did for the deployer. The
// Go layer wraps all of them into the Hyperion standard-JSON `sources` map,
// which is the compiler's whole file system: nothing is written to disk.
type VerifyRequest struct {
	Address              string            `json:"address" binding:"required"`
	SourceCode           string            `json:"sourceCode" binding:"required"`
	ContractName         string            `json:"contractName" binding:"required"`
	CompilerVersion      string            `json:"compilerVersion,omitempty"`
	OptimizerEnabled     bool              `json:"optimizerEnabled"`
	OptimizerRuns        int               `json:"optimizerRuns"`
	EvmVersion           string            `json:"evmVersion,omitempty"`
	ConstructorArguments string            `json:"constructorArguments,omitempty"`
	Libraries            map[string]string `json:"libraries,omitempty"`
	Imports              map[string]string `json:"imports,omitempty"`
	PrimarySourcePath    string            `json:"primarySourcePath,omitempty"`
	ViaIR                bool              `json:"viaIR,omitempty"`
	License              string            `json:"license,omitempty"`
}

// VerifyEnqueueResponse is returned synchronously from POST /contract/verify
// once the job has been created. Clients poll GET /contract/verify/:jobId
// for the real outcome.
type VerifyEnqueueResponse struct {
	JobID   string `json:"jobId"`
	Status  string `json:"status"`
	Address string `json:"address"`
}

// CompilerInfoResponse is the body of GET /contract/compiler-info: every
// hypc build the backend can verify against, plus which one is used when a
// submission omits compilerVersion. The top-level Language/BuildID mirror
// the default build so single-build clients (which only read those two
// fields) keep working unchanged.
type CompilerInfoResponse struct {
	Language   string                     `json:"language"`
	BuildID    string                     `json:"buildId"`
	Kind       string                     `json:"kind"`
	SHA256     string                     `json:"sha256"`
	Provenance *models.CompilerProvenance `json:"provenance"`
	Default    string                     `json:"default"`
	Compilers  []CompilerBuild            `json:"compilers"`
}

// CompilerBuild describes one selectable compiler build in the
// /contract/compiler-info response.
type CompilerBuild struct {
	BuildID    string                     `json:"buildId"`
	Language   string                     `json:"language"`
	Kind       string                     `json:"kind"`
	SHA256     string                     `json:"sha256"`
	Provenance *models.CompilerProvenance `json:"provenance"`
	Default    bool                       `json:"default"`
}

// StandardJSONInput is the Hyperion standard-JSON shape we feed to the
// compiler. Mirrors the Solidity standard-JSON layout, see
// theQRL/hyperion docs for details.
type StandardJSONInput struct {
	Language string                        `json:"language"`
	Sources  map[string]StandardJSONSource `json:"sources"`
	Settings StandardJSONSettings          `json:"settings"`
}

type StandardJSONSource struct {
	Content string `json:"content"`
}

type StandardJSONSettings struct {
	Optimizer  *Optimizer                   `json:"optimizer,omitempty"`
	EVMVersion string                       `json:"evmVersion,omitempty"`
	Libraries  map[string]map[string]string `json:"libraries,omitempty"`
	// ViaIR is omitted when false so default inputs stay byte-identical to
	// the payloads the verifier produced before via-IR support.
	ViaIR           bool                           `json:"viaIR,omitempty"`
	OutputSelection map[string]map[string][]string `json:"outputSelection"`
}

type Optimizer struct {
	Enabled bool `json:"enabled"`
	Runs    int  `json:"runs"`
}

// StandardJSONOutput is the compiler's JSON output shape.
type StandardJSONOutput struct {
	Errors    []CompilerError                        `json:"errors,omitempty"`
	Contracts map[string]map[string]CompiledContract `json:"contracts,omitempty"`
}

type CompilerError struct {
	Severity         string `json:"severity"`
	Message          string `json:"message"`
	FormattedMessage string `json:"formattedMessage,omitempty"`
	Type             string `json:"type,omitempty"`
}

// CompiledContract is the per-contract artifact from the compiler's output
// for a given source unit. The bytecode is published under different
// top-level keys depending on the hypc version:
//
//	hypc 0.0.2  → `zvm`  (the legacy Zond VM naming)
//	hypc 0.2.x+ → `qrvm` (post-fork QRL VM rename)
//
// Both shapes are parsed; the verifier picks whichever is populated via
// `CompiledContract.DeployedBytecode()` so downstream code doesn't have
// to branch on compiler version.
type CompiledContract struct {
	ABI      json.RawMessage `json:"abi,omitempty"`
	ZVM      vmArtifacts     `json:"zvm,omitempty"`
	QRVM     vmArtifacts     `json:"qrvm,omitempty"`
	Metadata string          `json:"metadata,omitempty"`
}

type vmArtifacts struct {
	DeployedBytecode deployedBytecodeArtifact `json:"deployedBytecode"`
	Bytecode         struct {
		Object string `json:"object"`
	} `json:"bytecode"`
}

type deployedBytecodeArtifact struct {
	Object              string                      `json:"object"`
	ImmutableReferences map[string][]ImmutableRange `json:"immutableReferences,omitempty"`
}

// DeployedBytecode returns the populated deployed-bytecode artifact,
// preferring qrvm (modern) over zvm (legacy 0.0.2).
func (c *CompiledContract) DeployedBytecode() deployedBytecodeArtifact {
	if c.QRVM.DeployedBytecode.Object != "" {
		return c.QRVM.DeployedBytecode
	}
	return c.ZVM.DeployedBytecode
}

// ImmutableRange is a byte slice within deployedBytecode that holds an
// `immutable` value patched at deploy time. We mask these ranges in both
// the compiled output and the on-chain code before byte-matching.
type ImmutableRange struct {
	Start  int `json:"start"`
	Length int `json:"length"`
}

// FatalErrors filters a compiler's output for errors that should fail the
// verification (severity == "error"). Warnings are surfaced but not fatal.
func (o *StandardJSONOutput) FatalErrors() []CompilerError {
	out := []CompilerError{}
	for _, e := range o.Errors {
		if e.Severity == "error" {
			out = append(out, e)
		}
	}
	return out
}
