package models

import (
	"encoding/json"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestContractInfoTracksVerificationRecordSchemaBSONPresence(t *testing.T) {
	for _, test := range []struct {
		name              string
		document          bson.D
		wantSchema        string
		wantSchemaPresent bool
		wantCompilerField bool
		wantSourceDigest  bool
		wantJSONFields    map[string]string
	}{
		{
			name:     "absent legacy trust fields",
			document: bson.D{{Key: "verified", Value: true}},
		},
		{
			name: "explicit null marker",
			document: bson.D{
				{Key: "verified", Value: true},
				{Key: "verificationRecordSchema", Value: nil},
			},
			wantSchemaPresent: true,
			wantJSONFields: map[string]string{
				"verificationRecordSchema": "null",
			},
		},
		{
			name: "explicit null provenance",
			document: bson.D{
				{Key: "verified", Value: true},
				{Key: "compilerProvenance", Value: nil},
			},
			wantCompilerField: true,
			wantJSONFields: map[string]string{
				"compilerProvenance": "null",
			},
		},
		{
			name: "explicit null source digest",
			document: bson.D{
				{Key: "verified", Value: true},
				{Key: "sourceBundleDigest", Value: nil},
			},
			wantSourceDigest: true,
			wantJSONFields: map[string]string{
				"sourceBundleDigest": "null",
			},
		},
		{
			name: "all trust fields explicit null",
			document: bson.D{
				{Key: "verified", Value: true},
				{Key: "verificationRecordSchema", Value: nil},
				{Key: "compilerProvenance", Value: nil},
				{Key: "sourceBundleDigest", Value: nil},
			},
			wantSchemaPresent: true,
			wantCompilerField: true,
			wantSourceDigest:  true,
			wantJSONFields: map[string]string{
				"verificationRecordSchema": "null",
				"compilerProvenance":       "null",
				"sourceBundleDigest":       "null",
			},
		},
		{
			name: "explicit empty marker",
			document: bson.D{
				{Key: "verified", Value: true},
				{Key: "verificationRecordSchema", Value: ""},
			},
			wantSchemaPresent: true,
			wantJSONFields: map[string]string{
				"verificationRecordSchema": `""`,
			},
		},
		{
			name: "explicit empty source digest",
			document: bson.D{
				{Key: "verified", Value: true},
				{Key: "sourceBundleDigest", Value: ""},
			},
			wantSourceDigest: true,
			wantJSONFields: map[string]string{
				"sourceBundleDigest": `""`,
			},
		},
		{
			name: "versioned marker",
			document: bson.D{
				{Key: "verified", Value: true},
				{Key: "verificationRecordSchema", Value: VerificationRecordSchemaV1},
			},
			wantSchema:        VerificationRecordSchemaV1,
			wantSchemaPresent: true,
			wantJSONFields: map[string]string{
				"verificationRecordSchema": `"qrl.contract-verification-record.v1"`,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := bson.Marshal(test.document)
			if err != nil {
				t.Fatalf("encode BSON: %v", err)
			}
			var contract ContractInfo
			if err := bson.Unmarshal(encoded, &contract); err != nil {
				t.Fatalf("decode BSON: %v", err)
			}
			if contract.VerificationRecordSchema != test.wantSchema {
				t.Fatalf("schema = %q, want %q", contract.VerificationRecordSchema, test.wantSchema)
			}
			if contract.HasVerificationRecordSchema() != test.wantSchemaPresent {
				t.Fatalf("schema presence = %t, want %t", contract.HasVerificationRecordSchema(), test.wantSchemaPresent)
			}
			if contract.HasCompilerProvenanceField() != test.wantCompilerField {
				t.Fatalf("compiler provenance presence = %t, want %t", contract.HasCompilerProvenanceField(), test.wantCompilerField)
			}
			if contract.HasSourceBundleDigestField() != test.wantSourceDigest {
				t.Fatalf("source digest presence = %t, want %t", contract.HasSourceBundleDigestField(), test.wantSourceDigest)
			}

			jsonDocument, err := json.Marshal(contract)
			if err != nil {
				t.Fatalf("encode JSON: %v", err)
			}
			for _, internalField := range []string{
				"verificationRecordSchemaSeen",
				"verificationRecordSchemaNull",
				"compilerProvenanceSeen",
				"compilerProvenanceNull",
				"sourceBundleDigestSeen",
				"sourceBundleDigestNull",
			} {
				if strings.Contains(string(jsonDocument), internalField) {
					t.Fatalf("internal presence state leaked into JSON: %s", jsonDocument)
				}
			}
			var jsonFields map[string]json.RawMessage
			if err := json.Unmarshal(jsonDocument, &jsonFields); err != nil {
				t.Fatalf("decode emitted JSON: %v", err)
			}
			if string(jsonFields["verified"]) != "true" {
				t.Fatalf("ordinary JSON field was not preserved: %s", jsonDocument)
			}
			for _, field := range []string{
				"verificationRecordSchema",
				"compilerProvenance",
				"sourceBundleDigest",
			} {
				want, wantPresent := test.wantJSONFields[field]
				got, gotPresent := jsonFields[field]
				if gotPresent != wantPresent || (wantPresent && string(got) != want) {
					t.Fatalf("JSON field %q = %s (present %t), want %s (present %t): %s", field, got, gotPresent, want, wantPresent, jsonDocument)
				}
			}
		})
	}
}

func TestContractInfoMarshalJSONPreservesOrdinaryTrustValues(t *testing.T) {
	provenance := &CompilerProvenance{
		Schema:          CompilerProvenanceSchemaV1,
		Kind:            "native",
		BuildID:         "current",
		ExecutionDigest: strings.Repeat("a", 64),
		Components: []CompilerProvenanceComponent{{
			Name:   "hypc",
			SHA256: strings.Repeat("b", 64),
		}},
	}
	encoded, err := json.Marshal(ContractInfo{
		ContractAddress:          "Qcurrent",
		Verified:                 true,
		VerificationRecordSchema: VerificationRecordSchemaV1,
		CompilerProvenance:       provenance,
		SourceBundleDigest:       "qrl.verified-source-bundle.v1:sha256:" + strings.Repeat("c", 64),
	})
	if err != nil {
		t.Fatalf("encode ordinary contract JSON: %v", err)
	}
	var decoded ContractInfo
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("decode ordinary contract JSON: %v", err)
	}
	if decoded.ContractAddress != "Qcurrent" || !decoded.Verified ||
		decoded.VerificationRecordSchema != VerificationRecordSchemaV1 ||
		decoded.CompilerProvenance == nil ||
		decoded.CompilerProvenance.ExecutionDigest != provenance.ExecutionDigest ||
		decoded.SourceBundleDigest == "" {
		t.Fatalf("ordinary contract JSON changed shape: %s", encoded)
	}
}
