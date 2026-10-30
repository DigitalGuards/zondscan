package routes

import (
	"backendAPI/cache"
	"backendAPI/db"
	"backendAPI/qns"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
)

func qnsRawString(value string) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

func configureQNSRoute(t *testing.T) {
	t.Helper()
	t.Setenv("QNS_REGISTRY_ADDRESS", "Q"+strings.Repeat("1", 128))
	t.Setenv("QNS_EXPECTED_CHAIN_ID", "1337")
}

func newQNSTestRouter(call qns.RPCFunc) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	responseCache := cache.New()
	router.GET("/qns/resolve/:name", newQNSResolveHandler(call, responseCache))
	router.GET("/qns/reverse/:address", newQNSReverseHandler(call, responseCache))
	return router
}

func serveQNSReverse(router *gin.Engine, address string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/qns/reverse/"+address, nil))
	return w
}

func TestQNSReverseRouteRegistered(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	RegisterQNSRoutes(router)
	for _, route := range router.Routes() {
		if route.Method == http.MethodGet && route.Path == "/qns/reverse/:address" {
			return
		}
	}
	t.Fatal("reverse route is missing")
}

func TestQNSReverseRouteCanonicalizesAndCaches(t *testing.T) {
	for _, found := range []bool{true, false} {
		t.Run(fmt.Sprintf("found=%t", found), func(t *testing.T) {
			configureQNSRoute(t)
			var mu sync.Mutex
			calls := 0
			qrlCalls := 0
			name := "ALICE.QRL"
			encoded := fmt.Sprintf("0x%0128x%0128x", 64, len(name)) +
				hex.EncodeToString([]byte(name)) + strings.Repeat("00", 64-len(name))
			responses := []string{"0x" + strings.Repeat("2", 128), encoded, "0x" + strings.Repeat("3", 128), "0x" + strings.Repeat("a", 128)}
			if !found {
				responses = []string{"0x"}
			}
			router := newQNSTestRouter(func(ctx context.Context, method string, params []interface{}) (json.RawMessage, *db.RPCError, error) {
				mu.Lock()
				defer mu.Unlock()
				calls++
				switch method {
				case "qrl_chainId":
					return qnsRawString("0x539"), nil, nil
				case "qrl_getCode":
					return qnsRawString("0x6000"), nil, nil
				case "qrl_call":
					if qrlCalls >= len(responses) {
						return nil, nil, errors.New("unexpected extra lookup")
					}
					result := responses[qrlCalls]
					qrlCalls++
					return qnsRawString(result), nil, nil
				default:
					return nil, nil, errors.New("unexpected method")
				}
			})
			for _, address := range []string{"0X" + strings.Repeat("A", 128), qip55ChecksumA} {
				w := serveQNSReverse(router, address)
				wantStatus := http.StatusOK
				if !found {
					wantStatus = http.StatusNotFound
				}
				if w.Code != wantStatus {
					t.Fatalf("status = %d, body = %s, want %d", w.Code, w.Body.String(), wantStatus)
				}
				var response map[string]string
				if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
					t.Fatal(err)
				}
				if response["address"] != qip55ChecksumA {
					t.Errorf("address = %q, want canonical address", response["address"])
				}
				if found && (response["name"] != "alice.qrl" || response["error"] != "") {
					t.Errorf("response = %#v", response)
				}
				if !found && (response["name"] != "" || response["error"] != "QNS address has no verified primary name") {
					t.Errorf("response = %#v", response)
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if calls != 2+len(responses) {
				t.Errorf("RPC calls = %d, want %d with cached alias lookup", calls, 2+len(responses))
			}
		})
	}
}

func TestQNSReverseRouteErrorsMatchForwardRoute(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		message string
		chain   string
		code    string
		failure error
	}{
		{"unconfigured", http.StatusServiceUnavailable, "QNS resolution is not configured", "", "", nil},
		{"chain mismatch", http.StatusServiceUnavailable, "QNS deployment does not match the connected chain", "0x1", "0x6000", nil},
		{"no registry code", http.StatusServiceUnavailable, "QNS registry is unavailable on the connected chain", "0x539", "0x", nil},
		{"RPC failure", http.StatusBadGateway, "QNS resolution failed", "0x539", "0x6000", errors.New("offline")},
		{"timeout", http.StatusGatewayTimeout, "QNS resolution timed out", "0x539", "0x6000", context.DeadlineExceeded},
		{"malformed resolver", http.StatusBadGateway, "QNS resolution failed", "0x539", "0x6000", nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			configureQNSRoute(t)
			if tc.name == "unconfigured" {
				t.Setenv("QNS_REGISTRY_ADDRESS", "")
			}
			router := newQNSTestRouter(func(ctx context.Context, method string, params []interface{}) (json.RawMessage, *db.RPCError, error) {
				switch method {
				case "qrl_chainId":
					return qnsRawString(tc.chain), nil, nil
				case "qrl_getCode":
					return qnsRawString(tc.code), nil, nil
				default:
					if tc.failure == context.DeadlineExceeded {
						<-ctx.Done()
						return nil, nil, ctx.Err()
					}
					return qnsRawString("0x12"), nil, tc.failure
				}
			})
			w := serveQNSReverse(router, qip55ChecksumA)
			if w.Code != tc.status {
				t.Errorf("status = %d, want %d", w.Code, tc.status)
			}
			var response map[string]string
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response["error"] != tc.message || response["name"] != "" {
				t.Errorf("response = %#v, want error %q", response, tc.message)
			}
		})
	}
}

