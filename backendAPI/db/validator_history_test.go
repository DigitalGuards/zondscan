package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"encoding/json"
	"os"
	"strconv"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestEpochSummaryIgnoresUnmarkedRecords(t *testing.T) {
	var detail models.EpochDetailResponse
	applyEpochSummary(&detail, models.ValidatorHistoryRecord{ValidatorsCount: 64, ActiveCount: 64, TotalStaked: "2560000000000000"})
	if detail.SummaryAvailable || detail.ValidatorsCount != nil || detail.ActiveCount != nil || detail.TotalStaked != nil {
		t.Fatal("legacy summary exposed")
	}
	body, err := json.Marshal(detail)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(body, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"validatorsCount", "activeCount", "pendingCount", "exitedCount", "slashedCount", "totalStaked"} {
		if fields[key] != nil {
			t.Fatalf("%s is not null", key)
		}
	}
	applyEpochSummary(&detail, models.ValidatorHistoryRecord{Source: models.BeaconStateHistorySource, TotalStaked: "0"})
	if !detail.SummaryAvailable || detail.ValidatorsCount == nil || *detail.ValidatorsCount != 0 || *detail.TotalStaked != "0" {
		t.Fatal("known zero lost")
	}
}

func TestEpochMappingUsesNetworkTiming(t *testing.T) {
	timing := models.ChainTiming{SlotsPerEpoch: 4, SecondsPerSlot: 12, GenesisTime: 1790582760}
	rows, proposed, err := mapEpochSlots(5000, timing, []epochExecutionBlock{{BlockNumberInt: 123, Result: models.Result{Hash: "0xaa", Timestamp: "1790822784"}}}, nil)
	if err != nil || len(rows) != 4 || proposed != 1 || rows[2].BlockNumber != "123" || rows[2].Slot != 20002 {
		t.Fatalf("rows=%+v proposed=%d err=%v", rows, proposed, err)
	}
}

func TestHistoryQueriesHideLegacyRecords(t *testing.T) {
	uri := os.Getenv("VALIDATOR_HISTORY_TEST_MONGOURI")
	if uri == "" {
		t.Skip("set VALIDATOR_HISTORY_TEST_MONGOURI for isolated MongoDB checks")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(ctx)
	database := client.Database("history_backend_test_" + strconv.FormatInt(time.Now().UnixNano(), 10))
	defer database.Drop(ctx)
	oldSync, oldInfo, oldHistory, oldBlocks, oldSlots := configs.SyncStateCollection, configs.EpochInfoCollection, configs.ValidatorHistoryCollection, configs.BlocksCollection, configs.SlotAssignmentsCollection
	defer func() {
		configs.SyncStateCollection, configs.EpochInfoCollection, configs.ValidatorHistoryCollection, configs.BlocksCollection, configs.SlotAssignmentsCollection = oldSync, oldInfo, oldHistory, oldBlocks, oldSlots
	}()
	configs.SyncStateCollection = database.Collection("sync_state")
	configs.EpochInfoCollection = database.Collection("epoch_info")
	configs.ValidatorHistoryCollection = database.Collection("validator_history")
	configs.BlocksCollection = database.Collection("blocks")
	configs.SlotAssignmentsCollection = database.Collection("slotAssignments")
	if _, err := configs.SyncStateCollection.InsertOne(ctx, bson.M{"_id": "chain_timing", "slotsPerEpoch": 8, "secondsPerSlot": 60, "genesisTime": 1790582760}); err != nil {
		t.Fatal(err)
	}
	if _, err := configs.EpochInfoCollection.InsertOne(ctx, bson.M{"_id": "current", "headSlot": "40007", "headEpoch": "309", "finalizedEpoch": "4998", "justifiedEpoch": "4999"}); err != nil {
		t.Fatal(err)
	}
	for _, record := range []bson.M{
		{"epoch": "309", "epochInt": 309, "validatorsCount": 999, "totalStaked": "999"},
		{"epoch": "4998", "epochInt": 4998, "source": "beacon-state", "validatorsCount": 64, "activeCount": 64, "totalStaked": "2560000000000000"},
		{"epoch": "9", "epochInt": 9, "source": "beacon-state", "validatorsCount": 0, "totalStaked": "0"},
	} {
		if _, err := configs.ValidatorHistoryCollection.InsertOne(ctx, record); err != nil {
			t.Fatal(err)
		}
	}
	epochs, err := GetEpochs(1, 15)
	if err != nil {
		t.Fatal(err)
	}
	if epochs.Total != 2 || len(epochs.Epochs) != 2 || epochs.Epochs[0].Epoch != "4998" || epochs.Epochs[1].Epoch != "9" {
		t.Fatalf("epochs=%+v", epochs)
	}
	history, err := GetValidatorHistory(10)
	if err != nil {
		t.Fatal(err)
	}
	if len(history.History) != 2 || history.History[0].Epoch != "4998" {
		t.Fatalf("history=%+v", history)
	}
	detail, err := GetEpochDetail("0309")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Epoch != "309" || detail.SummaryAvailable || detail.ValidatorsCount != nil || detail.TotalStaked != nil || detail.Timestamp != 1790731080 {
		t.Fatalf("detail=%+v", detail)
	}
	info, err := GetEpochInfo()
	if err != nil {
		t.Fatal(err)
	}
	if info.HeadEpoch != "5000" || info.SlotsPerEpoch != 8 || info.GenesisTime != 1790582760 || info.TimeToNextEpoch != 60 {
		t.Fatal(info)
	}
	stats, err := BlockStatsWithTiming(ctx, nil)
	if err != nil || stats.AvgBlockTimeSec != 60 {
		t.Fatalf("block clock=%+v %v", stats, err)
	}
	oldValidators := configs.ValidatorsCollections
	configs.ValidatorsCollections = database.Collection("validators")
	defer func() { configs.ValidatorsCollections = oldValidators }()
	for i, epochs := range [][2]string{{"0", FAR_FUTURE_EPOCH}, {"10000", FAR_FUTURE_EPOCH}, {"0", "900"}} {
		_, err := configs.ValidatorsCollections.InsertOne(ctx, bson.M{"_id": strconv.Itoa(i), "activationEpoch": epochs[0], "exitEpoch": epochs[1], "effectiveBalance": "40000000000000", "slashed": false})
		if err != nil {
			t.Fatal(err)
		}
	}
	validatorStats, err := GetValidatorStats()
	if err != nil {
		t.Fatal(err)
	}
	if validatorStats.CurrentEpoch != "5000" || validatorStats.ActiveCount != 1 || validatorStats.PendingCount != 1 || validatorStats.ExitedCount != 1 {
		t.Fatalf("stats=%+v", validatorStats)
	}
	validators, err := ReturnValidators("")
	if err != nil {
		t.Fatal(err)
	}
	if validators.Epoch != "5000" || validators.Validators[0].Age != 5000 {
		t.Fatalf("validators=%+v", validators)
	}
	validator, err := GetValidatorByID("0")
	if err != nil {
		t.Fatal(err)
	}
	if validator.CurrentEpoch != "5000" || validator.Age != 5000 {
		t.Fatalf("validator=%+v", validator)
	}

}
