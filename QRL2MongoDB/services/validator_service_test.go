package services

import (
	"context"
	"os"
	"strconv"
	"testing"
	"time"

	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func historyValidator(index int, status string, slashed bool) models.StateValidator {
	return models.StateValidator{Index: strconv.Itoa(index), Balance: "1", Status: status, Validator: models.StateValidatorDetails{
		EffectiveBalance: "18446744073709551615", Slashed: slashed,
		ActivationEligibilityEpoch: "0", ActivationEpoch: "0", ExitEpoch: "18446744073709551615", WithdrawableEpoch: "18446744073709551615",
	}}
}

func TestHistoryBuilderPreservesStatusBucketsAndExactBalances(t *testing.T) {
	statuses := []string{"pending_initialized", "pending_queued", "active_ongoing", "active_exiting", "active_slashed", "exited_unslashed", "exited_slashed", "withdrawal_possible", "withdrawal_done"}
	rows := make([]models.StateValidator, 0)
	for i, status := range statuses {
		rows = append(rows, historyValidator(i, status, false))
	}
	rows = append(rows, historyValidator(9, "withdrawal_done", true))
	rows[0].Validator.ActivationEpoch = "18446744073709551615"
	timing := models.ChainTiming{SlotsPerEpoch: 8, SecondsPerSlot: 60, GenesisTime: 1790582760}
	record, err := BuildValidatorHistory(timing, 5000, rows)
	if err != nil {
		t.Fatal(err)
	}
	if record.Epoch != "5000" || record.EpochInt != 5000 || record.Timestamp != 1792982760 || record.Source != "beacon-state" {
		t.Fatalf("record=%+v", record)
	}
	if record.ValidatorsCount != 10 || record.ActiveCount != 2 || record.PendingCount != 2 || record.ExitedCount != 3 || record.SlashedCount != 3 || record.TotalStaked != "184467440737095516150" {
		t.Fatalf("summary=%+v", record)
	}
	empty, err := BuildValidatorHistory(timing, 0, []models.StateValidator{})
	if err != nil || empty.TotalStaked != "0" || empty.Timestamp != timing.GenesisTime || empty.ValidatorsCount != 0 {
		t.Fatalf("genesis=%+v, %v", empty, err)
	}
	for _, mutate := range []func(*models.StateValidator){
		func(v *models.StateValidator) { v.Status = "bogus" },
		func(v *models.StateValidator) { v.Validator.EffectiveBalance = "-1" },
		func(v *models.StateValidator) { v.Validator.ExitEpoch = "overflow18446744073709551615" },
	} {
		row := historyValidator(0, "active_ongoing", false)
		mutate(&row)
		if _, err := BuildValidatorHistory(timing, 1, []models.StateValidator{row}); err == nil {
			t.Fatal("invalid state accepted")
		}
	}
	if _, err := BuildValidatorHistory(timing, 1, []models.StateValidator{rows[0], rows[0]}); err == nil {
		t.Fatal("duplicate accepted")
	}
}

// Run with an isolated MongoDB to exercise the actual legacy-record upsert.
func TestHistoryUpsertReplacesLegacyRecord(t *testing.T) {
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
	database := client.Database("history_syncer_test_" + strconv.FormatInt(time.Now().UnixNano(), 10))
	defer database.Drop(ctx)
	col := database.Collection("validator_history")
	if _, err := col.InsertOne(ctx, bson.M{"epoch": "1", "timestamp": 1, "validatorsCount": 999, "totalStaked": "999"}); err != nil {
		t.Fatal(err)
	}
	timing := models.ChainTiming{SlotsPerEpoch: 8, SecondsPerSlot: 60, GenesisTime: 1790582760}
	record, err := BuildValidatorHistory(timing, 1, []models.StateValidator{historyValidator(0, "active_ongoing", false)})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err := StoreValidatorHistory(ctx, col, record); err != nil {
			t.Fatal(err)
		}
	}
	count, err := col.CountDocuments(ctx, bson.M{})
	if err != nil || count != 1 {
		t.Fatalf("count=%d err=%v", count, err)
	}
	var got models.ValidatorHistoryRecord
	if err := col.FindOne(ctx, bson.M{"epoch": "1"}).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if got.Source != "beacon-state" || got.ValidatorsCount != 1 || got.ActiveCount != 1 || got.Timestamp != 1790583240 || got.TotalStaked != record.TotalStaked {
		t.Fatalf("replacement=%+v", got)
	}
	previousValidators := configs.ValidatorsCollections
	configs.ValidatorsCollections = database.Collection("validators")
	defer func() { configs.ValidatorsCollections = previousValidators }()
	validator := historyValidator(0, "active_ongoing", false)
	validator.Validator.Pubkey = "0xaabb"
	validator.Validator.WithdrawalRecipient = "0xccdd"
	if err := StoreValidators(ctx, []models.StateValidator{validator}, "5000"); err != nil {
		t.Fatal(err)
	}
	var live models.ValidatorDocument
	if err := configs.ValidatorsCollections.FindOne(ctx, bson.M{"_id": "0"}).Decode(&live); err != nil {
		t.Fatal(err)
	}
	if live.Epoch != "5000" || live.PublicKeyHex != "aabb" || live.WithdrawalCredentialsHex != "ccdd" {
		t.Fatalf("live=%+v", live)
	}
	count, err = col.CountDocuments(ctx, bson.M{})
	if err != nil || count != 1 {
		t.Fatal("live validator write changed history")
	}

}
