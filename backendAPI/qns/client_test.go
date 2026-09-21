package qns

import (
	"backendAPI/db"
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

const checksumA = "QaaaAAaaaAAaAaaAaAAAAaAAAaAaAaAAaAaaAaaaaAAAAAAAAaAAAAaAaAAaaAAaaaaAaAAAAaaAaAAaaaaaaAaAAaaaaAaAaaaaAaaaAAAAaAAAAaAaAaaAAAaAaaaAA"

func rawString(value string) json.RawMessage {
	raw, _ := json.Marshal(value)
	return raw
}

func testConfig(t *testing.T) Config {
	t.Helper()
	values := map[string]string{
		registryEnv: "Q" + strings.Repeat("1", 128),
		chainIDEnv:  "0x539",
	}
	config, err := LoadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	return config
}

func TestLoadConfigRequiresAndCanonicalizesDeployment(t *testing.T) {
	if _, err := LoadConfig(func(string) string { return "" }); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("missing config error = %v, want ErrNotConfigured", err)
	}

	values := map[string]string{
		registryEnv: "0x" + strings.Repeat("a", 128),
		chainIDEnv:  "001337",
	}
	config, err := LoadConfig(func(key string) string { return values[key] })
	if err != nil {
		t.Fatalf("LoadConfig: %v", err)
	}
	if config.registryAddress != checksumA {
		t.Errorf("registry = %q, want %q", config.registryAddress, checksumA)
	}
	if config.expectedChainID != "1337" {
		t.Errorf("chain ID = %q, want 1337", config.expectedChainID)
	}

	invalid := []map[string]string{
		{registryEnv: "Q" + strings.Repeat("0", 128), chainIDEnv: "1337"},
		{registryEnv: "Q" + strings.Repeat("a", 127), chainIDEnv: "1337"},
		{registryEnv: "Q" + strings.Repeat("Ab", 64), chainIDEnv: "1337"},
		{registryEnv: "Q" + strings.Repeat("1", 128), chainIDEnv: "0"},
		{registryEnv: "Q" + strings.Repeat("1", 128), chainIDEnv: "13x7"},
	}
	for _, values := range invalid {
		if _, err := LoadConfig(func(key string) string { return values[key] }); !errors.Is(err, ErrInvalidConfig) {
			t.Errorf("LoadConfig(%v) error = %v, want ErrInvalidConfig", values, err)
		}
	}
}

func TestCheckDeploymentValidatesChainAndRegistryCode(t *testing.T) {
	config := testConfig(t)
	var methods []string
	client := NewClient(func(
		ctx context.Context,
		method string,
		params []interface{},
	) (json.RawMessage, *db.RPCError, error) {
		methods = append(methods, method)
		switch method {
		case "qrl_chainId":
			if len(params) != 0 {
				t.Errorf("chain params = %#v, want empty", params)
			}
			return rawString("0x539"), nil, nil
		case "qrl_getCode":
			want := []interface{}{config.registryAddress, "latest"}
			if !reflect.DeepEqual(params, want) {
				t.Errorf("getCode params = %#v, want %#v", params, want)
			}
			return rawString("0x6000"), nil, nil
		default:
			t.Fatalf("unexpected RPC method %q", method)
			return nil, nil, nil
		}
	})

	deployment, err := client.CheckDeployment(context.Background(), config)
	if err != nil {
		t.Fatalf("CheckDeployment: %v", err)
	}
	if deployment.registryAddress != config.registryAddress || deployment.expectedChainID != "1337" {
		t.Errorf("deployment = %#v, want config values", deployment)
	}
	if want := []string{"qrl_chainId", "qrl_getCode"}; !reflect.DeepEqual(methods, want) {
		t.Errorf("methods = %#v, want %#v", methods, want)
	}
}

func TestCheckDeploymentFailsClosed(t *testing.T) {
	config := testConfig(t)
	cases := []struct {
		name      string
		chain     string
		code      string
		wantError error
	}{
		{"chain mismatch", "0x1", "0x6000", ErrChainMismatch},
		{"empty registry code", "0x539", "0x", ErrRegistryUnavailable},
		{"single zero registry code", "0x539", "0x0", ErrRegistryUnavailable},
		{"zero registry code", "0x539", "0x0000", ErrRegistryUnavailable},
		{"odd registry code", "0x539", "0x1", ErrMalformedResponse},
		{"non-hex registry code", "0x539", "0xzz", ErrMalformedResponse},
		{"malformed chain", "not-a-chain", "0x6000", ErrMalformedResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewClient(func(
				ctx context.Context,
				method string,
				params []interface{},
			) (json.RawMessage, *db.RPCError, error) {
				if method == "qrl_chainId" {
					return rawString(tc.chain), nil, nil
				}
				return rawString(tc.code), nil, nil
			})
			if _, err := client.CheckDeployment(context.Background(), config); !errors.Is(err, tc.wantError) {
				t.Errorf("CheckDeployment error = %v, want %v", err, tc.wantError)
			}
		})
	}
}

