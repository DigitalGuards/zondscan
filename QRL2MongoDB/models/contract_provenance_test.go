package models

import (
	"encoding/json"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestContractCompilerProvenanceBackwardCompatibility(t *testing.T) {
	legacyBSON, err := bson.Marshal(bson.M{
		"address":         "Qlegacy",
		"verified":        true,
		"compilerVersion": "historical",
	})
	if err != nil {
		t.Fatalf("encode legacy contract BSON: %v", err)
	}
	var legacy ContractInfo
	if err := bson.Unmarshal(legacyBSON, &legacy); err != nil {
		t.Fatalf("decode legacy contract BSON: %v", err)
	}
	if legacy.CompilerVersion != "historical" || legacy.CompilerProvenance != nil {
		t.Fatalf("legacy contract = %+v", legacy)
	}
	if legacy.HasVerificationRecordSchema() {
		t.Fatal("legacy contract unexpectedly has a verification record schema")
	}

	want := &CompilerProvenance{
		Schema:          "qrl.contract-compiler-provenance.v1",
		Kind:            "native",
		BuildID:         "current-q128",
		ExecutionDigest: strings.Repeat("a", 64),
		Components: []CompilerProvenanceComponent{{
			Name:   "hypc",
			SHA256: strings.Repeat("b", 64),
		}},
	}
	encoded, err := json.Marshal(ContractInfo{
		VerificationRecordSchema: VerificationRecordSchemaV1,
		CompilerVersion:          want.BuildID,
		CompilerProvenance:       want,
	})
	if err != nil {
		t.Fatalf("encode current contract JSON: %v", err)
	}
	var current ContractInfo
	if err := json.Unmarshal(encoded, &current); err != nil {
		t.Fatalf("decode current contract JSON: %v", err)
	}
	if current.CompilerProvenance == nil || current.CompilerProvenance.ExecutionDigest != want.ExecutionDigest ||
		current.CompilerProvenance.Components[0].SHA256 != want.Components[0].SHA256 {
		t.Fatalf("current contract provenance = %+v", current.CompilerProvenance)
	}
	if current.VerificationRecordSchema != VerificationRecordSchemaV1 || !current.HasVerificationRecordSchema() {
		t.Fatalf("current verification record schema = %q", current.VerificationRecordSchema)
	}
}
