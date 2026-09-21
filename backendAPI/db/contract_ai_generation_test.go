package db

import (
	"testing"
	"time"

	"backendAPI/models"

	"go.mongodb.org/mongo-driver/bson"
)

func TestExplanationLeaseAcquisitionFilterBindsVerificationGeneration(t *testing.T) {
	now := time.Date(2026, 8, 27, 23, 59, 59, 0, time.UTC)
	expected := models.ContractInfo{
		Verified:                 true,
		VerificationRecordSchema: models.VerificationRecordSchemaV1,
		VerifiedAt:               "2026-08-27T01:02:03.000000004Z",
		SourceCode:               "contract Primary {}",
		ContractName:             "Primary",
		SourceBundleDigest:       "source-digest",
	}
	filter := explanationLeaseAcquisitionFilter("Qcontract", expected, "source-digest", now, true)
	for key, want := range map[string]any{
		"address":                  "Qcontract",
		"verified":                 true,
		"verificationRecordSchema": models.VerificationRecordSchemaV1,
		"verifiedAt":               expected.VerifiedAt,
		"sourceCode":               expected.SourceCode,
		"contractName":             expected.ContractName,
		"sourceBundleDigest":       "source-digest",
	} {
		if got := filter[key]; got != want {
			t.Fatalf("filter[%q] = %#v, want %#v", key, got, want)
		}
	}
	conditions, ok := filter["$and"].(bson.A)
	if !ok || len(conditions) != 2 {
		t.Fatalf("lease and cache conditions = %#v", filter["$and"])
	}
	leaseCondition, ok := conditions[0].(bson.M)
	if !ok {
		t.Fatalf("lease condition = %#v", conditions[0])
	}
	branches, ok := leaseCondition["$or"].(bson.A)
	if !ok || len(branches) != 4 {
		t.Fatalf("lease availability branches = %#v", leaseCondition["$or"])
	}
	expiryBranch, ok := branches[3].(bson.M)
	if !ok {
		t.Fatalf("expiry branch = %#v", branches[3])
	}
	expiryCondition, ok := expiryBranch["aiExplanationLease.expiresAt"].(bson.M)
	if !ok || expiryCondition["$lte"] != now {
		t.Fatalf("expiry condition = %#v", expiryBranch)
	}
	cacheCondition, ok := conditions[1].(bson.M)
	if !ok {
		t.Fatalf("cache condition = %#v", conditions[1])
	}
	cacheBranches, ok := cacheCondition["$or"].(bson.A)
	if !ok || len(cacheBranches) != 3 {
		t.Fatalf("cache-miss branches = %#v", cacheCondition["$or"])
	}

	regenerationFilter := explanationLeaseAcquisitionFilter(
		"Qcontract",
		expected,
		"source-digest",
		now,
		false,
	)
	if _, ok := regenerationFilter["$and"]; ok {
		t.Fatalf("regeneration unexpectedly requires cache miss: %#v", regenerationFilter)
	}
	if _, ok := regenerationFilter["$or"]; !ok {
		t.Fatalf("regeneration lease availability is missing: %#v", regenerationFilter)
	}
}

func TestExplanationSaveFilterRequiresMatchingLeaseIdentity(t *testing.T) {
	expected := models.ContractInfo{
		Verified:                 true,
		VerificationRecordSchema: models.VerificationRecordSchemaV1,
		VerifiedAt:               "2026-08-27T01:02:03Z",
		SourceCode:               "contract Primary {}",
		ContractName:             "Primary",
		SourceBundleDigest:       "digest",
	}
	lease := models.AIExplanationLease{
		Token:        "lease-token",
		SourceDigest: "digest",
		VerifiedAt:   expected.VerifiedAt,
	}
	filter := explanationSaveFilter("Qcontract", expected, "digest", lease)
	for key, want := range map[string]string{
		"aiExplanationLease.token":        lease.Token,
		"aiExplanationLease.sourceDigest": lease.SourceDigest,
		"aiExplanationLease.verifiedAt":   lease.VerifiedAt,
	} {
		if got := filter[key]; got != want {
			t.Fatalf("filter[%q] = %#v, want %q", key, got, want)
		}
	}
	update := explanationSaveUpdate("explanation", "model", "generated-at", "digest")
	unset, ok := update["$unset"].(bson.M)
	if !ok {
		t.Fatalf("$unset = %#v", update["$unset"])
	}
	if _, ok := unset["aiExplanationLease"]; !ok {
		t.Fatalf("successful save does not clear lease: %#v", unset)
	}
}

func TestReverificationClearsAIExplanationLease(t *testing.T) {
	update := contractVerificationUpdate(models.VerificationResult{}, "2026-08-27T00:00:00Z")
	unset, ok := update["$unset"].(bson.M)
	if !ok {
		t.Fatalf("$unset = %#v", update["$unset"])
	}
	if _, ok := unset["aiExplanationLease"]; !ok {
		t.Fatalf("re-verification does not clear lease: %#v", unset)
	}
}

func TestUTCProviderBudgetWindowAndFilter(t *testing.T) {
	now := time.Date(2026, 8, 27, 23, 59, 59, 500_000_000, time.FixedZone("west", -7*60*60))
	start, next := utcProviderBudgetWindow(now)
	if start.Location() != time.UTC || start.Hour() != 0 || start.Minute() != 0 || start.Second() != 0 {
		t.Fatalf("start = %s", start)
	}
	if next.Sub(start) != 24*time.Hour {
		t.Fatalf("window length = %s", next.Sub(start))
	}
	if got := providerBudgetDocumentID(start); got != "provider-calls:2026-08-28" {
		t.Fatalf("document ID = %q", got)
	}
	filter := providerBudgetReservationFilter("provider-calls:2026-08-28", 100)
	if filter["_id"] != "provider-calls:2026-08-28" {
		t.Fatalf("filter _id = %#v", filter["_id"])
	}
	if filter["limit"] != 100 {
		t.Fatalf("filter limit = %#v", filter["limit"])
	}
	count, ok := filter["count"].(bson.M)
	if !ok || count["$lt"] != 100 {
		t.Fatalf("count filter = %#v", filter["count"])
	}
}
