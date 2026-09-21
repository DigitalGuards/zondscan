package db

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type countCollectionStub struct {
	count         int64
	estimate      int64
	err           error
	ctx           context.Context
	filter        interface{}
	options       []*options.CountOptions
	countCalls    int
	estimateCalls int
}

func (s *countCollectionStub) CountDocuments(ctx context.Context, filter interface{}, opts ...*options.CountOptions) (int64, error) {
	s.countCalls++
	s.ctx, s.filter, s.options = ctx, filter, opts
	return s.count, s.err
}

func (s *countCollectionStub) EstimatedDocumentCount(context.Context, ...*options.EstimatedDocumentCountOptions) (int64, error) {
	s.estimateCalls++
	return s.estimate, nil
}

func TestCountDocumentsResilientUsesExactIDIndexCount(t *testing.T) {
	for _, test := range []struct {
		name     string
		estimate int64
		count    int64
	}{
		{name: "stale positive estimate", estimate: 2211, count: 3234},
		{name: "stale zero estimate", estimate: 0, count: 3234},
		{name: "stale overestimate", estimate: 3234, count: 2211},
		{name: "empty collection", estimate: 0, count: 0},
	} {
		t.Run(test.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			collection := &countCollectionStub{count: test.count, estimate: test.estimate}
			got, err := countDocumentsResilient(ctx, collection)
			if err != nil || got != test.count {
				t.Fatalf("count = %d, %v; want %d, nil", got, err, test.count)
			}
			if collection.countCalls != 1 || collection.estimateCalls != 0 {
				t.Fatalf("exact calls = %d, estimate calls = %d", collection.countCalls, collection.estimateCalls)
			}
			if collection.ctx != ctx || !reflect.DeepEqual(collection.filter, primitive.D{}) {
				t.Fatalf("context or unfiltered count changed: %#v", collection.filter)
			}
			if len(collection.options) != 1 || collection.options[0].Hint != "_id_" {
				t.Fatalf("count options = %#v; want _id_ hint", collection.options)
			}
		})
	}
}

func TestCountDocumentsResilientPreservesCountErrors(t *testing.T) {
	for _, countErr := range []error{errors.New("count unavailable"), context.DeadlineExceeded, context.Canceled} {
		collection := &countCollectionStub{estimate: 2211, err: countErr}
		got, err := countDocumentsResilient(context.Background(), collection)
		if got != 0 || !errors.Is(err, countErr) {
			t.Fatalf("count = %d, %v; want 0, %v", got, err, countErr)
		}
		if collection.countCalls != 1 || collection.estimateCalls != 0 {
			t.Fatalf("exact calls = %d, estimate calls = %d", collection.countCalls, collection.estimateCalls)
		}
	}
}
