package models

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestVerificationImportsRoundTripAndLegacyEmpty(t *testing.T) {
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
	contract := ContractInfo{
		ContractAddress:           "Qcurrent",
		Verified:                  true,
		Imports:                   want,
		SourceBundleDigest:        "qrl.verified-source-bundle.v1:sha256:source",
		AIExplanationSourceDigest: "qrl.verified-source-bundle.v1:sha256:ai",
	}
	encodedContract, err := bson.Marshal(contract)
	if err != nil {
		t.Fatalf("encode contract BSON: %v", err)
	}
	var gotContract ContractInfo
	if err := bson.Unmarshal(encodedContract, &gotContract); err != nil {
		t.Fatalf("decode contract BSON: %v", err)
	}
	if !reflect.DeepEqual(gotContract.Imports, want) {
		t.Fatalf("contract imports = %#v, want %#v", gotContract.Imports, want)
	}
	if gotContract.SourceBundleDigest != contract.SourceBundleDigest ||
		gotContract.AIExplanationSourceDigest != contract.AIExplanationSourceDigest {
		t.Fatalf("contract source digests = %q / %q", gotContract.SourceBundleDigest, gotContract.AIExplanationSourceDigest)
	}
	contractJSON, err := json.Marshal(contract)
	if err != nil {
		t.Fatalf("encode contract JSON: %v", err)
	}
	var gotContractJSON ContractInfo
	if err := json.Unmarshal(contractJSON, &gotContractJSON); err != nil {
		t.Fatalf("decode contract JSON: %v", err)
	}
	if !reflect.DeepEqual(gotContractJSON.Imports, want) {
		t.Fatalf("JSON contract imports = %#v, want %#v", gotContractJSON.Imports, want)
	}

	job := ContractVerificationJob{
		JobID:   "multi-file-job",
		Payload: VerificationJobPayload{Imports: want},
		Target: VerificationTarget{
			Address:             "Q" + strings.Repeat("a", 128),
			CreationTransaction: "0x" + strings.Repeat("b", 64),
			CreationBlockNumber: "0x10",
			CreationBlockHash:   "0x" + strings.Repeat("c", 64),
			ChainID:             "0x539",
			DeployedCodeSHA256:  strings.Repeat("d", 64),
		},
	}
	encodedJob, err := json.Marshal(job)
	if err != nil {
		t.Fatalf("encode verification job JSON: %v", err)
	}
	var gotJob ContractVerificationJob
	if err := json.Unmarshal(encodedJob, &gotJob); err != nil {
		t.Fatalf("decode verification job JSON: %v", err)
	}
	if !reflect.DeepEqual(gotJob.Payload.Imports, want) {
		t.Fatalf("job imports = %#v, want %#v", gotJob.Payload.Imports, want)
	}
	if !reflect.DeepEqual(gotJob.Target, job.Target) {
		t.Fatalf("job JSON target = %#v, want %#v", gotJob.Target, job.Target)
	}
	encodedJobBSON, err := bson.Marshal(job)
	if err != nil {
		t.Fatalf("encode verification job BSON: %v", err)
	}
	var gotJobBSON ContractVerificationJob
	if err := bson.Unmarshal(encodedJobBSON, &gotJobBSON); err != nil {
		t.Fatalf("decode verification job BSON: %v", err)
	}
	if !reflect.DeepEqual(gotJobBSON.Payload.Imports, want) {
		t.Fatalf("BSON job imports = %#v, want %#v", gotJobBSON.Payload.Imports, want)
	}
	if !reflect.DeepEqual(gotJobBSON.Target, job.Target) {
		t.Fatalf("job BSON target = %#v, want %#v", gotJobBSON.Target, job.Target)
	}
}
