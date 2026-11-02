package db

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestBlockByHashFilterMatchesCompletedBlocksExactly(t *testing.T) {
	lower := "0x" + "ab12" + "00000000000000000000000000000000000000000000000000000000000c"
	got := blockByHashFilter(lower)
	want := primitive.D{
		{Key: "result.hash", Value: primitive.D{{Key: "$in", Value: primitive.A{lower}}}},
		{Key: "ingestionState", Value: completedBlockIngestionState},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filter = %#v, want %#v", got, want)
	}
}

func TestBlockByHashFilterKeepsLowercaseFirstForMixedCase(t *testing.T) {
	mixed := "0xAB12" + "00000000000000000000000000000000000000000000000000000000000C"
	got := blockByHashFilter(mixed)
	want := primitive.D{
		{Key: "result.hash", Value: primitive.D{{Key: "$in", Value: primitive.A{
			"0xab12" + "00000000000000000000000000000000000000000000000000000000000c",
			mixed,
		}}}},
		{Key: "ingestionState", Value: completedBlockIngestionState},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("filter = %#v, want %#v", got, want)
	}
}
