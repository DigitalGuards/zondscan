package networkprofile

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/gorilla/websocket"
)

const testGenesis = "0x1111111111111111111111111111111111111111111111111111111111111111"

func testEnv(values map[string]string) func(string) string {
	return func(key string) string {
		if key == "MONGOURI" && values[key] == "" {
			return "mongodb://127.0.0.1:27017"
		}
		return values[key]
	}
}

func TestParseProfiles(t *testing.T) {
	for _, tc := range []struct {
		name      string
		env       map[string]string
		bytes     int
		errorText string
	}{
		{"legacy defaults", nil, 20, ""},
		{"explicit legacy database", map[string]string{"MONGO_DB_NAME": "archive_v2"}, 20, ""},
		{"valid future build", map[string]string{"EXPLORER_NETWORK": "v3", "MONGO_DB_NAME": "qrldata-v3", "EXPECTED_CHAIN_ID": "0x539", "EXPECTED_GENESIS_HASH": testGenesis}, 64, ""},
		{"v3 missing database", map[string]string{"EXPLORER_NETWORK": "v3"}, 20, "explicit MONGO_DB_NAME"},
		{"v3 legacy database", map[string]string{"EXPLORER_NETWORK": "v3", "MONGO_DB_NAME": "QrlData-Z"}, 20, "distinct"},
		{"v3 missing pins", map[string]string{"EXPLORER_NETWORK": "v3", "MONGO_DB_NAME": "v3"}, 64, "requires EXPECTED"},
		{"v3 wrong binary", map[string]string{"EXPLORER_NETWORK": "v3", "MONGO_DB_NAME": "v3", "EXPECTED_CHAIN_ID": "1337", "EXPECTED_GENESIS_HASH": testGenesis}, 20, "reviewed 64-byte"},
		{"v2 wrong binary", nil, 64, "reviewed 20-byte"},
		{"invalid network", map[string]string{"EXPLORER_NETWORK": "v4"}, 20, "must be v2 or v3"},
		{"partial pins", map[string]string{"EXPECTED_CHAIN_ID": "1337"}, 20, "configured together"},
		{"partial genesis", map[string]string{"EXPECTED_GENESIS_HASH": testGenesis}, 20, "configured together"},
		{"bad chain ID", map[string]string{"EXPECTED_CHAIN_ID": "-1", "EXPECTED_GENESIS_HASH": testGenesis}, 20, "invalid chain ID"},
		{"zero genesis", map[string]string{"EXPECTED_CHAIN_ID": "1337", "EXPECTED_GENESIS_HASH": "0x" + strings.Repeat("0", 64)}, 20, "nonzero"},
		{"bad hash", map[string]string{"EXPECTED_CHAIN_ID": "1337", "EXPECTED_GENESIS_HASH": "0x01"}, 20, "32-byte"},
		{"admin database", map[string]string{"MONGO_DB_NAME": "ADMIN"}, 20, "explorer database"},
		{"local database", map[string]string{"MONGO_DB_NAME": "local"}, 20, "explorer database"},
		{"config database", map[string]string{"MONGO_DB_NAME": "config"}, 20, "explorer database"},
		{"database punctuation", map[string]string{"MONGO_DB_NAME": "v3.other"}, 20, "1-63"},
		{"database slash", map[string]string{"MONGO_DB_NAME": "v3/other"}, 20, "1-63"},
		{"database length", map[string]string{"MONGO_DB_NAME": strings.Repeat("a", 64)}, 20, "1-63"},
		{"URI mismatch", map[string]string{"MONGOURI": "mongodb://user:secret@localhost/other?authSource=admin"}, 20, "suffix must match"},
		{"URI match", map[string]string{"MONGOURI": "mongodb://user:secret@localhost/qrldata-z?authSource=admin"}, 20, ""},
		{"URI encoded mismatch", map[string]string{"MONGOURI": "mongodb://localhost/qrldata%2dv3"}, 20, "suffix must match"},
		{"URI invalid", map[string]string{"MONGOURI": "mongo://user:secret@localhost"}, 20, "MONGOURI is invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			profile, err := Parse(testEnv(tc.env), tc.bytes)
			if tc.errorText != "" {
				if err == nil || !strings.Contains(err.Error(), tc.errorText) {
					t.Fatalf("got %v, want %q", err, tc.errorText)
				}
				if strings.Contains(err.Error(), "secret") {
					t.Fatal("error leaked credentials")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if profile.Identity.NetworkID == "v3" && (profile.Identity.ChainID != "1337" || !profile.Pinned) {
				t.Fatalf("unexpected future profile: %+v", profile)
			}
		})
	}
}

func TestCanonicalPins(t *testing.T) {
	for _, v := range []string{"0x539", "0X0539", "001337", "1337"} {
		got, err := NormalizeChainID(v)
		if err != nil || got != "1337" {
			t.Fatalf("%q: %q %v", v, got, err)
		}
	}
	for _, v := range []string{"", "0", "-1", "+1", " 1", "0x", "0xffg", strings.Repeat("f", 79), "0x1" + strings.Repeat("0", 64)} {
		if _, err := NormalizeChainID(v); err == nil {
			t.Fatalf("accepted %q", v)
		}
	}
	got, err := NormalizeGenesisHash("0X" + strings.Repeat("A", 64))
	if err != nil || got != "0x"+strings.Repeat("a", 64) {
		t.Fatal(got, err)
	}
}

func TestIdentityMismatchIncludesEveryField(t *testing.T) {
	expected := Identity{"v3", "1337", testGenesis, 64, true}
	if err := matchIdentity(expected, expected); err != nil {
		t.Fatal(err)
	}
	for _, actual := range []Identity{{"v2", "1337", testGenesis, 64, true}, {"v3", "1338", testGenesis, 64, true}, {"v3", "1337", "0x" + strings.Repeat("2", 64), 64, true}, {"v3", "1337", testGenesis, 20, true}, {"v3", "", "", 64, false}} {
		if err := matchIdentity(actual, expected); err == nil {
			t.Fatalf("accepted %+v", actual)
		}
	}
}

func identityResponse(payload []byte, hash string) []byte {
	var req struct {
		Method string `json:"method"`
	}
	json.Unmarshal(payload, &req)
	var result interface{} = "0x539"
	if req.Method == "qrl_getBlockByNumber" {
		result = map[string]string{"hash": hash}
	}
	body, _ := json.Marshal(map[string]interface{}{"jsonrpc": "2.0", "id": 1, "result": result})
	return body
}

func identityServer(t *testing.T, hash string, websocketMode bool) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if websocketMode {
			upgrader := websocket.Upgrader{}
			c, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Error(err)
				return
			}
			defer c.Close()
			_, payload, err := c.ReadMessage()
			if err != nil {
				t.Error(err)
				return
			}
			c.WriteMessage(websocket.TextMessage, identityResponse(payload, hash))
			return
		}
		var raw json.RawMessage
		if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
			t.Error(err)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write(identityResponse(raw, hash))
	}))
}

