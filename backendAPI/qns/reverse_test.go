package qns

import (
	"backendAPI/db"
	"backendAPI/qrladdress"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

const reverseVectorAddress = "Q000102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f202122232425262728292a2b2c2d2e2f303132333435363738393a3b3c3d3e3f"
const reverseVectorNode = "cb93c352dcd0fe10fcde64cb2ca5a1a307a9b6f4757ed853bc51c96b82837072"

func encodedName(name string) string {
	padding := (64 - len(name)%64) % 64
	return fmt.Sprintf("0x%0128x%0128x", 64, len(name)) +
		hex.EncodeToString([]byte(name)) + strings.Repeat("00", padding)
}

func TestReverseNodeMatchesSDKVector(t *testing.T) {
	for _, alias := range []string{
		reverseVectorAddress,
		"0x" + reverseVectorAddress[1:],
		strings.ToUpper(reverseVectorAddress),
	} {
		canonical, ok := qrladdress.Canonicalize(alias)
		if !ok {
			t.Fatalf("invalid vector alias %q", alias)
		}
		label := sha3HexAddress(canonical)
		if got, want := hex.EncodeToString(label[:]), "a8b834d2d5a6790720953a9c023c9f92b573d14f4445cbee0c9a73ec94dbb1ca"; got != want {
			t.Errorf("sha3HexAddress = %s, want %s", got, want)
		}
		node := reverseNodeFor(canonical)
		if got := hex.EncodeToString(node[:]); got != reverseVectorNode {
			t.Errorf("reverseNodeFor = %s, want %s", got, reverseVectorNode)
		}
	}
}

func TestReverseNameVerifiesForwardRecordWithSDKCalls(t *testing.T) {
	config := testConfig(t)
	deployment := Deployment{registryAddress: config.registryAddress, expectedChainID: "1337"}
	responses := []string{
		"0x" + strings.Repeat("2", 128),
		encodedName("ALICE.QRL"),
		"0x" + strings.Repeat("3", 128),
		"0x" + strings.ToUpper(reverseVectorAddress[1:]),
	}
	var calls []map[string]interface{}
	client := NewClient(func(ctx context.Context, method string, params []interface{}) (json.RawMessage, *db.RPCError, error) {
		if method != "qrl_call" || len(params) != 2 || params[1] != "latest" {
			t.Fatalf("unexpected RPC call: %s %#v", method, params)
		}
		call, ok := params[0].(map[string]interface{})
		if !ok || len(calls) >= len(responses) {
			t.Fatalf("unexpected call: %#v", params)
		}
		result := responses[len(calls)]
		calls = append(calls, call)
		return rawString(result), nil, nil
	})

	resolution, err := client.ReverseName(context.Background(), deployment, "0X"+reverseVectorAddress[1:])
	if err != nil {
		t.Fatalf("ReverseName: %v", err)
	}
	canonical, _ := qrladdress.Canonicalize(reverseVectorAddress)
	if want := (Resolution{Name: "alice.qrl", Address: canonical}); resolution != want {
		t.Errorf("resolution = %#v, want %#v", resolution, want)
	}
	forwardNode := "efe3586aa9a851831a32d38044822af21cc5380e38f05cdc0dd562b4cfada103"
	padding := strings.Repeat("0", 64)
	wantCalls := []map[string]interface{}{
		{"to": config.registryAddress, "data": "0x0178b8bf" + reverseVectorNode + padding},
		{"to": "Q" + strings.Repeat("2", 128), "data": "0x691f3431" + reverseVectorNode + padding},
		{"to": config.registryAddress, "data": "0x0178b8bf" + forwardNode + padding},
		{"to": "Q" + strings.Repeat("3", 128), "data": "0x3b3b57de" + forwardNode + padding},
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Errorf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestReverseNameOmitsUnverifiedNames(t *testing.T) {
	resolver := "0x" + strings.Repeat("2", 128)
	cases := []struct {
		name      string
		responses []string
		missing   MissingRecord
	}{
		{"no resolver", []string{"0x"}, MissingResolver},
		{"zero resolver", []string{"0x" + strings.Repeat("0", 128)}, MissingResolver},
		{"empty name", []string{resolver, encodedName("")}, MissingName},
		{"empty return", []string{resolver, "0x"}, MissingName},
		{"invalid name", []string{resolver, encodedName("alice.eth")}, MissingName},
		{"unicode name", []string{resolver, encodedName("café.qrl")}, MissingName},
		{"oversized name", []string{resolver, encodedName(strings.Repeat("a", MaxNameBytes) + ".qrl")}, MissingName},
		{"no forward resolver", []string{resolver, encodedName("alice.qrl"), "0x"}, MissingName},
		{"no forward address", []string{resolver, encodedName("alice.qrl"), resolver, "0x"}, MissingName},
		{"forward mismatch", []string{resolver, encodedName("alice.qrl"), resolver, "0x" + strings.Repeat("b", 128)}, MissingName},
	}
	deployment := Deployment{registryAddress: "Q" + strings.Repeat("1", 128), expectedChainID: "1337"}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			client := NewClient(func(context.Context, string, []interface{}) (json.RawMessage, *db.RPCError, error) {
				if calls >= len(tc.responses) {
					t.Fatal("unexpected RPC call")
				}
				result := tc.responses[calls]
				calls++
				return rawString(result), nil, nil
			})
			resolution, err := client.ReverseName(context.Background(), deployment, "0x"+strings.Repeat("a", 128))
			if err != nil {
				t.Fatalf("ReverseName: %v", err)
			}
			if want := (Resolution{Address: checksumA, Missing: tc.missing}); resolution != want {
				t.Errorf("resolution = %#v, want %#v", resolution, want)
			}
			if calls != len(tc.responses) {
				t.Errorf("calls = %d, want %d", calls, len(tc.responses))
			}
		})
	}
}

