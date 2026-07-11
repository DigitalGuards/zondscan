package rpc

import (
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
	// Sanity: selector + padded uint256 = "0x" + 8 + 64 = 74 chars.
	got := expectedOwnerOfCalldata(big.NewInt(1))
	if len(got) != 74 {
		t.Errorf("ownerOf calldata length = %d, want 74 (%q)", len(got), got)
	}
	if !strings.HasPrefix(got, SIG_OWNER_OF) {
		t.Errorf("ownerOf calldata missing selector prefix: %q", got)
	}

	// Sanity: selector + padded address + padded uint256 = "0x" + 8 + 64 + 64 = 138.
	got1155 := expectedBalanceOf1155Calldata(aliceAddr, big.NewInt(1))
	if len(got1155) != 138 {
		t.Errorf("balanceOf(addr,id) calldata length = %d, want 138 (%q)", len(got1155), got1155)
	}
	if !strings.HasPrefix(got1155, SIG_BALANCE_OF_1155) {
		t.Errorf("balanceOf(addr,id) calldata missing selector prefix: %q", got1155)
	}

	// Phase 3b: tokenURI(uint256) selector. Same shape as ownerOf:
	// 8 hex char selector + 64 hex char tokenID word = 72 + "0x" = 74.
	idWord, _ := encodeUint256ForABI(big.NewInt(1))
	tokenURI := SIG_TOKEN_URI + idWord
	if len(tokenURI) != 74 || !strings.HasPrefix(tokenURI, SIG_TOKEN_URI) {
		t.Errorf("tokenURI calldata malformed: %q (len %d)", tokenURI, len(tokenURI))
	}

	// Phase 3b: uri(uint256) selector. Same shape.
	uri1155 := SIG_URI + idWord
	if len(uri1155) != 74 || !strings.HasPrefix(uri1155, SIG_URI) {
		t.Errorf("uri(uint256) calldata malformed: %q (len %d)", uri1155, len(uri1155))
	}
}
