package rpc

import (
	"QRL2MongoDB/beaconprofile"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

func TestBeaconResultsRejectedWhenSourceResetsDuringRead(t *testing.T) {
	for _, kind := range []string{"head", "validators"} {
		t.Run(kind, func(t *testing.T) {
			var reset atomic.Bool
			genesisRoot := "0x" + strings.Repeat("1", 64)
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/eth/v1/beacon/genesis" {
					root := genesisRoot
					if reset.Load() {
						root = "0x" + strings.Repeat("2", 64)
					}
					json.NewEncoder(w).Encode(map[string]interface{}{"data": map[string]string{"genesis_time": "1700000000", "genesis_validators_root": root}})
					return
				}
				reset.Store(true)
				w.Write([]byte(`{}`))
			}))
			defer server.Close()
			env := map[string]string{"BEACONCHAIN_API": server.URL, "EXPECTED_BEACON_GENESIS_VALIDATORS_ROOT": genesisRoot, "EXPECTED_BEACON_GENESIS_TIME": "1700000000"}
			if err := beaconprofile.Configure(context.Background(), func(k string) string { return env[k] }, true); err != nil {
				t.Fatal(err)
			}
			defer beaconprofile.Configure(context.Background(), func(string) string { return "" }, false)
			t.Setenv("BEACONCHAIN_API", server.URL)
			if kind == "head" {
				result, err := GetBeaconChainHead()
				if result != nil || err == nil || !strings.Contains(err.Error(), "beacon network identity mismatch") {
					t.Fatalf("accepted head from reset beacon: %+v %v", result, err)
				}
			} else {
				result, err := GetValidators()
				if result != nil || err == nil || !strings.Contains(err.Error(), "beacon network identity mismatch") {
					t.Fatalf("accepted validators from reset beacon: %+v %v", result, err)
				}
			}
		})
	}
}
