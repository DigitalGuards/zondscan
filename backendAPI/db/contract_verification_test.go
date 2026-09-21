package db

import (
	"strings"
	"testing"

	"backendAPI/models"
)

func TestContractVerificationSetPersistsExactCompilerProvenance(t *testing.T) {
	provenance := &models.CompilerProvenance{
		Schema:          "qrl.contract-compiler-provenance.v1",
		Kind:            "native",
		BuildID:         "current-q128",
		ExecutionDigest: strings.Repeat("a", 64),
		Components: []models.CompilerProvenanceComponent{{
			Name:   "hypc",
			SHA256: strings.Repeat("b", 64),
		}},
	}
	fields := contractVerificationSet(models.VerificationResult{
		CompilerVersion:    provenance.BuildID,
		CompilerProvenance: provenance,
	}, "2026-08-27T00:00:00Z")

	if fields["compilerVersion"] != provenance.BuildID {
		t.Fatalf("compilerVersion = %v", fields["compilerVersion"])
	}
	if fields["verificationRecordSchema"] != models.VerificationRecordSchemaV1 {
		t.Fatalf("verificationRecordSchema = %v", fields["verificationRecordSchema"])
	}
	if fields["compilerProvenance"] != provenance {
		t.Fatalf("compilerProvenance = %+v", fields["compilerProvenance"])
	}
	if fields["verifiedAt"] != "2026-08-27T00:00:00Z" {
		t.Fatalf("verifiedAt = %v", fields["verifiedAt"])
	}
}

func TestContractVerificationSetMarksNilProvenanceRecordForFailClosedClassification(t *testing.T) {
	fields := contractVerificationSet(models.VerificationResult{
		CompilerVersion: "historical",
	}, "2026-08-27T00:00:00Z")
	if _, exists := fields["compilerProvenance"]; exists {
		t.Fatalf("verification set unexpectedly contains provenance: %+v", fields)
	}
	if fields["verificationRecordSchema"] != models.VerificationRecordSchemaV1 {
		t.Fatalf("verificationRecordSchema = %v", fields["verificationRecordSchema"])
	}
}

func TestContractVerificationSetV2BindsDeploymentArtifact(t *testing.T) {
	provenance := &models.CompilerProvenance{
		Schema:          models.CompilerProvenanceSchemaV2,
		Kind:            "native",
		BuildID:         "current-q128",
		ExecutionDigest: strings.Repeat("a", 64),
	}
	result := models.VerificationResult{
		SourceCode:          "contract Main {}",
		Abi:                 `[{"type":"function","name":"value"}]`,
		ContractName:        "Main",
		CompilerVersion:     provenance.BuildID,
		CompilerProvenance:  provenance,
		OptimizationEnabled: true,
		OptimizationRuns:    200,
		VerificationMethod:  "full-source",
	}
	target, _ := verificationFenceFixture()
	fields := contractVerificationSet(result, "2026-08-27T00:00:00Z", target)
	if fields["verificationRecordSchema"] != models.VerificationRecordSchemaV2 {
		t.Fatalf("verificationRecordSchema = %v", fields["verificationRecordSchema"])
	}
	digest, ok := fields["verificationArtifactDigest"].(string)
	if !ok || !models.IsVerificationArtifactDigestV2(digest) {
		t.Fatalf("verificationArtifactDigest = %#v", fields["verificationArtifactDigest"])
	}
}
