package routes

import (
	"backendAPI/cache"
	"backendAPI/db"
	"backendAPI/qns"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
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

func TestQNSRoutesHaveIndependentRateLimits(t *testing.T) {
	t.Setenv("QNS_INTERNAL_TOKEN", "")
	gin.SetMode(gin.TestMode)
	for _, reverseFirst := range []bool{true, false} {
		t.Run(fmt.Sprintf("reverseFirst=%t", reverseFirst), func(t *testing.T) {
			router := gin.New()
			RegisterQNSRoutes(router)
			paths := []string{"/qns/reverse/invalid", "/qns/resolve/invalid"}
			limits := []int{60, 30}
			if !reverseFirst {
				paths[0], paths[1] = paths[1], paths[0]
				limits[0], limits[1] = limits[1], limits[0]
			}
			for i, path := range paths {
				for request := 0; request <= limits[i]; request++ {
					w := httptest.NewRecorder()
					router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, path, nil))
					want := http.StatusBadRequest
					if request == limits[i] {
						want = http.StatusTooManyRequests
					}
					if w.Code != want {
						t.Fatalf("%s request %d: status = %d, want %d", path, request+1, w.Code, want)
					}
				}
			}
		})
	}
}

func TestQNSInternalTokenRateLimitBypass(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var secret [32]byte
	if _, err := rand.Read(secret[:]); err != nil {
		t.Fatal("generate test secret")
	}
	token := hex.EncodeToString(secret[:])
	wrong := strings.Repeat("z", len(token))
	for _, route := range []struct {
		path  string
		limit int
	}{
		{"/qns/reverse/invalid", 60},
		{"/qns/resolve/invalid", 30},
	} {
		for _, setting := range []string{"matching", "wrong", "missing", "empty config", "empty both", "unset config"} {
			t.Run(route.path+"/"+setting, func(t *testing.T) {
				t.Setenv("QNS_INTERNAL_TOKEN", token)
				provided := token
				switch setting {
				case "wrong":
					provided = wrong
				case "missing":
					provided = ""
				case "empty config", "empty both":
					t.Setenv("QNS_INTERNAL_TOKEN", "")
					if setting == "empty both" {
						provided = ""
					}
				case "unset config":
					if err := os.Unsetenv("QNS_INTERNAL_TOKEN"); err != nil {
						t.Fatal(err)
					}
				}
				router := gin.New()
				RegisterQNSRoutes(router)
				request := func(header string, want int) {
					t.Helper()
					req := httptest.NewRequest(http.MethodGet, route.path, nil)
					if header != "" {
						req.Header.Set("X-QNS-Internal-Token", header)
					}
					w := httptest.NewRecorder()
					router.ServeHTTP(w, req)
					if w.Code != want {
						t.Fatalf("status = %d, want %d", w.Code, want)
					}
				}
				for i := 0; i <= route.limit; i++ {
					want := http.StatusBadRequest
					if i == route.limit && setting != "matching" {
						want = http.StatusTooManyRequests
					}
					request(provided, want)
				}
				if setting == "matching" {
					// Authenticated calls leave the entire public allowance available.
					for i := 0; i < route.limit; i++ {
						request("", http.StatusBadRequest)
					}
					request("", http.StatusTooManyRequests)
					request(token, http.StatusBadRequest)
					request(wrong, http.StatusTooManyRequests)
				}
			})
		}
	}
}

func qnsEncodedName(name string) string {
	padding := (64 - len(name)%64) % 64
	return fmt.Sprintf("0x%0128x%0128x", 64, len(name)) +
		hex.EncodeToString([]byte(name)) + strings.Repeat("00", padding)
}

