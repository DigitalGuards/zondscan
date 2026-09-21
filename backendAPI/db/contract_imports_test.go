package db

import (
	"reflect"
	"testing"

	"backendAPI/models"
	"go.mongodb.org/mongo-driver/bson"
)

func TestContractVerificationSetPersistsCanonicalImports(t *testing.T) {
	imports := map[string]string{
		"interfaces/Registry.hyp": "interface Registry {}",
		"lib/Owned.hyp":           "contract Owned {}",
	}
	fields := contractVerificationSet(models.VerificationResult{
		Imports: imports,
	}, "2026-08-27T00:00:00Z")

	if !reflect.DeepEqual(fields["imports"], imports) {
		t.Fatalf("imports = %#v, want %#v", fields["imports"], imports)
	}
}

func TestContractVerificationSetOmitsLegacyEmptyImports(t *testing.T) {
	fields := contractVerificationSet(models.VerificationResult{}, "2026-08-27T00:00:00Z")
	if _, exists := fields["imports"]; exists {
		t.Fatalf("legacy verification set unexpectedly contains imports: %+v", fields)
	}
}

func TestContractVerificationUpdateClearsStaleImportsAndAIExplanation(t *testing.T) {
	update := contractVerificationUpdate(models.VerificationResult{}, "2026-08-27T00:00:00Z")
	unset, ok := update["$unset"].(bson.M)
	if !ok {
		t.Fatalf("$unset = %#v", update["$unset"])
	}
	for _, field := range []string{"imports", "compilerProvenance", "aiExplanation", "aiExplanationAt", "aiExplanationModel", "aiExplanationSourceDigest"} {
		if _, exists := unset[field]; !exists {
			t.Fatalf("$unset is missing %q: %#v", field, unset)
		}
	}
}

func TestContractVerificationUpdateKeepsCurrentImportsAndInvalidatesAIExplanation(t *testing.T) {
	imports := map[string]string{"lib/Owned.hyp": "contract Owned {}"}
	provenance := &models.CompilerProvenance{BuildID: "current-q128"}
	update := contractVerificationUpdate(models.VerificationResult{
		Imports:            imports,
		CompilerProvenance: provenance,
	}, "2026-08-27T00:00:00Z")
	set, ok := update["$set"].(bson.M)
	if !ok {
		t.Fatalf("$set = %#v", update["$set"])
	}
	if !reflect.DeepEqual(set["imports"], imports) {
		t.Fatalf("persisted imports = %#v, want %#v", set["imports"], imports)
	}
	unset, ok := update["$unset"].(bson.M)
	if !ok {
		t.Fatalf("$unset = %#v", update["$unset"])
	}
	if _, exists := unset["imports"]; exists {
		t.Fatalf("current imports unexpectedly unset: %#v", unset)
	}
	if _, exists := unset["compilerProvenance"]; exists {
		t.Fatalf("current compiler provenance unexpectedly unset: %#v", unset)
	}
	for _, field := range []string{"aiExplanation", "aiExplanationAt", "aiExplanationModel", "aiExplanationSourceDigest"} {
		if _, exists := unset[field]; !exists {
			t.Fatalf("$unset is missing %q: %#v", field, unset)
		}
	}
}
