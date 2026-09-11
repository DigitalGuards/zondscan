package sourcebundle

import (
	"encoding/hex"
	"strings"

	"backendAPI/models"
)

// ClassifyStoredVerification composes compiler artifact identity with the
// exact persisted source bundle. New verifier writes persist both atomically;
// only records with no schema marker, compiler provenance, or source digest
// are legacy-unrecorded.
func ClassifyStoredVerification(contract models.ContractInfo) models.CompilerProvenanceStatus {
	if status, classified := classifyVerificationRecordSchema(contract); classified {
		return status
	}
	compilerStatus := models.ClassifyCompilerProvenance(
		contract.CompilerProvenance,
		contract.CompilerVersion,
	)
	if compilerStatus != models.CompilerProvenanceDigestBacked ||
		contract.SourceBundleDigest == "" ||
		contract.ContractName == "" ||
		contract.SourceCode == "" {
		return models.CompilerProvenanceInvalidRecorded
	}
	if contract.SourceBundleDigest != Digest(contract.ContractName, contract.SourceCode, contract.Imports) {
		return models.CompilerProvenanceInvalidRecorded
	}
	if contract.VerificationRecordSchema == models.VerificationRecordSchemaV2 {
		if !models.IsVerificationArtifactDigestV2(contract.VerificationArtifactDigest) ||
			contract.VerificationArtifactDigest != models.VerificationArtifactDigestV2(
				verificationTargetFromContract(contract),
				verificationResultFromContract(contract),
				contract.SourceBundleDigest,
			) {
			return models.CompilerProvenanceInvalidRecorded
		}
	}
	return models.CompilerProvenanceDigestBacked
}

// ClassifyRecordedMetadata is the lean classification used by compact
// transaction/log metadata reads that intentionally project out source bytes.
// It validates the compiler record and the versioned source-digest syntax but
// cannot recompute source identity. Full contract and AI paths must use
// ClassifyStoredVerification instead.
func ClassifyRecordedMetadata(contract models.ContractInfo) models.CompilerProvenanceStatus {
	if status, classified := classifyVerificationRecordSchema(contract); classified {
		return status
	}
	compilerStatus := models.ClassifyCompilerProvenance(
		contract.CompilerProvenance,
		contract.CompilerVersion,
	)
	if compilerStatus != models.CompilerProvenanceDigestBacked ||
		!isVersionedDigest(contract.SourceBundleDigest) {
		return models.CompilerProvenanceInvalidRecorded
	}
	if contract.VerificationRecordSchema == models.VerificationRecordSchemaV2 &&
		!models.IsVerificationArtifactDigestV2(contract.VerificationArtifactDigest) {
		return models.CompilerProvenanceInvalidRecorded
	}
	return models.CompilerProvenanceDigestBacked
}

func classifyVerificationRecordSchema(
	contract models.ContractInfo,
) (models.CompilerProvenanceStatus, bool) {
	if !contract.HasVerificationRecordSchema() {
		if !contract.HasCompilerProvenanceField() && !contract.HasSourceBundleDigestField() {
			return models.CompilerProvenanceLegacyUnrecorded, true
		}
		return models.CompilerProvenanceInvalidRecorded, true
	}
	if contract.VerificationRecordSchema != models.VerificationRecordSchemaV1 &&
		contract.VerificationRecordSchema != models.VerificationRecordSchemaV2 {
		return models.CompilerProvenanceInvalidRecorded, true
	}
	return "", false
}

func verificationTargetFromContract(contract models.ContractInfo) models.VerificationTarget {
	return models.VerificationTarget{
		Address:             contract.ContractAddress,
		CreationTransaction: contract.CreationTransaction,
		CreationBlockNumber: contract.CreationBlockNumber,
		CreationBlockHash:   contract.CreationBlockHash,
		ChainID:             contract.ChainID,
		DeployedCodeSHA256:  contract.ContractCodeSHA256,
		GenesisContract:     contract.GenesisContract,
	}
}

func verificationResultFromContract(contract models.ContractInfo) models.VerificationResult {
	return models.VerificationResult{
		SourceCode:           contract.SourceCode,
		Abi:                  contract.Abi,
		ContractName:         contract.ContractName,
		CompilerVersion:      contract.CompilerVersion,
		CompilerProvenance:   contract.CompilerProvenance,
		OptimizationEnabled:  contract.OptimizationEnabled,
		OptimizationRuns:     contract.OptimizationRuns,
		EvmVersion:           contract.EvmVersion,
		ConstructorArguments: contract.ConstructorArguments,
		Libraries:            contract.Libraries,
		Imports:              contract.Imports,
		License:              contract.License,
		VerificationMethod:   contract.VerificationMethod,
	}
}

func isVersionedDigest(value string) bool {
	if !strings.HasPrefix(value, digestPrefix) {
		return false
	}
	digest := strings.TrimPrefix(value, digestPrefix)
	if len(digest) != 64 || digest != strings.ToLower(digest) {
		return false
	}
	decoded, err := hex.DecodeString(digest)
	return err == nil && len(decoded) == 32
}
