package synchroniser

import (
	"QRL2MongoDB/db"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestTokenQueriesExcludePendingBlocks(t *testing.T) {
	rangeFilter := completedBlockRangeFilter(1, 3)
	if got := rangeFilter["ingestionState"]; got != db.BlockIngestionComplete {
		t.Fatalf("range ingestionState = %v, want %q", got, db.BlockIngestionComplete)
	}
	if got, ok := rangeFilter["result.transactions.0"].(bson.M); !ok || got["$exists"] != true {
		t.Fatalf("range transaction filter = %#v, want existence check", rangeFilter["result.transactions.0"])
	}

	identityFilter := completedBlockIdentityFilter("0x2")
	if got := identityFilter["ingestionState"]; got != db.BlockIngestionComplete {
		t.Fatalf("identity ingestionState = %v, want %q", got, db.BlockIngestionComplete)
	}
	if got := identityFilter["result.number"]; got != "0x2" {
		t.Fatalf("identity block number = %v, want 0x2", got)
	}
}