func TestQNSReverseRouteCachesOnlyRecordNegatives(t *testing.T) {
	for _, failure := range []string{
		"transport", "timeout", "deadline", "canceled", "node", "unrelated message", "unrelated code",
		"revert code", "revert message", "revert reason", "malformed", "empty", "null", "non-string", "invalid JSON",
	} {
		for failAt := 0; failAt < 4; failAt++ {
			t.Run(fmt.Sprintf("%s/call%d", failure, failAt), func(t *testing.T) {
				configureQNSRoute(t)
				var mu sync.Mutex
				calls := 0
				qrlCalls := 0
				responses := []string{
					"0x" + strings.Repeat("2", 128),
					qnsEncodedName("alice.qrl"),
					"0x" + strings.Repeat("3", 128),
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
					}
					step := qrlCalls % (failAt + 1)
					qrlCalls++
					if step == failAt {
						switch failure {
						case "transport":
							return nil, nil, errors.New("network failure mentioning execution reverted")
						case "timeout":
							return nil, nil, os.ErrDeadlineExceeded
						case "deadline":
							return nil, nil, fmt.Errorf("call interrupted: %w", context.DeadlineExceeded)
						case "canceled":
							return nil, nil, fmt.Errorf("call interrupted: %w", context.Canceled)
						case "node":
							return nil, &db.RPCError{Code: -32000, Message: "state unavailable"}, nil
						case "unrelated message":
							return nil, &db.RPCError{Code: -32000, Message: "upstream failed before execution reverted"}, nil
						case "unrelated code":
							return nil, &db.RPCError{Code: -32603, Message: "execution reverted"}, nil
						case "revert code":
							return nil, &db.RPCError{Code: 3}, nil
						case "revert message":
							return nil, &db.RPCError{Code: -32000, Message: "execution reverted"}, nil
						case "revert reason":
							return nil, &db.RPCError{Code: -32000, Message: "execution reverted: missing record"}, nil
						case "malformed":
							return qnsRawString("0x12"), nil, nil
						case "empty":
							return qnsRawString("0x"), nil, nil
						case "null":
							return json.RawMessage(`null`), nil, nil
						case "non-string":
							return json.RawMessage(`{}`), nil, nil
						default:
							return json.RawMessage(`{`), nil, nil
						}
					}
					return qnsRawString(responses[step]), nil, nil
				})
				wantStatus := http.StatusBadGateway
				wantCalls := 2 + 2*(failAt+1)
				if failure == "empty" || (failAt > 0 && (strings.HasPrefix(failure, "revert") || failure == "malformed")) {
					wantStatus = http.StatusNotFound
					wantCalls = 2 + failAt + 1
				}
				for request := 0; request < 2; request++ {
					w := serveQNSReverse(router, qip55ChecksumA)
					if w.Code != wantStatus {
						t.Fatalf("status = %d, body = %s, want %d", w.Code, w.Body.String(), wantStatus)
					}
					if wantStatus == http.StatusNotFound && w.Body.String() != fmt.Sprintf(
						`{"address":%q,"error":"QNS address has no verified primary name"}`, qip55ChecksumA,
					) {
						t.Errorf("unexpected negative result: %s", w.Body.String())
					}
				}
				mu.Lock()
				defer mu.Unlock()
				if calls != wantCalls {
					t.Errorf("RPC calls = %d, want %d", calls, wantCalls)
				}
			})
		}
	}
}

func TestQNSReverseRouteRecoversAfterTransientRecordFailure(t *testing.T) {
	for _, failAt := range []int{1, 3} {
		t.Run(fmt.Sprintf("call%d", failAt), func(t *testing.T) {
			configureQNSRoute(t)
			var mu sync.Mutex
			calls := 0
			responses := []string{
				"0x" + strings.Repeat("2", 128),
				qnsEncodedName("alice.qrl"),
				"0x" + strings.Repeat("3", 128),
				"0x" + strings.Repeat("a", 128),
			}
			router := newQNSTestRouter(func(ctx context.Context, method string, params []interface{}) (json.RawMessage, *db.RPCError, error) {
				mu.Lock()
				defer mu.Unlock()
				switch method {
				case "qrl_chainId":
					return qnsRawString("0x539"), nil, nil
				case "qrl_getCode":
					return qnsRawString("0x6000"), nil, nil
				}
				index := calls
				calls++
				if index == failAt {
					return nil, &db.RPCError{Code: -32000, Message: "temporarily unavailable"}, nil
				}
				if index > failAt {
					index -= failAt + 1
				}
				return qnsRawString(responses[index]), nil, nil
			})
			for _, want := range []int{http.StatusBadGateway, http.StatusOK, http.StatusOK} {
				w := serveQNSReverse(router, qip55ChecksumA)
				if w.Code != want {
					t.Fatalf("status = %d, want %d", w.Code, want)
				}
				if want == http.StatusOK && !strings.Contains(w.Body.String(), `"name":"alice.qrl"`) {
					t.Fatal("recovered response lacks the verified name")
				}
			}
			mu.Lock()
			defer mu.Unlock()
			if calls != failAt+1+len(responses) {
				t.Errorf("record calls = %d, want %d", calls, failAt+1+len(responses))
			}
		})
	}
}

