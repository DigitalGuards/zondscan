package sourcebundle

import (
	"strings"
	"testing"

	"backendAPI/models"
	"go.mongodb.org/mongo-driver/bson"
)

func digestBackedContract() models.ContractInfo {
	const (
		buildID        = "0.2.0-develop.2026.8.27+commit.6f862206.mod.Linux.g++"
		compilerSHA256 = "fe8e2344dbd902d6fc8c8cbb24114378c2de3a996b0d58f642d303c8bf30e930"
		nsjailSHA256   = "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"
		policySHA256   = "cc2c6d14e943c9b4b9252e69c2fbf9a5a4313938cd561594a255746393baaa8c"
	)
	contract := models.ContractInfo{
		Verified:                 true,
		VerificationRecordSchema: models.VerificationRecordSchemaV1,
		ContractName:             "Main",
		SourceCode:               "contract Main {}",
		Imports:                  map[string]string{"lib/A.hyp": "contract A {}"},
		CompilerVersion:          buildID,
		CompilerProvenance: &models.CompilerProvenance{
			Schema:  models.CompilerProvenanceSchemaV2,
			Kind:    "native",
			BuildID: buildID,
			ExecutionDigest: models.NativeSandboxCompilerExecutionDigestV2(
				buildID,
				compilerSHA256,
				nsjailSHA256,
				policySHA256,
			),
			Components: []models.CompilerProvenanceComponent{
				{Name: "hypc", SHA256: compilerSHA256},
				{Name: "nsjail", SHA256: nsjailSHA256},
				{Name: "policy", SHA256: policySHA256},
			},
		},
	}
	contract.SourceBundleDigest = Digest(contract.ContractName, contract.SourceCode, contract.Imports)
	return contract
}

func artifactBackedContract() models.ContractInfo {
	contract := digestBackedContract()
	contract.VerificationRecordSchema = models.VerificationRecordSchemaV2
	contract.ContractAddress = "Q" + strings.Repeat("a", 128)
	contract.ContractCodeSHA256 = strings.Repeat("b", 64)
	contract.CreationTransaction = "0x" + strings.Repeat("c", 64)
	contract.CreationBlockNumber = "0x10"
	contract.CreationBlockHash = "0x" + strings.Repeat("d", 64)
	contract.ChainID = "0x539"
	contract.Abi = `[{"type":"function","name":"value"}]`
	contract.OptimizationEnabled = true
	contract.OptimizationRuns = 200
	contract.EvmVersion = "cancun"
	contract.VerificationMethod = "full-source"
	contract.VerificationArtifactDigest = models.VerificationArtifactDigestV2(
		verificationTargetFromContract(contract),
		verificationResultFromContract(contract),
		contract.SourceBundleDigest,
	)
	return contract
}

func TestClassifyStoredVerificationV2BindsCompleteArtifact(t *testing.T) {
	current := artifactBackedContract()
	if got := ClassifyStoredVerification(current); got != models.CompilerProvenanceDigestBacked {
		t.Fatalf("V2 status = %q, want digest-backed", got)
	}

	for name, mutate := range map[string]func(*models.ContractInfo){
		"artifact digest absent":   func(c *models.ContractInfo) { c.VerificationArtifactDigest = "" },
		"ABI changed":              func(c *models.ContractInfo) { c.Abi += " " },
		"compiler setting changed": func(c *models.ContractInfo) { c.OptimizationRuns++ },
		"deployed code changed":    func(c *models.ContractInfo) { c.ContractCodeSHA256 = strings.Repeat("e", 64) },
		"chain changed":            func(c *models.ContractInfo) { c.ChainID = "0x1" },
		"creation block changed":   func(c *models.ContractInfo) { c.CreationBlockHash = "0x" + strings.Repeat("f", 64) },
	} {
		t.Run(name, func(t *testing.T) {
			contract := artifactBackedContract()
			mutate(&contract)
			if got := ClassifyStoredVerification(contract); got != models.CompilerProvenanceInvalidRecorded {
				t.Fatalf("V2 mutated status = %q, want invalid-recorded", got)
			}
		})
	}
}

func TestClassifyStoredVerificationCompositeTrust(t *testing.T) {
	current := digestBackedContract()
	if got := ClassifyStoredVerification(current); got != models.CompilerProvenanceDigestBacked {
		t.Fatalf("current status = %q, want digest-backed", got)
	}

	legacy := current
	legacy.VerificationRecordSchema = ""
	legacy.CompilerProvenance = nil
	legacy.SourceBundleDigest = ""
	if got := ClassifyStoredVerification(legacy); got != models.CompilerProvenanceLegacyUnrecorded {
		t.Fatalf("legacy status = %q, want legacy-unrecorded", got)
	}

	cases := map[string]func(*models.ContractInfo){
		"schema marker absent from composite":       func(c *models.ContractInfo) { c.VerificationRecordSchema = "" },
		"schema marker unknown":                     func(c *models.ContractInfo) { c.VerificationRecordSchema = "future" },
		"compiler valid and source digest absent":   func(c *models.ContractInfo) { c.SourceBundleDigest = "" },
		"compiler absent and source digest present": func(c *models.ContractInfo) { c.CompilerProvenance = nil },
		"source digest mismatch": func(c *models.ContractInfo) {
			c.SourceBundleDigest = Version + ":sha256:" + strings.Repeat("c", 64)
		},
		"source content mismatch": func(c *models.ContractInfo) { c.SourceCode += "\n" },
		"missing contract name":   func(c *models.ContractInfo) { c.ContractName = "" },
		"invalid compiler provenance": func(c *models.ContractInfo) {
			c.CompilerProvenance.ExecutionDigest = strings.Repeat("c", 64)
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			contract := digestBackedContract()
			mutate(&contract)
			if got := ClassifyStoredVerification(contract); got != models.CompilerProvenanceInvalidRecorded {
				t.Fatalf("status = %q, want invalid-recorded", got)
			}
		})
	}
}

