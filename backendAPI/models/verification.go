package models

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"hash"
	"sort"
	"strconv"
	"strings"
)

// VerificationJobStatus is one of the discrete states a contract-verification
// async job can be in.
type VerificationJobStatus string

const (
	VerificationJobPending   VerificationJobStatus = "pending"
	VerificationJobCompiling VerificationJobStatus = "compiling"
	VerificationJobSuccess   VerificationJobStatus = "success"
	VerificationJobFailed    VerificationJobStatus = "failed"
)

// ContractVerificationJob is the async-job-tracking document persisted to
// the `contractVerifications` collection. One job per /contract/verify
// submission; the result of a successful job is also written into the
// contract's verification fields via MarkContractVerified.
//
// `Payload` echoes the user-submitted request so we can re-run / debug
// without keeping a separate audit log.
type ContractVerificationJob struct {
	JobID     string                    `bson:"jobId" json:"jobId"`
	Address   string                    `bson:"address" json:"address"`
	Status    VerificationJobStatus     `bson:"status" json:"status"`
	Error     string                    `bson:"error,omitempty" json:"error,omitempty"`
	CreatedAt string                    `bson:"createdAt" json:"createdAt"`
	UpdatedAt string                    `bson:"updatedAt" json:"updatedAt"`
	Payload   VerificationJobPayload    `bson:"payload" json:"payload"`
	Target    VerificationTarget        `bson:"target" json:"target"`
	Result    *VerificationJobResultRef `bson:"result,omitempty" json:"result,omitempty"`
}

// VerificationTarget binds an async verification job to one canonical
// deployment generation. A worker may publish only while the contract row,
// canonical creation block, live chain, and runtime-code digest still match
// every field captured before compilation began.
type VerificationTarget struct {
	Address             string `bson:"address" json:"address"`
	CreationTransaction string `bson:"creationTransaction,omitempty" json:"creationTransaction,omitempty"`
	CreationBlockNumber string `bson:"creationBlockNumber" json:"creationBlockNumber"`
	CreationBlockHash   string `bson:"creationBlockHash" json:"creationBlockHash"`
	ChainID             string `bson:"chainId" json:"chainId"`
	DeployedCodeSHA256  string `bson:"deployedCodeSha256" json:"deployedCodeSha256"`
	GenesisContract     bool   `bson:"genesisContract" json:"genesisContract"`
}

// VerificationJobPayload mirrors the inputs accepted by the /contract/verify
// endpoint. Kept as its own type so the job document and the request body
// can share a schema.
type VerificationJobPayload struct {
	SourceCode           string              `bson:"sourceCode,omitempty" json:"sourceCode,omitempty"`
	ContractName         string              `bson:"contractName,omitempty" json:"contractName,omitempty"`
	CompilerVersion      string              `bson:"compilerVersion,omitempty" json:"compilerVersion,omitempty"`
	CompilerProvenance   *CompilerProvenance `bson:"compilerProvenance,omitempty" json:"compilerProvenance,omitempty"`
	OptimizationEnabled  bool                `bson:"optimizationEnabled" json:"optimizationEnabled"`
	OptimizationRuns     int                 `bson:"optimizationRuns" json:"optimizationRuns"`
	EvmVersion           string              `bson:"evmVersion,omitempty" json:"evmVersion,omitempty"`
	ConstructorArguments string              `bson:"constructorArguments,omitempty" json:"constructorArguments,omitempty"`
	Libraries            map[string]string   `bson:"libraries,omitempty" json:"libraries,omitempty"`
	Imports              map[string]string   `bson:"imports,omitempty" json:"imports,omitempty"`
	License              string              `bson:"license,omitempty" json:"license,omitempty"`
	VerificationMethod   string              `bson:"verificationMethod,omitempty" json:"verificationMethod,omitempty"`
}

// VerificationJobResultRef is the small handle written back into the job
// after a successful verification, the full materialised result lives on
// the contract document itself (verified=true + abi + …). This struct
// exists so /contract/verify/:jobId can return a useful payload without
// re-querying the contracts collection.
type VerificationJobResultRef struct {
	Abi                string              `bson:"abi,omitempty" json:"abi,omitempty"`
	BytecodeHash       string              `bson:"bytecodeHash,omitempty" json:"bytecodeHash,omitempty"`
	ArtifactDigest     string              `bson:"artifactDigest,omitempty" json:"artifactDigest,omitempty"`
	CompilerProvenance *CompilerProvenance `bson:"compilerProvenance,omitempty" json:"compilerProvenance,omitempty"`
	VerifiedAt         string              `bson:"verifiedAt,omitempty" json:"verifiedAt,omitempty"`
}