func TestQNSReverseRouteSharesForwardCache(t *testing.T) {
	for _, reverseFirst := range []bool{false, true} {
		for _, record := range []string{"verified", "mismatch", "missing resolver", "missing address"} {
			t.Run(fmt.Sprintf("%s/reverseFirst=%t", record, reverseFirst), func(t *testing.T) {
				configureQNSRoute(t)
				var mu sync.Mutex
				forwardCalls, reverseCalls := 0, 0
				forwardNode := qns.Namehash("alice.qrl")
				forwardResolverData := "0x0178b8bf" + hex.EncodeToString(forwardNode[:]) + strings.Repeat("0", 64)
				router := newQNSTestRouter(func(ctx context.Context, method string, params []interface{}) (json.RawMessage, *db.RPCError, error) {
					mu.Lock()
					defer mu.Unlock()
					switch method {
					case "qrl_chainId":
						return qnsRawString("0x539"), nil, nil
					case "qrl_getCode":
						return qnsRawString("0x6000"), nil, nil
					}
					data := params[0].(map[string]interface{})["data"].(string)
					switch {
					case data == forwardResolverData:
						forwardCalls++
						if record == "missing resolver" {
							return qnsRawString("0x"), nil, nil
						}
					case strings.HasPrefix(data, "0x3b3b57de"):
						forwardCalls++
						address := "0x" + strings.Repeat("a", 128)
						if record == "mismatch" {
							address = "0x" + strings.Repeat("b", 128)
						} else if record == "missing address" {
							address = "0x"
						}
						return qnsRawString(address), nil, nil
					case strings.HasPrefix(data, "0x691f3431"):
						reverseCalls++
						return qnsRawString(qnsEncodedName("ALICE.QRL")), nil, nil
					default:
						reverseCalls++
					}
					return qnsRawString("0x" + strings.Repeat("2", 128)), nil, nil
				})
				forwardStatus, reverseStatus := http.StatusOK, http.StatusNotFound
				if strings.HasPrefix(record, "missing") {
					forwardStatus = http.StatusNotFound
				} else if record == "verified" {
					reverseStatus = http.StatusOK
				}
				for _, reverse := range []bool{reverseFirst, !reverseFirst} {
					var w *httptest.ResponseRecorder
					want := forwardStatus
					if reverse {
						w = serveQNSReverse(router, qip55ChecksumA)
						want = reverseStatus
					} else {
						w = serveQNS(router, "alice.qrl")
					}
					if w.Code != want {
						t.Fatalf("reverse=%t: status = %d, body = %s, want %d", reverse, w.Code, w.Body.String(), want)
					}
				}
				mu.Lock()
				defer mu.Unlock()
				wantForwardCalls := 2
				if record == "missing resolver" {
					wantForwardCalls = 1
				}
				if forwardCalls != wantForwardCalls || reverseCalls != 2 {
					t.Errorf("forward calls = %d, reverse calls = %d, want %d and 2", forwardCalls, reverseCalls, wantForwardCalls)
				}
			})
		}
	}
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

func TestQNSReverseRouteErrors(t *testing.T) {
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
		{"timeout", http.StatusBadGateway, "QNS resolution failed", "0x539", "0x6000", context.DeadlineExceeded},
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
