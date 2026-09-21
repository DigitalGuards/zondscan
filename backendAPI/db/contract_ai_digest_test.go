package db

import (
	"context"
	"errors"
	"strings"
	"testing"

	"backendAPI/models"
	"backendAPI/sourcebundle"
)

func TestContractVerificationSetPersistsVersionedSourceBundleDigest(t *testing.T) {
	result := models.VerificationResult{
		ContractName: "Primary",
		SourceCode:   "contract Primary {}",
		Imports: map[string]string{
			"lib/A.hyp": "contract A {}",
		},
	}
	want := sourcebundle.Digest(result.ContractName, result.SourceCode, result.Imports)
	fields := contractVerificationSet(result, "2026-08-27T00:00:00Z")
	if fields["sourceBundleDigest"] != want {
		t.Fatalf("sourceBundleDigest = %q, want %q", fields["sourceBundleDigest"], want)
	}
	if fields["verificationRecordSchema"] != models.VerificationRecordSchemaV1 {
		t.Fatalf("verificationRecordSchema = %v", fields["verificationRecordSchema"])
	}
}

func TestSanitizeCachedExplanationRequiresExactBundleDigest(t *testing.T) {
	base := models.ContractInfo{
		ContractName: "Primary",
		SourceCode:   "contract Primary {}",
		Imports: map[string]string{
			"lib/A.hyp": "contract A {}",
		},
		AIExplanation:      "current explanation",
		AIExplanationAt:    "2026-08-27T00:00:00Z",
		AIExplanationModel: "test-model",
	}
	digest := sourcebundle.Digest(base.ContractName, base.SourceCode, base.Imports)
	base.CompilerVersion = "current-q128"
	base.CompilerProvenance = validCompilerProvenance(base.CompilerVersion)
	base.SourceBundleDigest = digest
	base.VerificationRecordSchema = models.VerificationRecordSchemaV1

	t.Run("matching modern cache", func(t *testing.T) {
		contract := base
		contract.AIExplanationSourceDigest = digest
		sanitizeCachedExplanation(&contract)
		if contract.AIExplanation != base.AIExplanation {
			t.Fatalf("matching cache was cleared: %+v", contract)
		}
	})

	t.Run("legacy cache without digest", func(t *testing.T) {
		contract := base
		sanitizeCachedExplanation(&contract)
		if contract.AIExplanation != "" || contract.AIExplanationAt != "" ||
			contract.AIExplanationModel != "" || contract.AIExplanationSourceDigest != "" {
			t.Fatalf("legacy cache was served: %+v", contract)
		}
	})

	t.Run("cache from prior bundle", func(t *testing.T) {
		contract := base
		contract.AIExplanationSourceDigest = sourcebundle.Digest("Primary", "old bundle", nil)
		sanitizeCachedExplanation(&contract)
		if contract.AIExplanation != "" {
			t.Fatalf("stale cache was served: %+v", contract)
		}
	})

	t.Run("non-digest-backed compiler provenance", func(t *testing.T) {
		contract := base
		contract.CompilerProvenance = nil
		contract.AIExplanationSourceDigest = digest
		sanitizeCachedExplanation(&contract)
		if contract.AIExplanation != "" {
			t.Fatalf("cache with legacy compiler provenance was served: %+v", contract)
		}
	})
}

func TestExplanationSourceSnapshotFilterBindsConcurrentReverification(t *testing.T) {
	oldSource := "contract Primary {}"
	oldDigest := sourcebundle.Digest("Primary", oldSource, nil)
	newDigest := sourcebundle.Digest("Primary", "contract Primary { function changed() {} }", nil)
	expected := models.ContractInfo{
		ContractName:             "Primary",
		SourceCode:               oldSource,
		SourceBundleDigest:       oldDigest,
		VerificationRecordSchema: models.VerificationRecordSchemaV1,
		VerifiedAt:               "2026-08-27T00:00:00.000000001Z",
	}
	filter := explanationSourceSnapshotFilter("Qcontract", expected, oldDigest)
	if filter["sourceBundleDigest"] != oldDigest {
		t.Fatalf("filter digest = %q, want %q", filter["sourceBundleDigest"], oldDigest)
	}
	if filter["verificationRecordSchema"] != models.VerificationRecordSchemaV1 {
		t.Fatalf("filter verificationRecordSchema = %q", filter["verificationRecordSchema"])
	}
	if filter["sourceBundleDigest"] == newDigest {
		t.Fatalf("old generation filter matches new digest %q", newDigest)
	}
	if filter["verifiedAt"] != expected.VerifiedAt {
		t.Fatalf("filter verifiedAt = %q, want %q", filter["verifiedAt"], expected.VerifiedAt)
	}
	sameSourceReverifiedAt := "2026-08-27T00:00:00.000000002Z"
	if filter["verifiedAt"] == sameSourceReverifiedAt {
		t.Fatalf("old generation filter matches same-source re-verification at %q", sameSourceReverifiedAt)
	}

	if err := SaveContractExplanation(
		context.Background(),
		"Qlegacy",
		"explanation",
		"model",
		"2026-08-27T00:00:00Z",
		oldDigest,
		models.AIExplanationLease{},
		models.ContractInfo{},
	); !errors.Is(err, ErrSourceBundleChanged) {
		t.Fatalf("undigested snapshot save error = %v", err)
	}

	invalidCompiler := expected
	invalidCompiler.CompilerVersion = "current-q128"
	invalidCompiler.CompilerProvenance = validCompilerProvenance(invalidCompiler.CompilerVersion)
	invalidCompiler.CompilerProvenance.ExecutionDigest = strings.Repeat("c", 64)
	if err := SaveContractExplanation(
		context.Background(),
		"Qinvalid",
		"explanation",
		"model",
		"2026-08-27T00:00:00Z",
		oldDigest,
		models.AIExplanationLease{},
		invalidCompiler,
	); !errors.Is(err, ErrSourceBundleChanged) {
		t.Fatalf("invalid provenance snapshot save error = %v", err)
	}
}

func validCompilerProvenance(buildID string) *models.CompilerProvenance {
	hypcSHA256 := strings.Repeat("a", 64)
	nsjailSHA256 := strings.Repeat("b", 64)
	policySHA256 := strings.Repeat("c", 64)
	return &models.CompilerProvenance{
		Schema:  models.CompilerProvenanceSchemaV2,
		Kind:    "native",
		BuildID: buildID,
		ExecutionDigest: models.NativeSandboxCompilerExecutionDigestV2(
			buildID,
			hypcSHA256,
			nsjailSHA256,
			policySHA256,
		),
		Components: []models.CompilerProvenanceComponent{
			{Name: "hypc", SHA256: hypcSHA256},
			{Name: "nsjail", SHA256: nsjailSHA256},
			{Name: "policy", SHA256: policySHA256},
		},
	}
}
