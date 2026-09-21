package db

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

const qip55ChecksumA = "QaaaAAaaaAAaAaaAaAAAAaAAAaAaAaAAaAaaAaaaaAAAAAAAAaAAAAaAaAAaaAAaaaaAaAAAAaaAaAAaaaaaaAaAAaaaaAaAaaaaAaaaAAAAaAAAAaAaAaaAAAaAaaaAA"

func TestGetBalanceCanonicalizesAddressAliasesForRPC(t *testing.T) {
	var mu sync.Mutex
	var received []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode RPC request: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if request.Method != "qrl_getBalance" {
			t.Errorf("RPC method = %q, want qrl_getBalance", request.Method)
		}
		if len(request.Params) != 2 {
			t.Errorf("RPC params length = %d, want 2", len(request.Params))
			http.Error(w, "bad params", http.StatusBadRequest)
			return
		}
		var address string
		if err := json.Unmarshal(request.Params[0], &address); err != nil {
			t.Errorf("decode address param: %v", err)
			http.Error(w, "bad address", http.StatusBadRequest)
			return
		}
		mu.Lock()
		received = append(received, address)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":"0xde0b6b3a7640000"}`))
	}))
	defer server.Close()

	originalClient := nodeRPCClient
	nodeRPCClient = server.Client()
	nodeURLOnce = sync.Once{}
	nodeURLValue = ""
	t.Setenv("NODE_URL", server.URL)
	t.Cleanup(func() {
		nodeRPCClient = originalClient
		nodeURLOnce = sync.Once{}
		nodeURLValue = ""
	})

	body := strings.Repeat("a", 128)
	aliases := []string{
		"Q" + body,
		"q" + body,
		"0x" + body,
		"0X" + body,
		qip55ChecksumA,
	}
	for _, address := range aliases {
		balance, message := GetBalance(address)
		if message != "" {
			t.Fatalf("GetBalance(%q) message = %q, want empty", address, message)
		}
		if balance != 1 {
			t.Errorf("GetBalance(%q) balance = %v, want 1", address, balance)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != len(aliases) {
		t.Fatalf("received %d RPC calls, want %d", len(received), len(aliases))
	}
	for i, address := range received {
		if address != qip55ChecksumA {
			t.Errorf("RPC call %d address = %q, want %q", i, address, qip55ChecksumA)
		}
	}
}

func TestGetBalanceRejectsInvalidChecksumBeforeRPC(t *testing.T) {
	balance, message := GetBalance("Q" + strings.Repeat("Ab", 64))
	if balance != 0 {
		t.Errorf("balance = %v, want 0", balance)
	}
	if message != "Invalid address" {
		t.Errorf("message = %q, want %q", message, "Invalid address")
	}
}
