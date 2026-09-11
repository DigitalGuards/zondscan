package rpc

import (
	"errors"
	"math/big"
	"strings"
	"testing"
)

// Helper to construct calldata that GetERC721Owner / GetERC1155Balance would
// emit. Used below by tests that verify the request payload (selector +
// padding) the helpers send to CallContractMethod.
func expectedOwnerOfCalldata(id *big.Int) string {
	w, _ := encodeUint256ForABI(id)
	return SIG_OWNER_OF + w
}

func TestNFTBalanceCallsDistinguishRevertFromRetryableRPCError(t *testing.T) {
	nodeErr := &RPCError{Code: -32000, Message: "historical state unavailable"}
	if _, err := getERC721OwnerWithCaller(
		opAddr,
		big.NewInt(7),
		func(string, string) (string, error) { return "", nodeErr },
	); !errors.Is(err, nodeErr) {
		t.Fatalf("ERC-721 node error = %v, want %v", err, nodeErr)
	}
	if owner, err := getERC721OwnerWithCaller(
		opAddr,
		big.NewInt(7),
		func(string, string) (string, error) {
			return "", &RPCError{Code: 3, Message: "execution reverted"}
		},
	); err != nil || owner != "" {
		t.Fatalf("ERC-721 revert = %q, %v", owner, err)
	}
	if owner, err := getERC721OwnerWithCaller(
		opAddr,
		big.NewInt(7),
		func(_ string, calldata string) (string, error) {
			if calldata != expectedOwnerOfCalldata(big.NewInt(7)) {
				t.Fatalf("ownerOf calldata = %s", calldata)
			}
			return "0x" + strings.TrimPrefix(aliceAddr, "Q"), nil
		},
	); err != nil || owner != aliceAddr {
		t.Fatalf("ERC-721 owner = %q, %v", owner, err)
	}
	if _, err := getERC721OwnerWithCaller(
		opAddr,
		big.NewInt(7),
		func(string, string) (string, error) {
			return "0x" + word("0") + strings.TrimPrefix(aliceAddr, "Q"), nil
		},
	); err == nil {
		t.Fatal("ERC-721 owner accepted a trailing ABI word")
	}

	if _, err := getERC1155BalanceWithCaller(
		opAddr,
		aliceAddr,
		big.NewInt(9),
		func(string, string) (string, error) { return "", nodeErr },
	); !errors.Is(err, nodeErr) {
		t.Fatalf("ERC-1155 node error = %v, want %v", err, nodeErr)
	}
	if balance, err := getERC1155BalanceWithCaller(
		opAddr,
		aliceAddr,
		big.NewInt(9),
		func(string, string) (string, error) {
			return "", &RPCError{Code: 3, Message: "execution reverted"}
		},
	); err != nil || balance.Sign() != 0 {
		t.Fatalf("ERC-1155 revert = %v, %v", balance, err)
	}
	if balance, err := getERC1155BalanceWithCaller(
		opAddr,
		aliceAddr,
		big.NewInt(9),
		func(_ string, calldata string) (string, error) {
			if calldata != expectedBalanceOf1155Calldata(aliceAddr, big.NewInt(9)) {
				t.Fatalf("ERC-1155 calldata = %s", calldata)
			}
			return "0x" + word("2a"), nil
		},
	); err != nil || balance.String() != "42" {
		t.Fatalf("ERC-1155 balance = %v, %v", balance, err)
	}
	if _, err := getERC1155BalanceWithCaller(
		opAddr,
		aliceAddr,
		big.NewInt(9),
		func(string, string) (string, error) {
			return "0x" + word("0") + word("2a"), nil
		},
	); err == nil {
		t.Fatal("ERC-1155 balance accepted a trailing ABI word")
	}
}

func expectedBalanceOf1155Calldata(holder string, id *big.Int) string {
	w, _ := encodeUint256ForABI(id)
	return SIG_BALANCE_OF_1155 + encodeAddressForABI(holder) + w
}

func TestSubstituteERC1155IDTemplate(t *testing.T) {
	tests := []struct {
		name    string
		uri     string
		tokenID *big.Int
		want    string
	}{
		{
			name:    "spec happy path: id=42 padded to 64 hex chars",
			uri:     "ipfs://Qm.../{id}.json",
			tokenID: big.NewInt(42),
			want:    "ipfs://Qm.../" + strings.Repeat("0", 62) + "2a" + ".json",
		},
		{
			name:    "id=0 fully zero-padded",
			uri:     "https://api.example.com/metadata/{id}",
			tokenID: big.NewInt(0),
			want:    "https://api.example.com/metadata/" + strings.Repeat("0", 64),
		},
		{
			name:    "uint256 max stays exactly 64 chars",
			uri:     "ipfs://{id}",
			tokenID: new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)),
			want:    "ipfs://" + strings.Repeat("f", 64),
		},
		{
			name:    "multiple placeholders all replaced",
			uri:     "https://h.test/{id}/contents/{id}.png",
			tokenID: big.NewInt(1),
			want:    "https://h.test/" + strings.Repeat("0", 63) + "1/contents/" + strings.Repeat("0", 63) + "1.png",
		},
		{
			name:    "no placeholder, URI unchanged",
			uri:     "ipfs://Qm.../fixed",
			tokenID: big.NewInt(1),
			want:    "ipfs://Qm.../fixed",
		},
		{
			name:    "empty URI stays empty",
			uri:     "",
			tokenID: big.NewInt(1),
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := substituteERC1155IDTemplate(tt.uri, tt.tokenID)
			if got != tt.want {
				t.Errorf("got %q\nwant %q", got, tt.want)
			}
		})
	}
}

func TestExpectedCalldataSelectors(t *testing.T) {
	// Sanity: selector + one 128-hex ABI word = 138 characters.
	got := expectedOwnerOfCalldata(big.NewInt(1))
	if len(got) != 138 {
		t.Errorf("ownerOf calldata length = %d, want 138 (%q)", len(got), got)
	}
	if !strings.HasPrefix(got, SIG_OWNER_OF) {
		t.Errorf("ownerOf calldata missing selector prefix: %q", got)
	}

	// Sanity: selector + address word + uint256 word = 266 characters.
	got1155 := expectedBalanceOf1155Calldata(aliceAddr, big.NewInt(1))
	if len(got1155) != 266 {
		t.Errorf("balanceOf(addr,id) calldata length = %d, want 266 (%q)", len(got1155), got1155)
	}
	if !strings.HasPrefix(got1155, SIG_BALANCE_OF_1155) {
		t.Errorf("balanceOf(addr,id) calldata missing selector prefix: %q", got1155)
	}

	// Phase 3b: tokenURI(uint256) selector. Same shape as ownerOf.
	idWord, _ := encodeUint256ForABI(big.NewInt(1))
	tokenURI := SIG_TOKEN_URI + idWord
	if len(tokenURI) != 138 || !strings.HasPrefix(tokenURI, SIG_TOKEN_URI) {
		t.Errorf("tokenURI calldata malformed: %q (len %d)", tokenURI, len(tokenURI))
	}

	// Phase 3b: uri(uint256) selector. Same shape.
	uri1155 := SIG_URI + idWord
	if len(uri1155) != 138 || !strings.HasPrefix(uri1155, SIG_URI) {
		t.Errorf("uri(uint256) calldata malformed: %q (len %d)", uri1155, len(uri1155))
	}
}
