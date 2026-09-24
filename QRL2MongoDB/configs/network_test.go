package configs

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWrongBeaconFailsBeforeMongoConnection(t *testing.T) {
	executionGenesis := "0x" + strings.Repeat("1", 64)
	execution := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string `json:"method"`
		}
		json.NewDecoder(r.Body).Decode(&request)
		var result interface{} = "0x539"
		if request.Method == "qrl_getBlockByNumber" {
			result = map[string]string{"hash": executionGenesis}
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": result})
	}))
	defer execution.Close()
	beacon := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]string{"genesis_time": "1700000000", "genesis_validators_root": "0x" + strings.Repeat("2", 64)}})
	}))
	defer beacon.Close()
	for key, value := range map[string]string{
		"EXPLORER_NETWORK": "v2", "MONGO_DB_NAME": "guard_test_v2", "MONGOURI": "mongodb://127.0.0.1:1",
		"NODE_URL": execution.URL, "NODE_URLS": "", "MEMPOOL_NODE_URL": "", "TRACE_NODE_URL": "",
		"EXPECTED_CHAIN_ID": "1337", "EXPECTED_GENESIS_HASH": executionGenesis, "BEACONCHAIN_API": beacon.URL,
		"EXPECTED_BEACON_GENESIS_TIME": "1700000000", "EXPECTED_BEACON_GENESIS_VALIDATORS_ROOT": "0x" + strings.Repeat("3", 64),
	} {
		t.Setenv(key, value)
	}
	err := connect()
	if err == nil || !strings.Contains(err.Error(), "beacon network identity mismatch") {
		t.Fatalf("expected beacon rejection before Mongo access, got %v", err)
	}
	if DB != nil || BlocksCollections != nil {
		t.Fatal("database handles exposed after beacon mismatch")
	}
}
