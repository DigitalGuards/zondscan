package db

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
)

func TestContractRollbackFilterMatchesEveryCreationIdentity(t *testing.T) {
	got := contractRollbackFilter(
		[]string{"0x10", "0x11"},
		[]string{"0xblock1", "0xblock2"},
		[]string{"0xtx1", "0xtx2"},
		[]string{"Qcontract1", "Qcontract2"},
	)
	want := bson.M{"$or": bson.A{
		bson.M{"creationBlockNumber": bson.M{"$in": []string{"0x10", "0x11"}}},
		bson.M{"creationBlockHash": bson.M{"$in": []string{"0xblock1", "0xblock2"}}},
		bson.M{"creationTransaction": bson.M{"$in": []string{"0xtx1", "0xtx2"}}},
		bson.M{"address": bson.M{"$in": []string{"Qcontract1", "Qcontract2"}}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("contract rollback filter = %#v, want %#v", got, want)
	}
}

func TestContractRollbackFilterRejectsEmptyScope(t *testing.T) {
	if got := contractRollbackFilter(nil, nil, nil, nil); got != nil {
		t.Fatalf("empty rollback filter = %#v, want nil", got)
	}
}

func TestVerificationJobRollbackFilterFencesEveryCreationIdentity(t *testing.T) {
	got := verificationJobRollbackFilter(
		[]string{"0x10"},
		[]string{"0xblock"},
		[]string{"0xtx"},
		[]string{"Qcontract"},
	)
	want := bson.M{
		"status": bson.M{"$in": []string{"pending", "compiling", "success"}},
		"$or": bson.A{
			bson.M{"target.creationBlockNumber": bson.M{"$in": []string{"0x10"}}},
			bson.M{"target.creationBlockHash": bson.M{"$in": []string{"0xblock"}}},
			bson.M{"target.creationTransaction": bson.M{"$in": []string{"0xtx"}}},
			bson.M{"target.address": bson.M{"$in": []string{"Qcontract"}}},
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("verification job rollback filter = %#v, want %#v", got, want)
	}
}

func TestVerificationJobRollbackFilterRejectsEmptyScope(t *testing.T) {
	if got := verificationJobRollbackFilter(nil, nil, nil, nil); got != nil {
		t.Fatalf("empty verification job rollback filter = %#v, want nil", got)
	}
}

func TestOrphanedContractAddressFilterUsesCanonicalAddressKey(t *testing.T) {
	got := orphanedContractAddressFilter([]string{"Qcontract"})
	want := bson.M{"id": bson.M{"$in": []string{"Qcontract"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("orphaned address filter = %#v, want %#v", got, want)
	}
	if _, wrongKey := got["address"]; wrongKey {
		t.Fatalf("orphaned address filter uses non-schema key: %#v", got)
	}
}

func TestAppendStringValuesDropsNonStringsEmptyAndDuplicates(t *testing.T) {
	got := appendStringValues(
		[]string{"Qexisting"},
		[]interface{}{"Qnew", "", 7, "Qexisting", "Qnew"},
	)
	want := []string{"Qexisting", "Qnew"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("appended values = %#v, want %#v", got, want)
	}
}

func TestRollbackSyncStateMutationIsUnconditionalExactTarget(t *testing.T) {
	filter, update := rollbackSyncStateMutation("0x10", 16)
	wantFilter := bson.M{"_id": LastSyncedBlockID}
	wantUpdate := bson.M{"$set": bson.M{
		"block_number":     "0x10",
		"block_number_int": int64(16),
	}}
	if !reflect.DeepEqual(filter, wantFilter) {
		t.Fatalf("sync-state filter = %#v, want %#v", filter, wantFilter)
	}
	if !reflect.DeepEqual(update, wantUpdate) {
		t.Fatalf("sync-state update = %#v, want %#v", update, wantUpdate)
	}
	if _, hasMonotonicGuard := filter["$or"]; hasMonotonicGuard {
		t.Fatalf("rollback sync-state filter retained a monotonic guard: %#v", filter)
	}
}