const verificationArtifactDigestPrefixV2 = "qrl.contract-verification-artifact.v2:sha256:"

func IsVerificationArtifactDigestV2(value string) bool {
	if !strings.HasPrefix(value, verificationArtifactDigestPrefixV2) {
		return false
	}
	digest := strings.TrimPrefix(value, verificationArtifactDigestPrefixV2)
	if len(digest) != sha256.Size*2 || digest != strings.ToLower(digest) {
		return false
	}
	decoded, err := hex.DecodeString(digest)
	return err == nil && len(decoded) == sha256.Size
}

// VerificationArtifactDigestV2 binds the complete published verification
// artifact to the exact canonical deployment target. Every variable-width
// field is uint64 length-prefixed and maps are sorted by key.
func VerificationArtifactDigestV2(
	target VerificationTarget,
	result VerificationResult,
	sourceBundleDigest string,
) string {
	h := sha256.New()
	for _, value := range []string{
		VerificationRecordSchemaV2,
		target.Address,
		target.CreationTransaction,
		target.CreationBlockNumber,
		target.CreationBlockHash,
		target.ChainID,
		target.DeployedCodeSHA256,
		strconv.FormatBool(target.GenesisContract),
		sourceBundleDigest,
		result.Abi,
		result.ContractName,
		result.CompilerVersion,
	} {
		writeCompilerProvenanceField(h, value)
	}
	if result.CompilerProvenance == nil {
		writeCompilerProvenanceUint64(h, 0)
	} else {
		writeCompilerProvenanceUint64(h, 1)
		for _, value := range []string{
			result.CompilerProvenance.Schema,
			result.CompilerProvenance.Kind,
			result.CompilerProvenance.BuildID,
			result.CompilerProvenance.ExecutionDigest,
		} {
			writeCompilerProvenanceField(h, value)
		}
		writeCompilerProvenanceUint64(h, uint64(len(result.CompilerProvenance.Components)))
		for _, component := range result.CompilerProvenance.Components {
			writeCompilerProvenanceField(h, component.Name)
			writeCompilerProvenanceField(h, component.SHA256)
		}
	}
	for _, value := range []string{
		strconv.FormatBool(result.OptimizationEnabled),
		strconv.Itoa(result.OptimizationRuns),
		result.EvmVersion,
		result.ConstructorArguments,
		result.License,
		result.VerificationMethod,
	} {
		writeCompilerProvenanceField(h, value)
	}
	writeSortedStringMap(h, result.Libraries)
	return verificationArtifactDigestPrefixV2 + hex.EncodeToString(h.Sum(nil))
}

func writeSortedStringMap(h hash.Hash, values map[string]string) {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	writeCompilerProvenanceUint64(h, uint64(len(keys)))
	for _, key := range keys {
		writeCompilerProvenanceField(h, key)
		writeCompilerProvenanceField(h, values[key])
	}
}

// CompilerProvenance identifies the exact executable artifact used for one
// verification. Absent provenance on older records means legacy-unrecorded;
// callers must not infer it from the registry's current contents.
type CompilerProvenance struct {
	Schema          string                        `bson:"schema" json:"schema"`
	Kind            string                        `bson:"kind" json:"kind"`
	BuildID         string                        `bson:"buildId" json:"buildId"`
	ExecutionDigest string                        `bson:"executionDigest" json:"executionDigest"`
	Components      []CompilerProvenanceComponent `bson:"components" json:"components"`
}

type CompilerProvenanceComponent struct {
	Name   string `bson:"name" json:"name"`
	SHA256 string `bson:"sha256" json:"sha256"`
}

const (
	CompilerProvenanceSchemaV1 = "qrl.contract-compiler-provenance.v1"
	CompilerProvenanceSchemaV2 = "qrl.contract-compiler-provenance.v2"
)

type CompilerProvenanceStatus string

