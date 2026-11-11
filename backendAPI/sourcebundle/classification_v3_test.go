package sourcebundle

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"backendAPI/models"
)

// v3GoldenVectors reads the schema-v3 records shared with the frontend's
// storedVerification tests, so both implementations prove the same bytes.
func v3GoldenVectors(t *testing.T) []models.ContractInfo {
	t.Helper()
	raw, err := os.ReadFile("../models/testdata/verification_record_v3.json")
	if err != nil {
		t.Fatalf("read v3 fixture: %v", err)
	}
	var fixture struct {
		Vectors []struct {
			Name   string          `json:"name"`
			Record json.RawMessage `json:"record"`
		} `json:"vectors"`
	}
	if err := json.Unmarshal(raw, &fixture); err != nil {
		t.Fatalf("decode v3 fixture: %v", err)
	}
	if len(fixture.Vectors) == 0 {
		t.Fatal("v3 fixture has no vectors")
	}
	records := make([]models.ContractInfo, 0, len(fixture.Vectors))
	for _, vector := range fixture.Vectors {
		var record models.ContractInfo
		if err := json.Unmarshal(vector.Record, &record); err != nil {
			t.Fatalf("decode v3 vector %q: %v", vector.Name, err)
		}
		records = append(records, record)
	}
	return records
}

func TestV3GoldenVectorsAreDigestBacked(t *testing.T) {
	for _, record := range v3GoldenVectors(t) {
		if record.VerificationRecordSchema != models.VerificationRecordSchemaV3 {
			t.Fatalf("vector schema = %q", record.VerificationRecordSchema)
		}
		if got := Digest(record.ContractName, record.SourceCode, record.Imports); got != record.SourceBundleDigest {
			t.Fatalf("source bundle digest = %q, fixture has %q", got, record.SourceBundleDigest)
		}
		want := models.VerificationArtifactDigestV3(
			verificationTargetFromContract(record),
			verificationResultFromContract(record),
			record.SourceBundleDigest,
		)
		if want != record.VerificationArtifactDigest {
			t.Fatalf("artifact digest = %q, fixture has %q", want, record.VerificationArtifactDigest)
		}
		if got := ClassifyStoredVerification(record); got != models.CompilerProvenanceDigestBacked {
			t.Fatalf("v3 vector classification = %q, want digest-backed", got)
		}
		if got := ClassifyRecordedMetadata(record); got != models.CompilerProvenanceDigestBacked {
			t.Fatalf("v3 vector metadata classification = %q, want digest-backed", got)
		}
	}
}

func TestClassifyStoredVerificationV3BindsCompileInputs(t *testing.T) {
	base := v3GoldenVectors(t)[0]
	if base.PrimarySourcePath == "" || !base.ViaIR {
		t.Fatalf("first vector must use a nested path and via-IR: %+v", base)
	}

	for name, mutate := range map[string]func(*models.ContractInfo){
		"via-IR dropped":       func(c *models.ContractInfo) { c.ViaIR = false },
		"primary path moved":   func(c *models.ContractInfo) { c.PrimarySourcePath = "contracts/Counter.hyp" },
		"primary path dropped": func(c *models.ContractInfo) { c.PrimarySourcePath = "" },
		"schema lowered to v2": func(c *models.ContractInfo) { c.VerificationRecordSchema = models.VerificationRecordSchemaV2 },
		"schema lowered to v1": func(c *models.ContractInfo) { c.VerificationRecordSchema = models.VerificationRecordSchemaV1 },
		"unknown schema":       func(c *models.ContractInfo) { c.VerificationRecordSchema = "qrl.contract-verification-record.v4" },
		"v2 digest namespace": func(c *models.ContractInfo) {
			c.VerificationArtifactDigest = strings.Replace(c.VerificationArtifactDigest, ".v3:", ".v2:", 1)
		},
		"import moved": func(c *models.ContractInfo) {
			c.Imports = map[string]string{"lib/MathLib.hyp": c.Imports["contracts/lib/MathLib.hyp"]}
		},
		"optimizer runs change": func(c *models.ContractInfo) { c.OptimizationRuns++ },
	} {
		t.Run(name, func(t *testing.T) {
			record := base
			record.Imports = map[string]string{}
			for key, value := range base.Imports {
				record.Imports[key] = value
			}
			mutate(&record)
			if got := ClassifyStoredVerification(record); got == models.CompilerProvenanceDigestBacked {
				t.Fatalf("mutated record classified as digest-backed")
			}
		})
	}
}

func TestClassifyStoredVerificationV3RejectsNonCanonicalInputs(t *testing.T) {
	vectors := v3GoldenVectors(t)
	viaOnly := vectors[1]
	if viaOnly.PrimarySourcePath != "" || !viaOnly.ViaIR {
		t.Fatalf("second vector must use via-IR at the default path: %+v", viaOnly)
	}

	// The default path spelled out binds the same digest but is not the
	// canonical stored form, so it is rejected.
	spelled := viaOnly
	spelled.PrimarySourcePath = models.DefaultPrimarySourcePath(spelled.ContractName)
	if got := ClassifyStoredVerification(spelled); got != models.CompilerProvenanceInvalidRecorded {
		t.Fatalf("spelled-out default path = %q, want invalid-recorded", got)
	}

	// A v3 record that carries neither v3 input must have been written as v2.
	neither := viaOnly
	neither.ViaIR = false
	neither.VerificationArtifactDigest = models.VerificationArtifactDigestV3(
		verificationTargetFromContract(neither),
		verificationResultFromContract(neither),
		neither.SourceBundleDigest,
	)
	if got := ClassifyStoredVerification(neither); got != models.CompilerProvenanceInvalidRecorded {
		t.Fatalf("v3 record without v3 inputs = %q, want invalid-recorded", got)
	}
	if got := ClassifyRecordedMetadata(neither); got != models.CompilerProvenanceInvalidRecorded {
		t.Fatalf("v3 metadata without v3 inputs = %q, want invalid-recorded", got)
	}
}

func TestClassifyStoredVerificationRejectsV3InputsOnOlderSchemas(t *testing.T) {
	for name, mutate := range map[string]func(*models.ContractInfo){
		"via-IR":       func(c *models.ContractInfo) { c.ViaIR = true },
		"primary path": func(c *models.ContractInfo) { c.PrimarySourcePath = "src/Main.hyp" },
	} {
		t.Run(name, func(t *testing.T) {
			v2 := artifactBackedContract()
			mutate(&v2)
			if got := ClassifyStoredVerification(v2); got != models.CompilerProvenanceInvalidRecorded {
				t.Fatalf("v2 record carrying %s = %q, want invalid-recorded", name, got)
			}
			if got := ClassifyRecordedMetadata(v2); got != models.CompilerProvenanceInvalidRecorded {
				t.Fatalf("v2 metadata carrying %s = %q, want invalid-recorded", name, got)
			}
			v1 := digestBackedContract()
			mutate(&v1)
			if got := ClassifyStoredVerification(v1); got != models.CompilerProvenanceInvalidRecorded {
				t.Fatalf("v1 record carrying %s = %q, want invalid-recorded", name, got)
			}
		})
	}
}
