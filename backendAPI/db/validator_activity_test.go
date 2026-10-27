package db

import (
	"backendAPI/models"
	"context"
	"errors"
	"reflect"
	"testing"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type activityCollectionStub struct {
	countFilter, findFilter interface{}
	findOptions             *options.FindOptions
	rows                    []interface{}
	err                     error
}

func (s *activityCollectionStub) CountDocuments(_ context.Context, filter interface{}, _ ...*options.CountOptions) (int64, error) {
	s.countFilter = filter
	return 3, s.err
}
func (s *activityCollectionStub) Find(_ context.Context, filter interface{}, opts ...*options.FindOptions) (*mongo.Cursor, error) {
	s.findFilter, s.findOptions = filter, opts[0]
	return mongo.NewCursorFromDocuments(s.rows, nil, nil)
}

func TestValidatorActivityFilteredPagination(t *testing.T) {
	from, to := int64(2), int64(11)
	q := models.ValidatorActivityQuery{Page: 2, Limit: 1, FromEpoch: &from, ToEpoch: &to, Kind: "ejected", ValidatorIndex: "0"}
	s := &activityCollectionStub{rows: []interface{}{models.ValidatorSetChange{ID: "7:ejected:0", Epoch: 7, Kind: "ejected", ValidatorIndex: "0", Pubkey: "0xaabb"}}}
	rows, total, err := activityPage[models.ValidatorSetChange](context.Background(), s, q, false)
	if err != nil || total != 3 || len(rows) != 1 || rows[0].Epoch != 7 {
		t.Fatalf("%v %d %v", rows, total, err)
	}
	want := bson.M{"epoch": bson.M{"$gte": int64(2), "$lte": int64(11)}, "kind": "ejected", "validatorIndex": "0"}
	if !reflect.DeepEqual(s.countFilter, want) || !reflect.DeepEqual(s.findFilter, want) {
		t.Fatalf("count=%v find=%v", s.countFilter, s.findFilter)
	}
	if *s.findOptions.Skip != 1 || *s.findOptions.Limit != 1 || !reflect.DeepEqual(s.findOptions.Sort, bson.D{{Key: "epoch", Value: -1}, {Key: "_id", Value: -1}}) {
		t.Fatal(s.findOptions)
	}
}

func TestSlashingPaginationOrderEmptyAndFailure(t *testing.T) {
	s := &activityCollectionStub{}
	rows, _, err := activityPage[models.SlashingEvent](context.Background(), s, models.ValidatorActivityQuery{Page: 1, Limit: 20, ValidatorIndex: "0"}, true)
	if err != nil || rows == nil || len(rows) != 0 {
		t.Fatalf("%v %v", rows, err)
	}
	if !reflect.DeepEqual(s.findFilter, bson.M{"validatorIndices": "0"}) || !reflect.DeepEqual(s.findOptions.Sort, bson.D{{Key: "slot", Value: -1}, {Key: "_id", Value: -1}}) {
		t.Fatal(s.findFilter, s.findOptions)
	}
	s.err = errors.New("unavailable")
	if _, _, err := activityPage[models.SlashingEvent](context.Background(), s, models.ValidatorActivityQuery{Page: 1, Limit: 20}, true); !errors.Is(err, s.err) {
		t.Fatal(err)
	}
}