func TestPinCheckEverySourceAndGenesis(t *testing.T) {
	good := identityServer(t, testGenesis, false)
	defer good.Close()
	other := identityServer(t, "0x"+strings.Repeat("2", 64), false)
	defer other.Close()
	ws := identityServer(t, testGenesis, true)
	defer ws.Close()
	profile := Profile{Identity: Identity{"v3", "1337", testGenesis, 64, true}, Pinned: true}
	sources := Sources(testEnv(map[string]string{"NODE_URL": good.URL, "NODE_URLS": good.URL + ", " + ws.URL, "MEMPOOL_NODE_URL": good.URL, "TRACE_NODE_URL": other.URL}))
	if len(sources) != 3 {
		t.Fatalf("sources %+v", sources)
	}
	if err := ValidateSources(context.Background(), profile, []string{good.URL, "ws" + strings.TrimPrefix(ws.URL, "http")}); err != nil {
		t.Fatal(err)
	}
	if err := ValidateSources(context.Background(), profile, []string{good.URL, other.URL}); err == nil || !strings.Contains(err.Error(), "genesis hash mismatch") {
		t.Fatalf("same ID with different genesis: %v", err)
	}
	profile.Identity.ChainID = "1338"
	if err := ValidateSources(context.Background(), profile, []string{good.URL}); err == nil || !strings.Contains(err.Error(), "chain ID mismatch") {
		t.Fatalf("wrong chain: %v", err)
	}
}

func TestPinCheckFailsClosedAndRedactsTransport(t *testing.T) {
	profile := Profile{Identity: Identity{"v3", "1337", testGenesis, 64, true}, Pinned: true}
	for _, response := range []string{`{"jsonrpc":"2.0","id":1,"result":null}`, `{"jsonrpc":"2.0","id":1,"error":{"message":"secret"}}`, `{"jsonrpc":"2.0","id":2,"result":"0x539"}`, `not json`, strings.Repeat("a", maxIdentityResponse+1)} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(response)) }))
		err := ValidateSources(context.Background(), profile, []string{strings.Replace(server.URL, "http://", "http://user:secret@", 1) + "/?token=secret"})
		server.Close()
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatalf("expected sanitized failure, got %v", err)
		}
	}
	if err := ValidateSources(context.Background(), profile, nil); err == nil {
		t.Fatal("accepted missing sources")
	}
	profile.Pinned = false
	if err := ValidateSources(context.Background(), profile, []string{"ws://legacy-debug-only"}); err != nil {
		t.Fatalf("legacy compatibility: %v", err)
	}
}

func TestRuntimeGuardDetectsEndpointReset(t *testing.T) {
	var reset atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var raw json.RawMessage
		json.NewDecoder(r.Body).Decode(&raw)
		hash := testGenesis
		if reset.Load() {
			hash = "0x" + strings.Repeat("2", 64)
		}
		w.Write(identityResponse(raw, hash))
	}))
	defer server.Close()
	Activate(Profile{Identity: Identity{"v3", "1337", testGenesis, 64, true}, Pinned: true})
	defer runtimeProfile.Store(nil)
	if err := GuardSource(context.Background(), server.URL); err != nil {
		t.Fatal(err)
	}
	reset.Store(true)
	if err := GuardSource(context.Background(), server.URL); err == nil || !strings.Contains(err.Error(), "genesis hash mismatch") {
		t.Fatalf("accepted reset endpoint: %v", err)
	}
}
