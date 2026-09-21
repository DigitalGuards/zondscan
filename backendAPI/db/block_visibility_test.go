package db

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestCompletedBlockFilterRequiresDurableCompanions(t *testing.T) {
	base := primitive.D{{Key: "blockNumberInt", Value: int64(7)}}
	got := completedBlockFilter(base)
	want := primitive.D{
		{Key: "blockNumberInt", Value: int64(7)},
		{Key: "ingestionState", Value: "complete"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("completed block filter = %#v, want %#v", got, want)
	}
	if len(base) != 1 {
		t.Fatalf("helper mutated caller filter: %#v", base)
	}
}

func TestCompletedBlockFilterRejectsMarkerlessCompatibility(t *testing.T) {
	filter := completedBlockFilter(nil)
	if len(filter) != 1 || filter[0].Key != "ingestionState" || filter[0].Value != "complete" {
		t.Fatalf("visibility filter permits non-complete rows: %#v", filter)
	}
}

func TestBlockIngestionReadinessRejectsMarkerlessAndUnknownStates(t *testing.T) {
	got := blockIngestionReadinessFilter()
	want := primitive.D{{
		Key: "ingestionState", Value: primitive.D{{
			Key: "$nin", Value: primitive.A{"pending", "complete"},
		}},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("readiness filter = %#v, want %#v", got, want)
	}
}

func TestBlockIngestionReadinessRequiresUniqueHeightIndex(t *testing.T) {
	unique := true
	nonUnique := false
	if hasUniqueBlockHeightIndex([]*mongo.IndexSpecification{{
		Name:   blockHeightUniqueIndexName,
		Unique: &nonUnique,
	}}) {
		t.Fatal("non-unique block-height index was accepted")
	}
	if !hasUniqueBlockHeightIndex([]*mongo.IndexSpecification{{
		Name:   blockHeightUniqueIndexName,
		Unique: &unique,
	}}) {
		t.Fatal("unique block-height index was rejected")
	}
}
