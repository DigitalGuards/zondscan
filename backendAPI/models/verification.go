package models

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
	Result    *VerificationJobResultRef `bson:"result,omitempty" json:"result,omitempty"`
}

// VerificationJobPayload mirrors the inputs accepted by the /contract/verify
// endpoint. Kept as its own type so the job document and the request body
// can share a schema.
type VerificationJobPayload struct {
	SourceCode           string            `bson:"sourceCode,omitempty" json:"sourceCode,omitempty"`
	ContractName         string            `bson:"contractName,omitempty" json:"contractName,omitempty"`
	CompilerVersion      string            `bson:"compilerVersion,omitempty" json:"compilerVersion,omitempty"`
	OptimizationEnabled  bool              `bson:"optimizationEnabled" json:"optimizationEnabled"`
	OptimizationRuns     int               `bson:"optimizationRuns" json:"optimizationRuns"`
	EvmVersion           string            `bson:"evmVersion,omitempty" json:"evmVersion,omitempty"`
	ConstructorArguments string            `bson:"constructorArguments,omitempty" json:"constructorArguments,omitempty"`
	Libraries            map[string]string `bson:"libraries,omitempty" json:"libraries,omitempty"`
	License              string            `bson:"license,omitempty" json:"license,omitempty"`
	VerificationMethod   string            `bson:"verificationMethod,omitempty" json:"verificationMethod,omitempty"`
}

// VerificationJobResultRef is the small handle written back into the job
// after a successful verification — the full materialised result lives on
// the contract document itself (verified=true + abi + …). This struct
// exists so /contract/verify/:jobId can return a useful payload without
// re-querying the contracts collection.
type VerificationJobResultRef struct {
	Abi          string `bson:"abi,omitempty" json:"abi,omitempty"`
	BytecodeHash string `bson:"bytecodeHash,omitempty" json:"bytecodeHash,omitempty"`
	VerifiedAt   string `bson:"verifiedAt,omitempty" json:"verifiedAt,omitempty"`
}

// VerificationResult is the in-process value passed from the verifier
// (M2) to MarkContractVerified. Lives in models so both db and
// verification packages can import it without an import cycle.
type VerificationResult struct {
	SourceCode           string
	Abi                  string
	ContractName         string
	CompilerVersion      string
	OptimizationEnabled  bool
	OptimizationRuns     int
	EvmVersion           string
	ConstructorArguments string
	Libraries            map[string]string
	License              string
	VerificationMethod   string
}
