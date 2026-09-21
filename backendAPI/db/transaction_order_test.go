package db

import (
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestSortedCanonicalTransactionsPipeline(t *testing.T) {
	pipeline := sortedCanonicalTransactionsPipeline()
	want := append(
		[]bson.M{{"$sort": primitive.D{{Key: "timeStamp", Value: -1}}}},
		canonicalCompanionPipeline(primitive.D{})...,
	)
	if !reflect.DeepEqual(pipeline, want) {
		t.Fatalf("transaction list pipeline = %#v, want indexed sort followed by unchanged canonical fence %#v", pipeline, want)
	}
	for _, stage := range pipeline {
		if _, exists := stage["$skip"]; exists {
			t.Fatal("pagination must follow the complete canonical fence")
		}
		if _, exists := stage["$limit"]; exists {
			t.Fatal("limit must follow the complete canonical fence")
		}
	}
}

func TestSortedCanonicalTransactionsPipelineDoesNotChangeCountFence(t *testing.T) {
	before := canonicalCompanionPipeline(primitive.D{})
	list := sortedCanonicalTransactionsPipeline()
	list[0]["$sort"] = bson.M{"timeStamp": 1}
	list[1]["$match"] = bson.M{"txHash": "0xignored"}
	after := canonicalCompanionPipeline(primitive.D{})
	if !reflect.DeepEqual(before, after) {
		t.Fatal("list pipeline mutation changed the canonical count pipeline")
	}
	if _, exists := after[0]["$sort"]; exists {
		t.Fatal("canonical exact counts must retain their existing unsorted fence")
	}
}
