package routes

import (
	"backendAPI/cache"
	"backendAPI/db"
	"backendAPI/qns"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

// qnsAliceNode is namehash("alice.qrl").
const qnsAliceNode = "0xefe3586aa9a851831a32d38044822af21cc5380e38f05cdc0dd562b4cfada103"

func newQNSNodeTestRouter(call qns.RPCFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/qns/node/:node", newQNSNodeHandler(call, cache.New()))
	return router
}

func qnsNodeRPC(responses []string, calls *int, mu *sync.Mutex) qns.RPCFunc {
	return func(ctx context.Context, method string, params []interface{}) (json.RawMessage, *db.RPCError, error) {
		mu.Lock()
		defer mu.Unlock()
		switch method {
		case "qrl_chainId":
			return qnsRawString("0x539"), nil, nil
		case "qrl_getCode":
			return qnsRawString("0x6000"), nil, nil
		}
		step := *calls
		*calls++
		if step >= len(responses) {
			return nil, &db.RPCError{Code: 3}, nil
		}
		return qnsRawString(responses[step]), nil, nil
	}
}

func serveQNSNode(router *gin.Engine, node string) (*httptest.ResponseRecorder, map[string]interface{}) {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/qns/node/"+node, nil))
	var body map[string]interface{}
	_ = json.Unmarshal(w.Body.Bytes(), &body)
	return w, body
}

func TestQNSNodeRouteRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterQNSRoutes(router)
	for _, route := range router.Routes() {
		if route.Method == http.MethodGet && route.Path == "/qns/node/:node" {
			return
		}
	}
	t.Fatal("node route is missing")
}

func TestQNSNodeRouteReturnsVerifiedName(t *testing.T) {
	configureQNSRoute(t)
	var mu sync.Mutex
	calls := 0
	router := newQNSNodeTestRouter(qnsNodeRPC([]string{
		"0x" + strings.Repeat("a", 128),
		"0x" + strings.Repeat("2", 128),
		qnsEncodedName("alice.qrl"),
	}, &calls, &mu))

	w, body := serveQNSNode(router, strings.ToUpper(qnsAliceNode[:2])+strings.ToUpper(qnsAliceNode[2:]))
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, body %s", w.Code, w.Body.String())
	}
	want := map[string]interface{}{
		"node":     qnsAliceNode,
		"owner":    "QaaaAAaaaAAaAaaAaAAAAaAAAaAaAaAAaAaaAaaaaAAAAAAAAaAAAAaAaAAaaAAaaaaAaAAAAaaAaAAaaaaaaAaAAaaaaAaAaaaaAaaaAAAAaAAAAaAaAaaAAAaAaaaAA",
		"resolver": "Q" + strings.Repeat("2", 128),
		"name":     "alice.qrl",
	}
	for key, value := range want {
		if body[key] != value {
			t.Errorf("%s = %#v, want %#v", key, body[key], value)
		}
	}

	// The second request is served from the shared cache.
	if w, _ := serveQNSNode(router, qnsAliceNode); w.Code != http.StatusOK || calls != 3 {
		t.Fatalf("cached status = %d, calls = %d", w.Code, calls)
	}
}

func TestQNSNodeRouteReportsUnpublishedName(t *testing.T) {
	configureQNSRoute(t)
	var mu sync.Mutex
	calls := 0
	router := newQNSNodeTestRouter(qnsNodeRPC([]string{
		"0x" + strings.Repeat("a", 128),
		"0x" + strings.Repeat("2", 128),
		qnsEncodedName("bob.qrl"),
	}, &calls, &mu))
	w, body := serveQNSNode(router, qnsAliceNode)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	if value, present := body["name"]; !present || value != nil {
		t.Errorf("name = %#v, want null", value)
	}
}

func TestQNSNodeRouteRejectsUnownedAndInvalidNodes(t *testing.T) {
	configureQNSRoute(t)
	var mu sync.Mutex
	calls := 0
	router := newQNSNodeTestRouter(qnsNodeRPC([]string{"0x" + strings.Repeat("0", 128)}, &calls, &mu))
	w, body := serveQNSNode(router, qnsAliceNode)
	if w.Code != http.StatusNotFound || body["error"] != "QNS node has no owner" || body["node"] != qnsAliceNode {
		t.Fatalf("unowned status = %d, body %s", w.Code, w.Body.String())
	}
	for _, input := range []string{"alice.qrl", qnsAliceNode[:65], qnsAliceNode + "0", "0x" + strings.Repeat("g", 64)} {
		if w, _ := serveQNSNode(router, input); w.Code != http.StatusBadRequest {
			t.Errorf("%q status = %d, want 400", input, w.Code)
		}
	}
}

func TestQNSNodeRouteFailsClosedOnRegistryFailure(t *testing.T) {
	configureQNSRoute(t)
	router := newQNSNodeTestRouter(func(ctx context.Context, method string, params []interface{}) (json.RawMessage, *db.RPCError, error) {
		switch method {
		case "qrl_chainId":
			return qnsRawString("0x539"), nil, nil
		case "qrl_getCode":
			return qnsRawString("0x6000"), nil, nil
		}
		return nil, &db.RPCError{Code: -32000, Message: "state unavailable"}, nil
	})
	if w, _ := serveQNSNode(router, qnsAliceNode); w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", w.Code)
	}
}

func TestQNSNodeRouteRequiresConfiguration(t *testing.T) {
	t.Setenv("QNS_REGISTRY_ADDRESS", "")
	t.Setenv("QNS_EXPECTED_CHAIN_ID", "")
	router := newQNSNodeTestRouter(func(context.Context, string, []interface{}) (json.RawMessage, *db.RPCError, error) {
		t.Fatal("unexpected RPC call")
		return nil, nil, nil
	})
	if w, _ := serveQNSNode(router, qnsAliceNode); w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want 503", w.Code)
	}
}
