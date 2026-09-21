package models

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestContractImportsRoundTripAndLegacyEmpty(t *testing.T) {
	legacyBSON, err := bson.Marshal(bson.M{
		"address":  "Qlegacy",
		"verified": true,
	})
	if err != nil {
		t.Fatalf("encode legacy contract BSON: %v", err)
	}
	var legacy ContractInfo
	if err := bson.Unmarshal(legacyBSON, &legacy); err != nil {
		t.Fatalf("decode legacy contract BSON: %v", err)
	}
	if legacy.Imports != nil {
		t.Fatalf("legacy imports = %#v, want nil", legacy.Imports)
	}
	legacyJSON, err := json.Marshal(legacy)
	if err != nil {
		t.Fatalf("encode legacy contract JSON: %v", err)
	}
	if strings.Contains(string(legacyJSON), `"imports"`) {
		t.Fatalf("legacy JSON unexpectedly contains imports: %s", legacyJSON)
	}

	want := map[string]string{
		"interfaces/Registry.hyp": "interface Registry {}",
		"lib/Owned.hyp":           "contract Owned {}",
	}
	encoded, err := bson.Marshal(ContractInfo{
		Address:                   "Qcurrent",
		Verified:                  true,
		VerificationRecordSchema:  VerificationRecordSchemaV1,
		Imports:                   want,
		SourceBundleDigest:        "qrl.verified-source-bundle.v1:sha256:source",
		AIExplanationSourceDigest: "qrl.verified-source-bundle.v1:sha256:ai",
	})
	if err != nil {
		t.Fatalf("encode current contract BSON: %v", err)
	}
	var got ContractInfo
	if err := bson.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("decode current contract BSON: %v", err)
	}
	if !reflect.DeepEqual(got.Imports, want) {
		t.Fatalf("imports = %#v, want %#v", got.Imports, want)
	}
	if got.VerificationRecordSchema != VerificationRecordSchemaV1 || !got.HasVerificationRecordSchema() {
		t.Fatalf("verification record schema = %q", got.VerificationRecordSchema)
	}
	if got.SourceBundleDigest != "qrl.verified-source-bundle.v1:sha256:source" {
		t.Fatalf("source bundle digest = %q", got.SourceBundleDigest)
	}
	if got.AIExplanationSourceDigest != "qrl.verified-source-bundle.v1:sha256:ai" {
		t.Fatalf("AI explanation source digest = %q", got.AIExplanationSourceDigest)
	}
}