const (
	CompilerProvenanceDigestBacked     CompilerProvenanceStatus = "digest-backed"
	CompilerProvenanceLegacyUnrecorded CompilerProvenanceStatus = "legacy-unrecorded"
	CompilerProvenanceInvalidRecorded  CompilerProvenanceStatus = "invalid-recorded"
)

// ClassifyCompilerProvenance validates only persisted fields. It never
// consults the current compiler registry or infers an artifact identity for
// historical records. A nil pointer represents an omitted legacy field.
func ClassifyCompilerProvenance(
	provenance *CompilerProvenance,
	compilerVersion string,
) CompilerProvenanceStatus {
	if provenance == nil {
		return CompilerProvenanceLegacyUnrecorded
	}
	if provenance.Schema != CompilerProvenanceSchemaV2 ||
		provenance.Kind != "native" ||
		provenance.BuildID == "" ||
		compilerVersion == "" ||
		provenance.BuildID != compilerVersion ||
		len(provenance.Components) != 3 {
		return CompilerProvenanceInvalidRecorded
	}
	if provenance.Components[0].Name != "hypc" ||
		provenance.Components[1].Name != "nsjail" ||
		provenance.Components[2].Name != "policy" {
		return CompilerProvenanceInvalidRecorded
	}
	for _, component := range provenance.Components {
		if !isLowerSHA256(component.SHA256) {
			return CompilerProvenanceInvalidRecorded
		}
	}
	if !isLowerSHA256(provenance.ExecutionDigest) ||
		provenance.ExecutionDigest != NativeSandboxCompilerExecutionDigestV2(
			provenance.BuildID,
			provenance.Components[0].SHA256,
			provenance.Components[1].SHA256,
			provenance.Components[2].SHA256,
		) {
		return CompilerProvenanceInvalidRecorded
	}
	return CompilerProvenanceDigestBacked
}

// NativeCompilerExecutionDigestV1 binds every schema-v1 identity field using
// the same canonical bytes written into verification records.
func NativeCompilerExecutionDigestV1(buildID, hypcSHA256 string) string {
	canonical := CompilerProvenanceSchemaV1 + "\n" +
		"kind=native\n" +
		"buildId=" + buildID + "\n" +
		"hypc.sha256=" + hypcSHA256 + "\n"
	digest := sha256.Sum256([]byte(canonical))
	return hex.EncodeToString(digest[:])
}

// NativeSandboxCompilerExecutionDigestV2 binds the compiler, the sandbox
// launcher, and the exact sandbox policy. Every variable-length field is
// length-prefixed and components have a fixed order, so no field can
// impersonate a delimiter or an adjacent field.
func NativeSandboxCompilerExecutionDigestV2(
	buildID string,
	hypcSHA256 string,
	nsjailSHA256 string,
	policySHA256 string,
) string {
	h := sha256.New()
	writeCompilerProvenanceField(h, CompilerProvenanceSchemaV2)
	writeCompilerProvenanceField(h, "native")
	writeCompilerProvenanceField(h, buildID)
	writeCompilerProvenanceUint64(h, 3)
	for _, component := range []CompilerProvenanceComponent{
		{Name: "hypc", SHA256: hypcSHA256},
		{Name: "nsjail", SHA256: nsjailSHA256},
		{Name: "policy", SHA256: policySHA256},
	} {
		writeCompilerProvenanceField(h, component.Name)
		writeCompilerProvenanceField(h, component.SHA256)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func writeCompilerProvenanceField(h hash.Hash, value string) {
	writeCompilerProvenanceUint64(h, uint64(len(value)))
	_, _ = h.Write([]byte(value))
}

func writeCompilerProvenanceUint64(h hash.Hash, value uint64) {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], value)
	_, _ = h.Write(encoded[:])
}

func isLowerSHA256(value string) bool {
	if len(value) != sha256.Size*2 || value != strings.ToLower(value) {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == sha256.Size
}

// VerificationResult is the in-process value passed from the verifier
// (M2) to MarkContractVerified. Lives in models so both db and
// verification packages can import it without an import cycle.
type VerificationResult struct {
	SourceCode           string
	Abi                  string
	ContractName         string
	CompilerVersion      string
	CompilerProvenance   *CompilerProvenance
	OptimizationEnabled  bool
	OptimizationRuns     int
	EvmVersion           string
	ConstructorArguments string
	Libraries            map[string]string
	Imports              map[string]string
	License              string
	VerificationMethod   string
}
