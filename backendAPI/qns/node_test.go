package qns

import (
	"backendAPI/db"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"
)

// aliceNode is namehash("alice.qrl"), shared with the forward-resolution tests.
const aliceNode = "efe3586aa9a851831a32d38044822af21cc5380e38f05cdc0dd562b4cfada103"

func TestOwnerSelectorMatchesSignature(t *testing.T) {
	sum := keccak256([]byte("owner(bytes32)"))
	if got := hex.EncodeToString(sum[:4]); got != ownerSelector {
		t.Fatalf("owner selector = %s, want %s", ownerSelector, got)
	}
}

func TestParseNodeAcceptsOnly32ByteHex(t *testing.T) {
	node, err := ParseNode("0X" + strings.ToUpper(aliceNode))
	if err != nil {
		t.Fatalf("ParseNode: %v", err)
	}
	if got := hex.EncodeToString(node[:]); got != aliceNode {
		t.Errorf("node = %s, want %s", got, aliceNode)
	}
	for _, input := range []string{
		"", aliceNode, "0x" + aliceNode[:63], "0x" + aliceNode + "0", "0x" + aliceNode[:63] + "g", "Qx" + aliceNode,
	} {
		if _, err := ParseNode(input); !errors.Is(err, ErrInvalidNode) {
			t.Errorf("ParseNode(%q) error = %v, want ErrInvalidNode", input, err)
		}
	}
}

func lookupAliceNode(t *testing.T, responses []string) (NodeRecord, []map[string]interface{}, error) {
	t.Helper()
	deployment := Deployment{registryAddress: "Q" + strings.Repeat("1", 128), expectedChainID: "1337"}
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
	node, err := ParseNode("0x" + aliceNode)
	if err != nil {
		t.Fatal(err)
	}
	record, err := client.LookupNode(context.Background(), deployment, node)
	return record, calls, err
}

func TestLookupNodeVerifiesPublishedNameWithRegistryCalls(t *testing.T) {
	record, calls, err := lookupAliceNode(t, []string{
		"0x" + strings.Repeat("a", 128),
		"0x" + strings.Repeat("2", 128),
		encodedName("ALICE.QRL"),
	})
	if err != nil {
		t.Fatalf("LookupNode: %v", err)
	}
	want := NodeRecord{
		Node:     "0x" + aliceNode,
		Owner:    checksumA,
		Resolver: "Q" + strings.Repeat("2", 128),
		Name:     "alice.qrl",
	}
	if record != want {
		t.Errorf("record = %#v, want %#v", record, want)
	}
	padding := strings.Repeat("0", 64)
	wantCalls := []map[string]interface{}{
		{"to": "Q" + strings.Repeat("1", 128), "data": "0x02571be3" + aliceNode + padding},
		{"to": "Q" + strings.Repeat("1", 128), "data": "0x0178b8bf" + aliceNode + padding},
		{"to": "Q" + strings.Repeat("2", 128), "data": "0x691f3431" + aliceNode + padding},
	}
	if !reflect.DeepEqual(calls, wantCalls) {
		t.Errorf("calls = %#v, want %#v", calls, wantCalls)
	}
}

func TestLookupNodeOmitsMissingAndUnverifiedRecords(t *testing.T) {
	owner := "0x" + strings.Repeat("a", 128)
	resolver := "0x" + strings.Repeat("2", 128)
	withResolver := NodeRecord{Node: "0x" + aliceNode, Owner: checksumA, Resolver: "Q" + strings.Repeat("2", 128)}
	cases := []struct {
		name      string
		responses []string
		want      NodeRecord
	}{
		{"zero owner", []string{"0x" + strings.Repeat("0", 128)}, NodeRecord{Node: "0x" + aliceNode}},
		{"empty owner", []string{"0x"}, NodeRecord{Node: "0x" + aliceNode}},
		{"no resolver", []string{owner, "0x" + strings.Repeat("0", 128)}, NodeRecord{Node: "0x" + aliceNode, Owner: checksumA}},
		{"empty name", []string{owner, resolver, encodedName("")}, withResolver},
		{"empty return", []string{owner, resolver, "0x"}, withResolver},
		{"malformed name", []string{owner, resolver, "0x12"}, withResolver},
		{"other name", []string{owner, resolver, encodedName("bob.qrl")}, withResolver},
		{"invalid name", []string{owner, resolver, encodedName("alice.eth")}, withResolver},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			record, calls, err := lookupAliceNode(t, tc.responses)
			if err != nil {
				t.Fatalf("LookupNode: %v", err)
			}
			if record != tc.want {
				t.Errorf("record = %#v, want %#v", record, tc.want)
			}
			if len(calls) != len(tc.responses) {
				t.Errorf("calls = %d, want %d", len(calls), len(tc.responses))
			}
		})
	}
}

func TestLookupNodeClassifiesFailures(t *testing.T) {
	deployment := Deployment{registryAddress: "Q" + strings.Repeat("1", 128), expectedChainID: "1337"}
	node, _ := ParseNode("0x" + aliceNode)
	owner := "0x" + strings.Repeat("a", 128)
	resolver := "0x" + strings.Repeat("2", 128)
	revert := &db.RPCError{Code: 3}
	cases := []struct {
		name    string
		failAt  int
		rpcErr  *db.RPCError
		raw     string
		wantErr error
	}{
		{"owner revert", 0, revert, "", ErrRegistryLookup},
		{"owner malformed", 0, nil, "0x12", ErrRegistryLookup},
		{"resolver node error", 1, &db.RPCError{Code: -32000, Message: "state unavailable"}, "", ErrRegistryLookup},
		{"name revert", 2, revert, "", nil},
		{"name node error", 2, &db.RPCError{Code: -32000, Message: "state unavailable"}, "", ErrRPC},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			responses := []string{owner, resolver, encodedName("alice.qrl")}
			calls := 0
			client := NewClient(func(context.Context, string, []interface{}) (json.RawMessage, *db.RPCError, error) {
				step := calls
				calls++
				if step == tc.failAt {
					if tc.rpcErr != nil {
						return nil, tc.rpcErr, nil
					}
					return rawString(tc.raw), nil, nil
				}
				return rawString(responses[step]), nil, nil
			})
			record, err := client.LookupNode(context.Background(), deployment, node)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("error = %v, want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("LookupNode: %v", err)
			}
			if record.Owner != checksumA || record.Name != "" {
				t.Errorf("record = %#v", record)
			}
		})
	}
}

func TestLookupNodeRequiresCheckedDeployment(t *testing.T) {
	client := NewClient(func(context.Context, string, []interface{}) (json.RawMessage, *db.RPCError, error) {
		t.Fatal("unexpected RPC call")
		return nil, nil, nil
	})
	if _, err := client.LookupNode(context.Background(), Deployment{}, [32]byte{}); !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("error = %v, want ErrNotConfigured", err)
	}
}
