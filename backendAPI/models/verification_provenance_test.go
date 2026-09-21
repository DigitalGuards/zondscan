package models

import (
	"encoding/json"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func testCompilerProvenance() *CompilerProvenance {
	return &CompilerProvenance{
		Schema:          "qrl.contract-compiler-provenance.v1",
		Kind:            "native",
		BuildID:         "current-q128",
		ExecutionDigest: strings.Repeat("a", 64),
		Components: []CompilerProvenanceComponent{{
			Name:   "hypc",
			SHA256: strings.Repeat("b", 64),
		}},
	}
}

func TestLegacyContractInfoDecodesWithoutCompilerProvenance(t *testing.T) {
	legacyJSON := []byte(`{"address":"Qlegacy","verified":true,"compilerVersion":"historical"}`)
	var fromJSON ContractInfo
	if err := json.Unmarshal(legacyJSON, &fromJSON); err != nil {
		t.Fatalf("decode legacy contract JSON: %v", err)
	}
	if fromJSON.CompilerVersion != "historical" || fromJSON.CompilerProvenance != nil {
		t.Fatalf("legacy contract JSON = %+v", fromJSON)
	}

	legacyBSON, err := bson.Marshal(bson.M{
		"address":         "Qlegacy",
		"verified":        true,
		"compilerVersion": "historical",
	})
	if err != nil {
		t.Fatalf("encode legacy contract BSON: %v", err)
	}
	var fromBSON ContractInfo
	if err := bson.Unmarshal(legacyBSON, &fromBSON); err != nil {
		t.Fatalf("decode legacy contract BSON: %v", err)
	}
	if fromBSON.CompilerVersion != "historical" || fromBSON.CompilerProvenance != nil {
		t.Fatalf("legacy contract BSON = %+v", fromBSON)
	}
}

func TestVerificationJobProvenanceRoundTripAndLegacyDecode(t *testing.T) {
	legacyJSON := []byte(`{
		"jobId":"legacy-job",
		"payload":{"compilerVersion":"historical"},
		"result":{"verifiedAt":"2026-08-27T00:00:00Z"}
	}`)
	var legacy ContractVerificationJob
	if err := json.Unmarshal(legacyJSON, &legacy); err != nil {
		t.Fatalf("decode legacy verification job: %v", err)
	}
	if legacy.Payload.CompilerProvenance != nil || legacy.Result == nil || legacy.Result.CompilerProvenance != nil {
		t.Fatalf("legacy verification job = %+v", legacy)
	}

	want := testCompilerProvenance()
	job := ContractVerificationJob{
		JobID: "current-job",
		Payload: VerificationJobPayload{
			CompilerVersion:    want.BuildID,
			CompilerProvenance: want,
		},
		Result: &VerificationJobResultRef{CompilerProvenance: want},
	}
	encoded, err := bson.Marshal(job)
	if err != nil {
		t.Fatalf("encode verification job BSON: %v", err)
	}
	var got ContractVerificationJob
	if err := bson.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("decode verification job BSON: %v", err)
	}
	if got.Payload.CompilerProvenance == nil || got.Result == nil || got.Result.CompilerProvenance == nil ||
		got.Payload.CompilerProvenance.ExecutionDigest != want.ExecutionDigest ||
		got.Result.CompilerProvenance.Components[0].SHA256 != want.Components[0].SHA256 {
		t.Fatalf("verification job provenance = %+v", got)
	}
}

func TestNilCompilerProvenanceIsOmittedFromJSON(t *testing.T) {
	encoded, err := json.Marshal(ContractInfo{CompilerVersion: "historical"})
	if err != nil {
		t.Fatalf("encode contract info: %v", err)
	}
	if strings.Contains(string(encoded), "compilerProvenance") {
		t.Fatalf("legacy contract JSON unexpectedly contains provenance: %s", encoded)
	}
}

func TestVerificationArtifactDigestV2IsDeterministicAndDeploymentBound(t *testing.T) {
	target := VerificationTarget{
		Address:             "Q" + strings.Repeat("a", 128),
		CreationTransaction: "0x" + strings.Repeat("b", 64),
		CreationBlockNumber: "0x10",
		CreationBlockHash:   "0x" + strings.Repeat("c", 64),
		ChainID:             "0x539",
		DeployedCodeSHA256:  strings.Repeat("d", 64),
	}
	result := VerificationResult{
		Abi:                 `[{"type":"function","name":"value"}]`,
		ContractName:        "Main",
		CompilerVersion:     "current-q128",
		CompilerProvenance:  testCompilerProvenance(),
		OptimizationEnabled: true,
		OptimizationRuns:    200,
		Libraries: map[string]string{
			"Z": "Qz",
			"A": "Qa",
		},
		VerificationMethod: "full-source",
	}
	sourceDigest := "qrl.verified-source-bundle.v1:sha256:" + strings.Repeat("e", 64)
	want := VerificationArtifactDigestV2(target, result, sourceDigest)
	if !IsVerificationArtifactDigestV2(want) {
		t.Fatalf("artifact digest is malformed: %q", want)
	}

	reordered := result
	reordered.Libraries = map[string]string{"A": "Qa", "Z": "Qz"}
	if got := VerificationArtifactDigestV2(target, reordered, sourceDigest); got != want {
		t.Fatalf("map insertion order changed digest: %q != %q", got, want)
	}

	changedTarget := target
	changedTarget.CreationBlockHash = "0x" + strings.Repeat("f", 64)
	if got := VerificationArtifactDigestV2(changedTarget, result, sourceDigest); got == want {
		t.Fatal("creation block mutation retained artifact digest")
	}
	changedResult := result
	changedResult.Abi += " "
	if got := VerificationArtifactDigestV2(target, changedResult, sourceDigest); got == want {
		t.Fatal("ABI mutation retained artifact digest")
	}
}
