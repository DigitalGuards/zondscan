package routes

import (
	"backendAPI/cache"
	"backendAPI/configs"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestGasAndPendingETARoutesWithOptionalTiming(t *testing.T) {
	uri := os.Getenv("VALIDATOR_HISTORY_TEST_MONGOURI")
	if uri == "" {
		t.Skip("set VALIDATOR_HISTORY_TEST_MONGOURI for isolated MongoDB checks")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Disconnect(ctx)
	database := client.Database(fmt.Sprintf("gas_timing_routes_test_%d", time.Now().UnixNano()))
	defer database.Drop(ctx)
	oldBlocks, oldSync, oldPending, oldPrice, oldCache := configs.BlocksCollection, configs.SyncStateCollection, configs.PendingTransactionsCollection, configs.CoinGeckoCollection, routeCache
	defer func() {
		configs.BlocksCollection, configs.SyncStateCollection, configs.PendingTransactionsCollection, configs.CoinGeckoCollection, routeCache = oldBlocks, oldSync, oldPending, oldPrice, oldCache
	}()
	configs.BlocksCollection = database.Collection("blocks")
	configs.SyncStateCollection = database.Collection("sync_state")
	configs.PendingTransactionsCollection = database.Collection("pending_transactions")
	configs.CoinGeckoCollection = database.Collection("coingecko")
	hash := "0x" + strings.Repeat("a", 64)
	if _, err := configs.PendingTransactionsCollection.InsertOne(ctx, bson.M{"_id": hash, "status": "pending", "gas": "0x5208", "gasPrice": "0x2"}); err != nil {
		t.Fatal(err)
	}
	if _, err := configs.PendingTransactionsCollection.InsertOne(ctx, bson.M{"_id": "0x" + strings.Repeat("b", 64), "status": "pending", "gas": "0x5208", "gasPrice": "0x4"}); err != nil {
		t.Fatal(err)
	}
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/gas/summary", handleGasSummary)
	router.GET("/pending-tx-eta/:hash", handlePendingTxETA)
	for _, known := range []bool{false, true} {
		for _, blocks := range []int{0, 1, 2} {
			t.Run(fmt.Sprintf("timing_%t_blocks_%d", known, blocks), func(t *testing.T) {
				routeCache = cache.New()
				if _, err := configs.SyncStateCollection.DeleteMany(ctx, bson.M{}); err != nil {
					t.Fatal(err)
				}
				if known {
					if _, err := configs.SyncStateCollection.InsertOne(ctx, bson.M{"_id": "chain_timing", "slotsPerEpoch": 8, "secondsPerSlot": 60, "genesisTime": 1700000000}); err != nil {
						t.Fatal(err)
					}
				}
				if _, err := configs.BlocksCollection.DeleteMany(ctx, bson.M{}); err != nil {
					t.Fatal(err)
				}
				for i := 0; i < blocks; i++ {
					_, err := configs.BlocksCollection.InsertOne(ctx, bson.M{
						"blockNumberInt": i + 1, "ingestionState": "complete",
						"result": bson.M{"number": fmt.Sprintf("0x%x", i+1), "timestamp": fmt.Sprintf("0x%x", 1700000000+i*90), "gasused": "0x5208", "gaslimit": "0x100000", "transactions": bson.A{bson.M{"hash": hash, "gasprice": "0x2"}}},
					})
					if err != nil {
						t.Fatal(err)
					}
				}
				var expected any
				if known {
					expected = float64(60)
				}
				if blocks >= 2 {
					expected = float64(90)
				}
				for _, path := range []string{"/gas/summary", "/pending-tx-eta/" + hash} {
					recorder := httptest.NewRecorder()
					router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
					if recorder.Code != http.StatusOK {
						t.Fatalf("%s: %d %s", path, recorder.Code, recorder.Body.String())
					}
					var body map[string]any
					if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
						t.Fatal(err)
					}
					value, exists := body["avgBlockTimeSec"]
					if !exists || value != expected {
						t.Fatalf("%s timing=%v present=%t, want %v", path, value, exists, expected)
					}
					if body["pendingCount"] != float64(2) {
						t.Fatalf("mempool data lost: %v", body)
					}
					gas := "0x0"
					if blocks > 0 {
						gas = "0x5208"
					}
					if body["avgGasUsedHex"] != gas {
						t.Fatalf("gas data lost: %v", body)
					}
					if strings.HasPrefix(path, "/pending-tx-eta/") {
						eta, present := body["etaSec"]
						if !present || (expected == nil && eta != nil) || (expected != nil && (eta == nil || eta.(float64) < expected.(float64))) {
							t.Fatalf("ETA=%v, block time=%v", eta, expected)
						}
						if body["gasAheadHex"] != "0x5208" || body["yourGasPriceHex"] != "0x2" {
							t.Fatalf("transaction data lost: %v", body)
						}
					} else if body["avgGasPriceHex"] == nil || body["gasPriceHistogram"] == nil {
						t.Fatalf("gas summary incomplete: %v", body)
					}
				}
			})
		}
	}
}