func TestReverseNameSeparatesRegistryAndRecordFailures(t *testing.T) {
	deployment := Deployment{registryAddress: "Q" + strings.Repeat("1", 128), expectedChainID: "1337"}
	responses := []string{"0x" + strings.Repeat("2", 128), encodedName("alice.qrl"), "0x" + strings.Repeat("3", 128)}
	for _, failure := range []string{"transport", "node", "revert", "malformed", "canceled"} {
		for failAt := 0; failAt < 4; failAt++ {
			t.Run(fmt.Sprintf("%s/call%d", failure, failAt), func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				calls := 0
				client := NewClient(func(context.Context, string, []interface{}) (json.RawMessage, *db.RPCError, error) {
					index := calls
					calls++
					if index == failAt {
						switch failure {
						case "transport":
							return nil, nil, errors.New("offline")
						case "node":
							return nil, &db.RPCError{Code: -32000}, nil
						case "revert":
							return nil, &db.RPCError{Code: 3}, nil
						case "malformed":
							return rawString("0x12"), nil, nil
						default:
							cancel()
							return nil, nil, ctx.Err()
						}
					}
					return rawString(responses[index]), nil, nil
				})
				resolution, err := client.ReverseName(ctx, deployment, checksumA)
				wantError := ErrRPC
				if failure == "malformed" {
					wantError = ErrMalformedResponse
				} else if failure == "canceled" {
					wantError = context.Canceled
				}
				if failAt == 1 || failAt == 3 {
					if want := (Resolution{Address: checksumA, Missing: MissingName}); err != nil || resolution != want {
						t.Errorf("ReverseName = %#v, %v, want %#v and no error", resolution, err, want)
					}
				} else if !errors.Is(err, wantError) || resolution.Name != "" {
					t.Errorf("ReverseName = %#v, %v, want empty name and %v", resolution, err, wantError)
				}
				if calls != failAt+1 {
					t.Errorf("calls = %d, want %d", calls, failAt+1)
				}
			})
		}
	}
}

func TestReverseNameValidatesBeforeRPC(t *testing.T) {
	client := NewClient(func(context.Context, string, []interface{}) (json.RawMessage, *db.RPCError, error) {
		t.Fatal("unexpected RPC call")
		return nil, nil, nil
	})
	deployment := Deployment{registryAddress: "Q" + strings.Repeat("1", 128), expectedChainID: "1337"}
	for _, address := range []string{"", "alice.qrl", "Q1234", "Q" + strings.Repeat("Ab", 64), "Q" + strings.Repeat("z", 128)} {
		if _, err := client.ReverseName(context.Background(), deployment, address); !errors.Is(err, ErrInvalidAddress) {
			t.Errorf("ReverseName(%q) error = %v, want ErrInvalidAddress", address, err)
		}
	}
	if _, err := client.ReverseName(context.Background(), Deployment{}, checksumA); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("unchecked deployment error = %v, want ErrNotConfigured", err)
	}
}

func TestDecodeNameValidatesQRVM64String(t *testing.T) {
	for _, name := range []string{"", "alice.qrl", strings.Repeat("a", 64), strings.Repeat("b", 65)} {
		if got, err := decodeName(encodedName(name)); err != nil || got != name {
			t.Errorf("decodeName = %q, %v, want %q", got, err, name)
		}
	}
	valid := encodedName("alice.qrl")
	for _, value := range []string{
		"",
		"0xz0",
		"0x1",
		valid[2:],
		"0x" + strings.Repeat("0", 128) + valid[130:],
		"0x" + strings.Repeat("f", 128) + valid[130:],
		valid[:130] + strings.Repeat("f", 128) + valid[258:],
		valid[:len(valid)-2],
		valid[:len(valid)-2] + "01",
		valid + strings.Repeat("0", 128),
	} {
		if _, err := decodeName(value); !errors.Is(err, ErrMalformedResponse) {
			t.Errorf("decodeName(%q) error = %v, want ErrMalformedResponse", value, err)
		}
	}
}