func TestClassifyRecordedMetadataUsesLeanDigestSyntaxGate(t *testing.T) {
	current := digestBackedContract()
	current.SourceCode = ""
	current.Imports = nil
	if got := ClassifyRecordedMetadata(current); got != models.CompilerProvenanceDigestBacked {
		t.Fatalf("compact current status = %q, want digest-backed", got)
	}

	legacy := current
	legacy.VerificationRecordSchema = ""
	legacy.CompilerProvenance = nil
	legacy.SourceBundleDigest = ""
	if got := ClassifyRecordedMetadata(legacy); got != models.CompilerProvenanceLegacyUnrecorded {
		t.Fatalf("compact legacy status = %q, want legacy-unrecorded", got)
	}

	for name, mutate := range map[string]func(*models.ContractInfo){
		"schema marker absent":    func(c *models.ContractInfo) { c.VerificationRecordSchema = "" },
		"schema marker unknown":   func(c *models.ContractInfo) { c.VerificationRecordSchema = "future" },
		"missing source digest":   func(c *models.ContractInfo) { c.SourceBundleDigest = "" },
		"malformed source digest": func(c *models.ContractInfo) { c.SourceBundleDigest = Version + ":sha256:nope" },
		"mixed legacy fields":     func(c *models.ContractInfo) { c.CompilerProvenance = nil },
		"invalid provenance": func(c *models.ContractInfo) {
			c.CompilerProvenance.ExecutionDigest = strings.Repeat("c", 64)
		},
	} {
		t.Run(name, func(t *testing.T) {
			contract := current
			provenanceCopy := *current.CompilerProvenance
			provenanceCopy.Components = append([]models.CompilerProvenanceComponent(nil), current.CompilerProvenance.Components...)
			contract.CompilerProvenance = &provenanceCopy
			mutate(&contract)
			if got := ClassifyRecordedMetadata(contract); got != models.CompilerProvenanceInvalidRecorded {
				t.Fatalf("compact status = %q, want invalid-recorded", got)
			}
		})
	}
}

func TestClassifyVerificationRecordBSONNullsFailClosed(t *testing.T) {
	for _, test := range []struct {
		name       string
		document   bson.D
		wantStatus models.CompilerProvenanceStatus
	}{
		{
			name:       "all trust fields absent",
			document:   bson.D{{Key: "verified", Value: true}},
			wantStatus: models.CompilerProvenanceLegacyUnrecorded,
		},
		{
			name: "schema marker explicit null",
			document: bson.D{
				{Key: "verified", Value: true},
				{Key: "verificationRecordSchema", Value: nil},
			},
			wantStatus: models.CompilerProvenanceInvalidRecorded,
		},
		{
			name: "schema absent with explicit null provenance",
			document: bson.D{
				{Key: "verified", Value: true},
				{Key: "compilerProvenance", Value: nil},
			},
			wantStatus: models.CompilerProvenanceInvalidRecorded,
		},
		{
			name: "schema absent with explicit null source digest",
			document: bson.D{
				{Key: "verified", Value: true},
				{Key: "sourceBundleDigest", Value: nil},
			},
			wantStatus: models.CompilerProvenanceInvalidRecorded,
		},
		{
			name: "all trust fields explicit null",
			document: bson.D{
				{Key: "verified", Value: true},
				{Key: "verificationRecordSchema", Value: nil},
				{Key: "compilerProvenance", Value: nil},
				{Key: "sourceBundleDigest", Value: nil},
			},
			wantStatus: models.CompilerProvenanceInvalidRecorded,
		},
		{
			name: "versioned marker with null composite",
			document: bson.D{
				{Key: "verified", Value: true},
				{Key: "verificationRecordSchema", Value: models.VerificationRecordSchemaV1},
				{Key: "compilerProvenance", Value: nil},
				{Key: "sourceBundleDigest", Value: nil},
			},
			wantStatus: models.CompilerProvenanceInvalidRecorded,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			encoded, err := bson.Marshal(test.document)
			if err != nil {
				t.Fatalf("encode BSON: %v", err)
			}
			var contract models.ContractInfo
			if err := bson.Unmarshal(encoded, &contract); err != nil {
				t.Fatalf("decode BSON: %v", err)
			}
			if got := ClassifyStoredVerification(contract); got != test.wantStatus {
				t.Fatalf("stored status = %q, want %q", got, test.wantStatus)
			}
			if got := ClassifyRecordedMetadata(contract); got != test.wantStatus {
				t.Fatalf("metadata status = %q, want %q", got, test.wantStatus)
			}
		})
	}
}
