package verification

import "encoding/json"

// VerifyRequest is the JSON body for POST /contract/verify.
//
// Single-file submissions wrap the primary contract source in the
// `sourceCode` field; additional dependencies (imports) go into the
// `imports` map keyed by the path the source uses for the import (e.g.
// "./Context.hyp" → the Context.hyp content). The Go layer wraps both
// into a Hyperion standard-JSON `sources` map before invoking the
// runner, users never see standard-JSON directly in v1.
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

// CompilerInfoResponse is the body of GET /contract/compiler-info, the
// single pinned hypc build the backend is willing to verify against.
type CompilerInfoResponse struct {
	Language string `json:"language"`
	BuildID  string `json:"buildId"`
}

// StandardJSONInput is the Hyperion standard-JSON shape we feed to the
// runner. Mirrors the Solidity standard-JSON layout, see
// theQRL/hyperion docs for details.
type StandardJSONInput struct {
	Language string                              `json:"language"`
	Sources  map[string]StandardJSONSource       `json:"sources"`
	Settings StandardJSONSettings                `json:"settings"`
}

type StandardJSONSource struct {
	Content string `json:"content"`
}

type StandardJSONSettings struct {
	Optimizer       *Optimizer                              `json:"optimizer,omitempty"`
	EVMVersion      string                                  `json:"evmVersion,omitempty"`
	Libraries       map[string]map[string]string            `json:"libraries,omitempty"`
	OutputSelection map[string]map[string][]string          `json:"outputSelection"`
}

type Optimizer struct {
	Enabled bool `json:"enabled"`
	Runs    int  `json:"runs"`
}

// StandardJSONOutput is the runner's JSON output shape.
type StandardJSONOutput struct {
	Errors    []CompilerError                            `json:"errors,omitempty"`
	Contracts map[string]map[string]CompiledContract     `json:"contracts,omitempty"`
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
//   hypc 0.0.2  → `zvm`  (the legacy Zond VM naming)
//   hypc 0.2.x+ → `qrvm` (post-fork QRL VM rename)
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

// FatalErrors filters a runner's output for errors that should fail the
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
