package rpc

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

const reportedFaucetTx = "0xd5e416fa509d3b157f6f5f160562cd4e4b076114f39bd7b97e1c22c0510c4dc9"

func TestTraceEndpointViaPrivateWebSocket(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(_ *http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			t.Errorf("upgrade WebSocket: %v", err)
			return
		}
		defer conn.Close()

		_, body, err := conn.ReadMessage()
		if err != nil {
			t.Errorf("read request: %v", err)
			return
		}
		var request struct {
			Method string `json:"method"`
		}
		if err := json.Unmarshal(body, &request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}

		var response string
		switch request.Method {
		case "rpc_modules":
			response = `{"jsonrpc":"2.0","id":1,"result":{"debug":"1.0","qrl":"1.0"}}`
		case "debug_traceTransaction":
			// Regression fixture for the reported faucet claim: the outer call
			// moves no value, while two nested calls pay 200 QRL and a gas refund.
			response = `{"jsonrpc":"2.0","id":1,"result":{"type":"CALL","from":"0xc670e4e2d24db18ee19710eb4ece9dd3794d5740","to":"0x75e6770674f9f954801c4d7d4cc0c8f8c2c3f1ea","gas":"0x14b00","gasUsed":"0x14b00","input":"0xddeae033","output":"0x","value":"0x0","calls":[{"type":"CALL","from":"0x75e6770674f9f954801c4d7d4cc0c8f8c2c3f1ea","to":"0xc670e4e2d24db18ee19710eb4ece9dd3794d5740","gas":"0x1000","gasUsed":"0x900","input":"0x","output":"0x","value":"0xad78ebc5ac6200000"},{"type":"CALL","from":"0x75e6770674f9f954801c4d7d4cc0c8f8c2c3f1ea","to":"0xc670e4e2d24db18ee19710eb4ece9dd3794d5740","gas":"0x800","gasUsed":"0x400","input":"0x","output":"0x","value":"0x5443fdfb536d"}]}}`
		default:
			t.Errorf("unexpected RPC method %q", request.Method)
			return
		}
		if err := conn.WriteMessage(websocket.TextMessage, []byte(response)); err != nil {
			t.Errorf("write response: %v", err)
		}
	}))
	defer server.Close()

	t.Setenv("ENABLE_DEBUG_TRACE", "true")
	t.Setenv("TRACE_NODE_URL", "ws"+strings.TrimPrefix(server.URL, "http"))

	if err := ValidateDebugTraceEndpoint(t.Context()); err != nil {
		t.Fatalf("validate private trace endpoint: %v", err)
	}

	trace := CallDebugTraceTransaction(reportedFaucetTx)
	if trace.Err != nil {
		t.Fatalf("trace reported transaction: %v", trace.Err)
	}
	if len(trace.InternalCalls) != 2 {
		t.Fatalf("expected payout and gas-refund calls, got %d", len(trace.InternalCalls))
	}
	if trace.InternalCalls[0].Value != 200 {
		t.Errorf("expected 200 QRL payout, got %v", trace.InternalCalls[0].Value)
	}
	if trace.InternalCalls[0].From != "Q75e6770674f9f954801c4d7d4cc0c8f8c2c3f1ea" {
		t.Errorf("unexpected payout sender %q", trace.InternalCalls[0].From)
	}
	if trace.InternalCalls[0].To != "Qc670e4e2d24db18ee19710eb4ece9dd3794d5740" {
		t.Errorf("unexpected payout recipient %q", trace.InternalCalls[0].To)
	}
	if !reflect.DeepEqual(trace.InternalCalls[1].TraceAddress, []int{1}) {
		t.Errorf("expected gas-refund trace path [1], got %v", trace.InternalCalls[1].TraceAddress)
	}
}

func TestValidateDebugTraceEndpointRejectsMissingModule(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"qrl":"1.0","web3":"1.0"}}`))
	}))
	defer server.Close()

	t.Setenv("ENABLE_DEBUG_TRACE", "true")
	t.Setenv("TRACE_NODE_URL", server.URL)

	err := ValidateDebugTraceEndpoint(t.Context())
	if err == nil || !strings.Contains(err.Error(), "does not expose the debug module") {
		t.Fatalf("expected missing-debug error, got %v", err)
	}
}

func TestValidateDebugTraceEndpointSkipsProbeWhenDisabled(t *testing.T) {
	t.Setenv("ENABLE_DEBUG_TRACE", "false")
	t.Setenv("TRACE_NODE_URL", "unsupported://trace")

	if err := ValidateDebugTraceEndpoint(t.Context()); err != nil {
		t.Fatalf("disabled tracing should not probe an endpoint: %v", err)
	}
}