func TestQNSReverseRouteRejectsInvalidAddressBeforeRPC(t *testing.T) {
	router := newQNSTestRouter(func(context.Context, string, []interface{}) (json.RawMessage, *db.RPCError, error) {
		t.Error("unexpected RPC call")
		return nil, nil, errors.New("invalid input reached RPC")
	})
	for _, address := range []string{"alice.qrl", "Q1234", "Q" + strings.Repeat("Ab", 64)} {
		w := serveQNSReverse(router, address)
		if w.Code != http.StatusBadRequest || w.Body.String() != `{"error":"invalid QNS address"}` {
			t.Errorf("status = %d, body = %s", w.Code, w.Body.String())
		}
	}
}

func serveQNS(router *gin.Engine, name string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/qns/resolve/"+name, nil)
	router.ServeHTTP(w, req)
	return w
}

func TestQNSResolveRouteNormalizesAndCachesKnownVector(t *testing.T) {
	configureQNSRoute(t)
	var mu sync.Mutex
	var methods []string
	qrlCallCount := 0
	call := func(
		ctx context.Context,
		method string,
		params []interface{},
	) (json.RawMessage, *db.RPCError, error) {
		mu.Lock()
		defer mu.Unlock()
		methods = append(methods, method)
		switch method {
		case "qrl_chainId":
			return qnsRawString("0x539"), nil, nil
		case "qrl_getCode":
			return qnsRawString("0x6000"), nil, nil
		case "qrl_call":
			qrlCallCount++
			if qrlCallCount == 1 {
				return qnsRawString("0x" + strings.Repeat("2", 128)), nil, nil
			}
			return qnsRawString("0x" + strings.Repeat("Aa", 64)), nil, nil
		default:
			return nil, nil, errors.New("unexpected method")
		}
	}
	router := newQNSTestRouter(call)

	for _, name := range []string{"ALICE.QRL", "alice.qrl"} {
		w := serveQNS(router, name)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: status = %d, body = %s", name, w.Code, w.Body.String())
		}
		var response struct {
			Name    string `json:"name"`
			Address string `json:"address"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if response.Name != "alice.qrl" || response.Address != qip55ChecksumA {
			t.Errorf("response = %#v", response)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	wantMethods := []string{"qrl_chainId", "qrl_getCode", "qrl_call", "qrl_call"}
	if strings.Join(methods, ",") != strings.Join(wantMethods, ",") {
		t.Errorf("methods = %#v, want %#v", methods, wantMethods)
	}
}

func TestQNSResolveRouteRejectsInvalidNameBeforeRPC(t *testing.T) {
	configureQNSRoute(t)
	calls := 0
	router := newQNSTestRouter(func(
		ctx context.Context,
		method string,
		params []interface{},
	) (json.RawMessage, *db.RPCError, error) {
		calls++
		return nil, nil, errors.New("should not be called")
	})

	w := serveQNS(router, "alice.eth")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if got, want := w.Body.String(), `{"error":"invalid QNS name"}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
	if calls != 0 {
		t.Errorf("RPC calls = %d, want 0", calls)
	}
}

func TestQNSResolveRouteReturns503WhenUnconfigured(t *testing.T) {
	t.Setenv("QNS_REGISTRY_ADDRESS", "")
	t.Setenv("QNS_EXPECTED_CHAIN_ID", "")
	calls := 0
	router := newQNSTestRouter(func(
		ctx context.Context,
		method string,
		params []interface{},
	) (json.RawMessage, *db.RPCError, error) {
		calls++
		return nil, nil, errors.New("should not be called")
	})

	w := serveQNS(router, "alice.qrl")
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, body = %s, want 503", w.Code, w.Body.String())
	}
	if got, want := w.Body.String(), `{"error":"QNS resolution is not configured"}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
	if calls != 0 {
		t.Errorf("RPC calls = %d, want 0", calls)
	}
}

func TestQNSResolveRouteReturns502OnRPCError(t *testing.T) {
	configureQNSRoute(t)
	router := newQNSTestRouter(func(
		ctx context.Context,
		method string,
		params []interface{},
	) (json.RawMessage, *db.RPCError, error) {
		return nil, &db.RPCError{Code: -32000, Message: "failed"}, nil
	})

	w := serveQNS(router, "alice.qrl")
	if w.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, body = %s, want 502", w.Code, w.Body.String())
	}
	if got, want := w.Body.String(), `{"error":"QNS resolution failed"}`; got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func TestQNSResolveRouteFailsClosedOnDeploymentAndRecords(t *testing.T) {
	cases := []struct {
		name          string
		chain         string
		code          string
		callResponses []string
		wantStatus    int
		wantError     string
		wantName      string
	}{
		{"chain mismatch", "0x1", "0x6000", nil, http.StatusServiceUnavailable, "QNS deployment does not match the connected chain", ""},
		{"no registry code", "0x539", "0x", nil, http.StatusServiceUnavailable, "QNS registry is unavailable on the connected chain", ""},
		{"zero resolver", "0x539", "0x6000", []string{"0x"}, http.StatusNotFound, "QNS name has no resolver", "alice.qrl"},
		{"zero address", "0x539", "0x6000", []string{"0x" + strings.Repeat("2", 128), "0x"}, http.StatusNotFound, "QNS name has no address record", "alice.qrl"},
		{"malformed resolver", "0x539", "0x6000", []string{"0x12"}, http.StatusBadGateway, "QNS resolution failed", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			configureQNSRoute(t)
			callIndex := 0
			router := newQNSTestRouter(func(
				ctx context.Context,
				method string,
				params []interface{},
			) (json.RawMessage, *db.RPCError, error) {
				switch method {
				case "qrl_chainId":
					return qnsRawString(tc.chain), nil, nil
				case "qrl_getCode":
					return qnsRawString(tc.code), nil, nil
				case "qrl_call":
					response := tc.callResponses[callIndex]
					callIndex++
					return qnsRawString(response), nil, nil
				default:
					return nil, nil, errors.New("unexpected method")
				}
			})
			w := serveQNS(router, "alice.qrl")
			if w.Code != tc.wantStatus {
				t.Errorf("status = %d, body = %s, want %d", w.Code, w.Body.String(), tc.wantStatus)
			}
			var response struct {
				Error string `json:"error"`
				Name  string `json:"name"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if response.Error != tc.wantError || response.Name != tc.wantName {
				t.Errorf("response = %#v, want error %q and name %q", response, tc.wantError, tc.wantName)
			}
		})
	}
}
