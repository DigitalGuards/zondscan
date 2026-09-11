package configs

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestExplainChallengeIndexIsImmediateTTL(t *testing.T) {
	indexes := explainChallengeIndexModels()
	if len(indexes) != 1 {
		t.Fatalf("index count = %d, want 1", len(indexes))
	}
	if !reflect.DeepEqual(indexes[0].Keys, bson.D{{Key: "expiresAt", Value: 1}}) {
		t.Fatalf("index keys = %#v", indexes[0].Keys)
	}
	options := indexes[0].Options
	if options.Name == nil || *options.Name != "contract_explain_challenge_expiry" {
		t.Fatalf("index name = %#v", options.Name)
	}
	if options.ExpireAfterSeconds == nil || *options.ExpireAfterSeconds != 0 {
		t.Fatalf("expireAfterSeconds = %#v, want 0", options.ExpireAfterSeconds)
	}
}

func TestExplainUsageIndexIsImmediateTTL(t *testing.T) {
	indexes := explainUsageIndexModels()
	if len(indexes) != 1 {
		t.Fatalf("index count = %d, want 1", len(indexes))
	}
	if !reflect.DeepEqual(indexes[0].Keys, bson.D{{Key: "expiresAt", Value: 1}}) {
		t.Fatalf("index keys = %#v", indexes[0].Keys)
	}
	indexOptions := indexes[0].Options
	if indexOptions.Name == nil || *indexOptions.Name != "contract_explain_provider_usage_expiry" {
		t.Fatalf("index name = %#v", indexOptions.Name)
	}
	if indexOptions.ExpireAfterSeconds == nil || *indexOptions.ExpireAfterSeconds != 0 {
		t.Fatalf("expireAfterSeconds = %#v, want 0", indexOptions.ExpireAfterSeconds)
	}
}

func TestValidateExplainChallengeTTLIndexDocuments(t *testing.T) {
	valid := bson.M{
		"name":               "contract_explain_challenge_expiry",
		"key":                bson.M{"expiresAt": int32(1)},
		"expireAfterSeconds": int64(0),
	}
	if err := validateExplainChallengeTTLIndexDocuments([]bson.M{valid}); err != nil {
		t.Fatalf("valid TTL index rejected: %v", err)
	}

	tests := []struct {
		name    string
		indexes []bson.M
	}{
		{name: "missing", indexes: []bson.M{{"name": "_id_"}}},
		{
			name: "wrong field",
			indexes: []bson.M{{
				"name":               "contract_explain_challenge_expiry",
				"key":                bson.M{"issuedAt": int32(1)},
				"expireAfterSeconds": int64(0),
			}},
		},
		{
			name: "wrong expiry",
			indexes: []bson.M{{
				"name":               "contract_explain_challenge_expiry",
				"key":                bson.D{{Key: "expiresAt", Value: int32(1)}},
				"expireAfterSeconds": int64(60),
			}},
		},
		{
			name: "partial index",
			indexes: []bson.M{{
				"name":                    "contract_explain_challenge_expiry",
				"key":                     bson.M{"expiresAt": int32(1)},
				"expireAfterSeconds":      int64(0),
				"partialFilterExpression": bson.M{"action": "other"},
			}},
		},
		{
			name: "unique index",
			indexes: []bson.M{{
				"name":               "contract_explain_challenge_expiry",
				"key":                bson.M{"expiresAt": int32(1)},
				"expireAfterSeconds": int64(0),
				"unique":             true,
			}},
		},
		{
			name: "sparse index",
			indexes: []bson.M{{
				"name":               "contract_explain_challenge_expiry",
				"key":                bson.M{"expiresAt": int32(1)},
				"expireAfterSeconds": int64(0),
				"sparse":             true,
			}},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateExplainChallengeTTLIndexDocuments(test.indexes); err == nil {
				t.Fatal("invalid TTL index accepted")
			}
		})
	}
}
