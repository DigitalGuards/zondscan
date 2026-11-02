package db

import (
	"testing"

	"backendAPI/models"
	"go.mongodb.org/mongo-driver/bson"
)

func v3WriterResult() models.VerificationResult {
	return models.VerificationResult{
		SourceCode:          "import \"../lib/Math.hyp\"; contract Main {}",
		Abi:                 `[]`,
		ContractName:        "Main",
		CompilerVersion:     "current-q128",
		OptimizationEnabled: true,
		OptimizationRuns:    200,
		Imports:             map[string]string{"contracts/lib/Math.hyp": "library Math {}"},
		PrimarySourcePath:   "contracts/core/Main.hyp",
		ViaIR:               true,
		VerificationMethod:  "full-source",
	}
}

func TestContractVerificationSetWritesV3ForSourceTreeAndViaIR(t *testing.T) {
	target, _ := verificationFenceFixture()
	result := v3WriterResult()
	fields := contractVerificationSet(result, "2026-11-02T00:00:00Z", target)

	if fields["verificationRecordSchema"] != models.VerificationRecordSchemaV3 {
		t.Fatalf("verificationRecordSchema = %v", fields["verificationRecordSchema"])
	}
	digest, ok := fields["verificationArtifactDigest"].(string)
	if !ok || !models.IsVerificationArtifactDigestV3(digest) {
		t.Fatalf("verificationArtifactDigest = %#v", fields["verificationArtifactDigest"])
	}
	if fields["primarySourcePath"] != "contracts/core/Main.hyp" {
		t.Fatalf("primarySourcePath = %#v", fields["primarySourcePath"])
	}
	if fields["viaIR"] != true {
		t.Fatalf("viaIR = %#v", fields["viaIR"])
	}

	update := contractVerificationUpdate(result, "2026-11-02T00:00:00Z", target)
	unset, ok := update["$unset"].(bson.M)
	if !ok {
		t.Fatalf("$unset = %#v", update["$unset"])
	}
	for _, field := range []string{"primarySourcePath", "viaIR"} {
		if _, exists := unset[field]; exists {
			t.Fatalf("current %s is both set and unset: %#v", field, unset)
		}
	}
}

func TestContractVerificationSetOmitsDefaultPrimaryPathOnV3(t *testing.T) {
	target, _ := verificationFenceFixture()
	result := v3WriterResult()
	result.PrimarySourcePath = "Main.hyp"
	fields := contractVerificationSet(result, "2026-11-02T00:00:00Z", target)
	if fields["verificationRecordSchema"] != models.VerificationRecordSchemaV3 {
		t.Fatalf("via-IR record schema = %v", fields["verificationRecordSchema"])
	}
	if _, exists := fields["primarySourcePath"]; exists {
		t.Fatalf("default primary path was persisted: %#v", fields["primarySourcePath"])
	}
}

func TestContractVerificationUpdateDefaultLayoutStaysV2AndClearsV3Inputs(t *testing.T) {
	target, _ := verificationFenceFixture()
	result := v3WriterResult()
	result.PrimarySourcePath = ""
	result.ViaIR = false
	result.Imports = map[string]string{"lib/Math.hyp": "library Math {}"}

	update := contractVerificationUpdate(result, "2026-11-02T00:00:00Z", target)
	set, ok := update["$set"].(bson.M)
	if !ok {
		t.Fatalf("$set = %#v", update["$set"])
	}
	if set["verificationRecordSchema"] != models.VerificationRecordSchemaV2 {
		t.Fatalf("verificationRecordSchema = %v", set["verificationRecordSchema"])
	}
	if digest, _ := set["verificationArtifactDigest"].(string); !models.IsVerificationArtifactDigestV2(digest) {
		t.Fatalf("verificationArtifactDigest = %#v", set["verificationArtifactDigest"])
	}
	unset, ok := update["$unset"].(bson.M)
	if !ok {
		t.Fatalf("$unset = %#v", update["$unset"])
	}
	for _, field := range []string{"primarySourcePath", "viaIR"} {
		if _, exists := set[field]; exists {
			t.Fatalf("default layout persisted %s: %#v", field, set)
		}
		if _, exists := unset[field]; !exists {
			t.Fatalf("$unset is missing stale %s: %#v", field, unset)
		}
	}
}

func TestContractVerificationSetV1NeverPersistsV3Inputs(t *testing.T) {
	fields := contractVerificationSet(v3WriterResult(), "2026-11-02T00:00:00Z")
	if fields["verificationRecordSchema"] != models.VerificationRecordSchemaV1 {
		t.Fatalf("verificationRecordSchema = %v", fields["verificationRecordSchema"])
	}
	for _, field := range []string{"primarySourcePath", "viaIR", "verificationArtifactDigest"} {
		if _, exists := fields[field]; exists {
			t.Fatalf("legacy v1 set contains %s: %#v", field, fields)
		}
	}
}
