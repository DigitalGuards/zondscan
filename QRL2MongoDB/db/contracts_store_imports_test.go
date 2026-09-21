package db

import (
	"testing"

	"QRL2MongoDB/models"
)

func TestSyncerOwnedSetCannotOverwriteVerificationImports(t *testing.T) {
	fields := syncerOwnedSet(models.ContractInfo{
		Address:                   "Qcontract",
		VerificationRecordSchema:  models.VerificationRecordSchemaV1,
		Imports:                   map[string]string{"lib/Owned.hyp": "contract Owned {}"},
		SourceBundleDigest:        "qrl.verified-source-bundle.v1:sha256:source",
		AIExplanationSourceDigest: "qrl.verified-source-bundle.v1:sha256:ai",
	})
	for _, field := range []string{"verificationRecordSchema", "imports", "sourceBundleDigest", "aiExplanationSourceDigest"} {
		if _, exists := fields[field]; exists {
			t.Fatalf("syncer-owned update unexpectedly contains %q: %+v", field, fields)
		}
	}
}

func TestSyncerOwnedSetPersistsCreatorAddressProvenance(t *testing.T) {
	fields := syncerOwnedSet(models.ContractInfo{
		Address:                  "Qcontract",
		CreatorAddress:           "Qcreator",
		CreatorAddressProvenance: models.CreatorAddressProvenanceDirectDeployment,
	})
	if got := fields["creatorAddressProvenance"]; got != models.CreatorAddressProvenanceDirectDeployment {
		t.Fatalf("creatorAddressProvenance = %#v", got)
	}
}

func TestSyncerOwnedSetPersistsVerificationTargetIdentity(t *testing.T) {
	fields := syncerOwnedSet(models.ContractInfo{
		Address:             "Qcontract",
		ContractCodeSHA256:  "code-digest",
		CreationTransaction: "0xtx",
		CreationBlockNumber: "0x10",
		CreationBlockHash:   "0xblock",
		ChainID:             "0x539",
	})
	for field, want := range map[string]string{
		"contractCodeSha256":  "code-digest",
		"creationTransaction": "0xtx",
		"creationBlockNumber": "0x10",
		"creationBlockHash":   "0xblock",
		"chainId":             "0x539",
	} {
		if got := fields[field]; got != want {
			t.Fatalf("%s = %#v, want %q", field, got, want)
		}
	}
}

func TestMergeCreationEvidencePromotesAuthoritativeSources(t *testing.T) {
	existing := models.ContractInfo{
		CreatorAddress:           "Qminter",
		CreatorAddressProvenance: models.CreatorAddressProvenanceMintHeuristic,
		CreationTransaction:      "mint-tx",
		CreationBlockNumber:      "0x20",
		CreationBlockHash:        "0xold-block",
		ChainID:                  "0x1",
	}
	mergeCreationEvidence(&existing, models.ContractInfo{
		CreatorAddress:           "Qdeployer",
		CreatorAddressProvenance: models.CreatorAddressProvenanceCreateTraceOuter,
		CreationTransaction:      "deploy-tx",
		CreationBlockNumber:      "0x10",
		CreationBlockHash:        "0xnew-block",
		ChainID:                  "0x539",
	})
	if existing.CreatorAddress != "Qdeployer" ||
		existing.CreatorAddressProvenance != models.CreatorAddressProvenanceCreateTraceOuter ||
		existing.CreationTransaction != "deploy-tx" || existing.CreationBlockNumber != "0x10" ||
		existing.CreationBlockHash != "0xnew-block" || existing.ChainID != "0x539" {
		t.Fatalf("promoted creation evidence = %#v", existing)
	}

	mergeCreationEvidence(&existing, models.ContractInfo{
		CreatorAddress:           "Qattacker",
		CreatorAddressProvenance: models.CreatorAddressProvenanceMintHeuristic,
		CreationTransaction:      "later-mint",
		CreationBlockNumber:      "0x30",
	})
	if existing.CreatorAddress != "Qdeployer" || existing.CreationTransaction != "deploy-tx" {
		t.Fatalf("heuristic overwrote authoritative evidence: %#v", existing)
	}
}
