package rpc

import (
	"errors"
	"strings"
	"testing"
)

func TestGetTokenInfoStrictSeparatesUnsupportedAndRetryableFailures(t *testing.T) {
	transportErr := errors.New("temporary transport failure")
	tests := []struct {
		name      string
		call      contractMethodCaller
		wantToken bool
		wantErr   error
	}{
		{
			name: "canonical ERC20",
			call: func(_ string, method string) (string, error) {
				switch method {
				case SIG_SUPPLY:
					return "0x" + word("64"), nil
				case SIG_BALANCE + encodeAddressForABI(aliceAddr):
					return "0x" + word("0"), nil
				case SIG_NAME:
					return encodeDynamicResult("Quanta"), nil
				case SIG_SYMBOL:
					return encodeDynamicResult("QTA"), nil
				case SIG_DECIMALS:
					return "0x" + word("12"), nil
				default:
					t.Fatalf("unexpected method %s", method)
					return "", nil
				}
			},
			wantToken: true,
		},
		{
			name: "confirmed contract revert",
			call: func(_ string, _ string) (string, error) {
				return "", &RPCError{Code: 3, Message: "execution reverted"}
			},
		},
		{
			name: "malformed unsupported ABI",
			call: func(_ string, _ string) (string, error) {
				return "0x12", nil
			},
		},
		{
			name: "transport failure",
			call: func(_ string, method string) (string, error) {
				switch method {
				case SIG_SUPPLY:
					return "0x" + word("64"), nil
				case SIG_BALANCE + encodeAddressForABI(aliceAddr):
					return "0x" + word("0"), nil
				}
				return "", transportErr
			},
			wantErr: transportErr,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, _, isToken, err := getTokenInfoStrict(aliceAddr, test.call)
			if !errors.Is(err, test.wantErr) {
				t.Fatalf("error = %v, want %v", err, test.wantErr)
			}
			if isToken != test.wantToken {
				t.Fatalf("isToken = %v, want %v", isToken, test.wantToken)
			}
		})
	}
}

func TestOptionalTokenMetadataPropagatesOnlyTransportFailures(t *testing.T) {
	transportErr := errors.New("RPC unavailable")
	if _, err := optionalTokenString(
		"Qtoken",
		SIG_NAME,
		decodeTokenName,
		func(string, string) (string, error) { return "", transportErr },
	); !errors.Is(err, transportErr) {
		t.Fatalf("transport error = %v, want %v", err, transportErr)
	}
	if value, err := optionalTokenString(
		"Qtoken",
		SIG_NAME,
		decodeTokenName,
		func(string, string) (string, error) {
			return "", &RPCError{Code: 3, Message: "execution reverted"}
		},
	); err != nil || value != "" {
		t.Fatalf("revert result = %q, %v", value, err)
	}
	if value, err := optionalTokenString(
		"Qtoken",
		SIG_NAME,
		decodeTokenName,
		func(string, string) (string, error) { return "0x1", nil },
	); err != nil || value != "" {
		t.Fatalf("malformed result = %q, %v", value, err)
	}
	nodeErr := &RPCError{Code: -32000, Message: "historical state unavailable"}
	if _, err := optionalTokenString(
		"Qtoken",
		SIG_NAME,
		decodeTokenName,
		func(string, string) (string, error) { return "", nodeErr },
	); !errors.Is(err, nodeErr) {
		t.Fatalf("node RPC error = %v, want retryable %v", err, nodeErr)
	}
}

func TestGetTokenInfoStrictAcceptsMetadataLightERC20(t *testing.T) {
	name, symbol, decimals, isToken, err := getTokenInfoStrict(aliceAddr,
		func(_ string, method string) (string, error) {
			switch method {
			case SIG_SUPPLY:
				return "0x" + word("64"), nil
			case SIG_BALANCE + encodeAddressForABI(aliceAddr):
				return "0x" + word("0"), nil
			default:
				return "", &RPCError{Code: 3, Message: "execution reverted"}
			}
		})
	if err != nil || !isToken || name != "" || symbol != "" || decimals != 0 {
		t.Fatalf("metadata-light ERC20 = %q %q %d %v %v", name, symbol, decimals, isToken, err)
	}
}

func TestExactBlockContractCallParamsUseHashSelector(t *testing.T) {
	blockHash := "0x" + strings.Repeat("a", 64)
	params, err := exactBlockContractCallParams(aliceAddr, SIG_SUPPLY, blockHash)
	if err != nil {
		t.Fatal(err)
	}
	if len(params) != 2 {
		t.Fatalf("params = %#v", params)
	}
	call, ok := params[0].(map[string]string)
	if !ok || call["to"] != aliceAddr || call["data"] != SIG_SUPPLY {
		t.Fatalf("call params = %#v", params[0])
	}
	selector, ok := params[1].(map[string]interface{})
	if !ok || selector["blockHash"] != blockHash || selector["requireCanonical"] != false {
		t.Fatalf("block selector = %#v", params[1])
	}
}

func TestSupportsInterfacePropagatesNonRevertRPCError(t *testing.T) {
	nodeErr := &RPCError{Code: -32000, Message: "missing historical state"}
	if _, _, err := supportsInterfaceWithCaller(
		aliceAddr,
		InterfaceIDERC721,
		func(string, string) (string, error) { return "", nodeErr },
	); !errors.Is(err, nodeErr) {
		t.Fatalf("supportsInterface error = %v, want %v", err, nodeErr)
	}
	if supports, hasERC165, err := supportsInterfaceWithCaller(
		aliceAddr,
		InterfaceIDERC721,
		func(string, string) (string, error) {
			return "", &RPCError{Code: 3, Message: "execution reverted"}
		},
	); err != nil || supports || hasERC165 {
		t.Fatalf("confirmed revert = %v %v %v", supports, hasERC165, err)
	}
}
