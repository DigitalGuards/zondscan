package beaconprofile

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const rootPin = "0x1111111111111111111111111111111111111111111111111111111111111111"

func testEnvironment(source string) map[string]string {
	return map[string]string{"BEACONCHAIN_API": source, "EXPECTED_BEACON_GENESIS_VALIDATORS_ROOT": rootPin, "EXPECTED_BEACON_GENESIS_TIME": "1700000000"}
}

func TestPinnedBeaconConfiguration(t *testing.T) {
	for _, key := range []string{"BEACONCHAIN_API", "EXPECTED_BEACON_GENESIS_VALIDATORS_ROOT", "EXPECTED_BEACON_GENESIS_TIME"} {
		environ := testEnvironment("http://localhost:3500")
		delete(environ, key)
		if _, err := Parse(func(k string) string { return environ[k] }, true); err == nil {
			t.Fatalf("accepted missing %s", key)
		}
	}
	for _, value := range []string{"0", "+1", "01", "0x123", "18446744073709551616"} {
		environ := testEnvironment("http://localhost:3500")
		environ["EXPECTED_BEACON_GENESIS_TIME"] = value
		if _, err := Parse(func(k string) string { return environ[k] }, true); err == nil {
			t.Fatalf("accepted time %q", value)
		}
	}
	if p, err := Parse(func(string) string { return "" }, false); err != nil || p != nil {
		t.Fatalf("legacy compatibility: %v %v", p, err)
	}
}

func TestBeaconGenesisAndRuntimeReset(t *testing.T) {
	var reset atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/eth/v1/beacon/genesis" {
			t.Error("wrong identity path", r.URL.Path)
		}
		root := rootPin
		if reset.Load() {
			root = "0x" + strings.Repeat("2", 64)
		}
		fmt.Fprintf(w, `{"data":{"genesis_time":"1700000000","genesis_validators_root":%q}}`, root)
	}))
	defer server.Close()
	env := testEnvironment(server.URL)
	if err := Configure(context.Background(), func(k string) string { return env[k] }, true); err != nil {
		t.Fatal(err)
	}
	defer active.Store(nil)
	if err := GuardSource(context.Background(), server.URL); err != nil {
		t.Fatal(err)
	}
	reset.Store(true)
	if err := GuardSource(context.Background(), server.URL); err == nil || !strings.Contains(err.Error(), "identity mismatch") {
		t.Fatal("accepted reset beacon", err)
	}
	if err := GuardSource(context.Background(), "http://different"); err == nil {
		t.Fatal("accepted changed beacon source")
	}
}

func TestBeaconTransportFailsClosed(t *testing.T) {
	for _, body := range []string{`{"data":{"genesis_time":"1700000001","genesis_validators_root":"` + rootPin + `"}}`, `{"data":{}}`, strings.Repeat("a", (64<<10)+1)} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte(body)) }))
		env := testEnvironment(strings.Replace(server.URL, "http://", "http://user:secret@", 1))
		err := Configure(context.Background(), func(k string) string { return env[k] }, true)
		server.Close()
		if err == nil || strings.Contains(err.Error(), "secret") {
			t.Fatal("expected sanitized beacon failure", err)
		}
	}
}
