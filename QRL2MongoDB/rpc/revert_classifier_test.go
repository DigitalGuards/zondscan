package rpc

import (
	"errors"
	"fmt"
	"testing"
)

// go-qrl answers a bare revert (no return data) as -32000 "execution reverted"
// and a revert with data as code 3. Both must classify as a contract answer;
// node-side failures must stay retryable.
func TestIsConfirmedContractRevert(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"code 3 with reason", &RPCError{Code: 3, Message: "execution reverted: nope"}, true},
		{"bare revert code -32000", &RPCError{Code: -32000, Message: "execution reverted"}, true},
		{"bare revert wrapped", fmt.Errorf("call: %w", &RPCError{Code: -32000, Message: "execution reverted"}), true},
		{"missing state", &RPCError{Code: -32000, Message: "missing historical state"}, false},
		{"historical unavailable", &RPCError{Code: -32000, Message: "historical state unavailable"}, false},
		{"invalid argument", &RPCError{Code: -32602, Message: "invalid argument 0: execution reverted lookalike"}, false},
		{"transport", errors.New("dial tcp: connection refused"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isConfirmedContractRevert(tc.err); got != tc.want {
				t.Fatalf("isConfirmedContractRevert(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}

// A contract without ERC-165 (every plain ERC-20) reverts on supportsInterface
// with the bare -32000 form. Detection must fall through instead of failing.
func TestDetectContractTypeBareRevertOnERC165(t *testing.T) {
	calls := 0
	_, err := detectContractType("Q"+repeatHex("ab", 64), func(_ string, method string) (string, error) {
		calls++
		if len(method) >= len(SIG_SUPPORTS_INTERFACE) && method[:len(SIG_SUPPORTS_INTERFACE)] == SIG_SUPPORTS_INTERFACE {
			return "", &RPCError{Code: -32000, Message: "execution reverted"}
		}
		return "", &RPCError{Code: -32000, Message: "execution reverted"}
	})
	if err != nil {
		t.Fatalf("detection must not fail on bare reverts, got %v", err)
	}
	if calls < 2 {
		t.Fatalf("expected detection to continue past the ERC-165 probes, made %d calls", calls)
	}
}

func repeatHex(pair string, n int) string {
	out := ""
	for i := 0; i < n; i++ {
		out += pair
	}
	return out
}