func TestRPCFailuresAndMalformedEnvelopesFailClosed(t *testing.T) {
	config := testConfig(t)
	cases := []struct {
		name      string
		call      RPCFunc
		wantError error
	}{
		{
			"transport error",
			func(context.Context, string, []interface{}) (json.RawMessage, *db.RPCError, error) {
				return nil, nil, errors.New("offline")
			},
			ErrRPC,
		},
		{
			"node error",
			func(context.Context, string, []interface{}) (json.RawMessage, *db.RPCError, error) {
				return nil, &db.RPCError{Code: -32000, Message: "failed"}, nil
			},
			ErrRPC,
		},
		{
			"non-string result",
			func(context.Context, string, []interface{}) (json.RawMessage, *db.RPCError, error) {
				return json.RawMessage(`null`), nil, nil
			},
			ErrMalformedResponse,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			client := NewClient(tc.call)
			if _, err := client.CheckDeployment(context.Background(), config); !errors.Is(err, tc.wantError) {
				t.Errorf("CheckDeployment error = %v, want %v", err, tc.wantError)
			}
		})
	}
}

func TestResolveNameUsesSDKCallsAndCanonicalizesAddress(t *testing.T) {
	config := testConfig(t)
	deployment := Deployment{registryAddress: config.registryAddress, expectedChainID: "1337"}
	node := "efe3586aa9a851831a32d38044822af21cc5380e38f05cdc0dd562b4cfada103"
	wantResolverData := "0x0178b8bf" + node + strings.Repeat("0", 64)
	wantAddrData := "0x3b3b57de" + node + strings.Repeat("0", 64)
	resolver := "Q" + strings.Repeat("2", 128)

	var calls []map[string]interface{}
	client := NewClient(func(
		ctx context.Context,
		method string,
		params []interface{},
	) (json.RawMessage, *db.RPCError, error) {
		if method != "qrl_call" {
			t.Fatalf("method = %q, want qrl_call", method)
		}
		call, ok := params[0].(map[string]interface{})
		if !ok {
			t.Fatalf("call param = %#v", params[0])
		}
		calls = append(calls, call)
		if len(calls) == 1 {
			return rawString("0x" + strings.Repeat("2", 128)), nil, nil
		}
		return rawString("0x" + strings.Repeat("Aa", 64)), nil, nil
	})

	resolution, err := client.ResolveName(context.Background(), deployment, "ALICE.QRL")
	if err != nil {
		t.Fatalf("ResolveName: %v", err)
	}
	if resolution.Name != "alice.qrl" || resolution.Address != checksumA || resolution.Missing != MissingNone {
		t.Errorf("resolution = %#v", resolution)
	}
	wantCalls := []map[string]interface{}{
		{"to": config.registryAddress, "data": wantResolverData},
		{"to": resolver, "data": wantAddrData},
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Errorf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestResolveNameFailsClosedOnMissingAndMalformedRecords(t *testing.T) {
	deployment := Deployment{
		registryAddress: "Q" + strings.Repeat("1", 128),
		expectedChainID: "1337",
	}
	cases := []struct {
		name          string
		responses     []string
		wantMissing   MissingRecord
		wantError     error
		wantCallCount int
	}{
		{"zero resolver", []string{"0x"}, MissingResolver, nil, 1},
		{"full zero resolver", []string{"0x" + strings.Repeat("0", 128)}, MissingResolver, nil, 1},
		{"zero address", []string{"0x" + strings.Repeat("2", 128), "0x"}, MissingAddress, nil, 2},
		{"short resolver", []string{"0x12"}, MissingNone, ErrMalformedResponse, 1},
		{"non-hex resolver", []string{"0x" + strings.Repeat("z", 128)}, MissingNone, ErrMalformedResponse, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := NewClient(func(
				ctx context.Context,
				method string,
				params []interface{},
			) (json.RawMessage, *db.RPCError, error) {
				value := tc.responses[calls]
				calls++
				return rawString(value), nil, nil
			})
			resolution, err := client.ResolveName(context.Background(), deployment, "alice.qrl")
			if !errors.Is(err, tc.wantError) {
				t.Errorf("ResolveName error = %v, want %v", err, tc.wantError)
			}
			if resolution.Missing != tc.wantMissing {
				t.Errorf("missing = %q, want %q", resolution.Missing, tc.wantMissing)
			}
			if calls != tc.wantCallCount {
				t.Errorf("calls = %d, want %d", calls, tc.wantCallCount)
			}
		})
	}
}
