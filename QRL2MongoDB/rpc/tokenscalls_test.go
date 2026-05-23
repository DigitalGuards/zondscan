package rpc

import (
	"QRL2MongoDB/models"
	"math/big"
	"strings"
	"testing"
)

// Test fixtures: realistic Q-addresses that pass validation.IsValidAddress.
// `topic(addr)` left-pads the 20-byte address into a 32-byte indexed topic.
const (
	aliceAddr = "Q6153d37fa4da7193e6219dcbd2bbe62fa12905b1"
	bobAddr   = "Q539f73306bdd4288f93a5e50b4d5bf1a9b07f147"
	opAddr    = "Qa1b2c3d4e5f60708090a0b0c0d0e0f1011121314"
)

// topic builds an indexed-topic representation of a Q-prefix address:
// "0x" + 24 zero hex chars + 40-char lowercase hex.
func topic(addr string) string {
	stripped := strings.TrimPrefix(strings.ToLower(addr), "q")
	return "0x" + strings.Repeat("0", 24) + stripped
}

// word builds a 32-byte ABI-encoded uint256 word from a hex string (no 0x).
func word(hexStr string) string {
	if len(hexStr) > 64 {
		panic("word > 32 bytes")
	}
	return strings.Repeat("0", 64-len(hexStr)) + hexStr
}

func TestParseERC721Transfer(t *testing.T) {
	tests := []struct {
		name        string
		log         models.Log
		wantFrom    string
		wantTo      string
		wantTokenID string
		wantErr     bool
	}{
		{
			name: "valid Transfer with tokenID=42",
			log: models.Log{
				Topics: []string{
					TransferEventSignature,
					topic(aliceAddr),
					topic(bobAddr),
					"0x" + word("2a"),
				},
				Data: "0x",
			},
			wantFrom:    aliceAddr,
			wantTo:      bobAddr,
			wantTokenID: "42",
		},
		{
			name: "valid Transfer with tokenID=0 (genesis mint)",
			log: models.Log{
				Topics: []string{
					TransferEventSignature,
					"0x" + strings.Repeat("0", 64),
					topic(bobAddr),
					"0x" + word("0"),
				},
				Data: "0x",
			},
			// Zero-address from is canonicalized to Q + 40 zeros.
			wantFrom:    "Q" + strings.Repeat("0", 40),
			wantTo:      bobAddr,
			wantTokenID: "0",
		},
		{
			name: "valid Transfer with large tokenID (uint256 max bits)",
			log: models.Log{
				Topics: []string{
					TransferEventSignature,
					topic(aliceAddr),
					topic(bobAddr),
					"0x" + strings.Repeat("f", 64),
				},
				Data: "0x",
			},
			wantFrom:    aliceAddr,
			wantTo:      bobAddr,
			wantTokenID: new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)).String(),
		},
		{
			name: "wrong topic count (3, looks like ERC-20)",
			log: models.Log{
				Topics: []string{
					TransferEventSignature,
					topic(aliceAddr),
					topic(bobAddr),
				},
				Data: "0x" + word("64"),
			},
			wantErr: true,
		},
		{
			name: "wrong topic count (5)",
			log: models.Log{
				Topics: []string{
					TransferEventSignature,
					topic(aliceAddr),
					topic(bobAddr),
					"0x" + word("1"),
					"0x" + word("2"),
				},
				Data: "0x",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, to, id, err := ParseERC721Transfer(tt.log)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got from=%s to=%s id=%v", from, to, id)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.EqualFold(from, tt.wantFrom) {
				t.Errorf("from = %s, want %s", from, tt.wantFrom)
			}
			if !strings.EqualFold(to, tt.wantTo) {
				t.Errorf("to = %s, want %s", to, tt.wantTo)
			}
			if id.String() != tt.wantTokenID {
				t.Errorf("tokenID = %s, want %s", id.String(), tt.wantTokenID)
			}
		})
	}
}

func TestParseERC1155TransferSingle(t *testing.T) {
	tests := []struct {
		name      string
		log       models.Log
		wantFrom  string
		wantTo    string
		wantID    string
		wantValue string
		wantErr   bool
	}{
		{
			name: "valid TransferSingle id=42 value=100",
			log: models.Log{
				Topics: []string{
					TransferSingleEventSignature,
					topic(opAddr),
					topic(aliceAddr),
					topic(bobAddr),
				},
				Data: "0x" + word("2a") + word("64"),
			},
			wantFrom:  aliceAddr,
			wantTo:    bobAddr,
			wantID:    "42",
			wantValue: "100",
		},
		{
			name: "valid TransferSingle mint (from=0)",
			log: models.Log{
				Topics: []string{
					TransferSingleEventSignature,
					topic(opAddr),
					"0x" + strings.Repeat("0", 64),
					topic(bobAddr),
				},
				Data: "0x" + word("1") + word("3e8"),
			},
			wantFrom:  "Q" + strings.Repeat("0", 40),
			wantTo:    bobAddr,
			wantID:    "1",
			wantValue: "1000",
		},
		{
			name: "wrong topic count",
			log: models.Log{
				Topics: []string{TransferSingleEventSignature, topic(opAddr), topic(aliceAddr)},
				Data:   "0x" + word("2a") + word("64"),
			},
			wantErr: true,
		},
		{
			name: "data too short",
			log: models.Log{
				Topics: []string{
					TransferSingleEventSignature,
					topic(opAddr),
					topic(aliceAddr),
					topic(bobAddr),
				},
				Data: "0x" + word("2a"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, to, id, value, err := ParseERC1155TransferSingle(tt.log)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.EqualFold(from, tt.wantFrom) {
				t.Errorf("from = %s, want %s", from, tt.wantFrom)
			}
			if !strings.EqualFold(to, tt.wantTo) {
				t.Errorf("to = %s, want %s", to, tt.wantTo)
			}
			if id.String() != tt.wantID {
				t.Errorf("id = %s, want %s", id.String(), tt.wantID)
			}
			if value.String() != tt.wantValue {
				t.Errorf("value = %s, want %s", value.String(), tt.wantValue)
			}
		})
	}
}

func TestEncodeAddressForABI(t *testing.T) {
	tests := []struct {
		name string
		addr string
		want string
	}{
		{
			name: "Q-prefix lowercase",
			addr: aliceAddr,
			want: strings.Repeat("0", 24) + "6153d37fa4da7193e6219dcbd2bbe62fa12905b1",
		},
		{
			name: "0x-prefix uppercase canonicalised to lowercase",
			addr: "0x6153D37FA4DA7193E6219DCBD2BBE62FA12905B1",
			want: strings.Repeat("0", 24) + "6153d37fa4da7193e6219dcbd2bbe62fa12905b1",
		},
		{
			name: "no prefix",
			addr: "6153d37fa4da7193e6219dcbd2bbe62fa12905b1",
			want: strings.Repeat("0", 24) + "6153d37fa4da7193e6219dcbd2bbe62fa12905b1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := encodeAddressForABI(tt.addr)
			if got != tt.want {
				t.Errorf("encodeAddressForABI(%s)\n got %s\nwant %s", tt.addr, got, tt.want)
			}
			if len(got) != 64 {
				t.Errorf("encoded length = %d, want 64", len(got))
			}
		})
	}
}

func TestEncodeUint256ForABI(t *testing.T) {
	maxUint256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

	tests := []struct {
		name    string
		v       *big.Int
		want    string
		wantErr bool
	}{
		{
			name: "zero",
			v:    big.NewInt(0),
			want: strings.Repeat("0", 64),
		},
		{
			name: "42",
			v:    big.NewInt(42),
			want: strings.Repeat("0", 62) + "2a",
		},
		{
			name: "uint256 max",
			v:    maxUint256,
			want: strings.Repeat("f", 64),
		},
		{
			name:    "nil",
			v:       nil,
			wantErr: true,
		},
		{
			name:    "negative",
			v:       big.NewInt(-1),
			wantErr: true,
		},
		{
			name:    "exceeds 32 bytes",
			v:       new(big.Int).Lsh(big.NewInt(1), 256),
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := encodeUint256ForABI(tt.v)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestParseAddressFromWord(t *testing.T) {
	aliceRaw := strings.TrimPrefix(strings.ToLower(aliceAddr), "q")

	tests := []struct {
		name     string
		result   string
		wantAddr string
		wantErr  bool
	}{
		{
			name:     "happy path: alice's address padded",
			result:   "0x" + strings.Repeat("0", 24) + aliceRaw,
			wantAddr: aliceAddr,
		},
		{
			name:     "zero address returns empty (no owner)",
			result:   "0x" + strings.Repeat("0", 64),
			wantAddr: "",
		},
		{
			name:     "no 0x prefix accepted",
			result:   strings.Repeat("0", 24) + aliceRaw,
			wantAddr: aliceAddr,
		},
		{
			name:    "too-short response",
			result:  "0x" + aliceRaw,
			wantErr: true,
		},
		{
			name:    "nonzero high bytes (malformed)",
			result:  "0x" + "ff" + strings.Repeat("0", 22) + aliceRaw,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseAddressFromWord(tt.result)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.EqualFold(got, tt.wantAddr) {
				t.Errorf("got %s, want %s", got, tt.wantAddr)
			}
		})
	}
}

func TestParseUint256FromWord(t *testing.T) {
	maxHex := strings.Repeat("f", 64)
	maxUint256 := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1))

	tests := []struct {
		name    string
		result  string
		want    string
		wantErr bool
	}{
		{
			name:   "happy path: 100",
			result: "0x" + word("64"),
			want:   "100",
		},
		{
			name:   "zero",
			result: "0x" + strings.Repeat("0", 64),
			want:   "0",
		},
		{
			name:   "uint256 max",
			result: "0x" + maxHex,
			want:   maxUint256.String(),
		},
		{
			name:    "too short",
			result:  "0x" + word("64")[:32],
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseUint256FromWord(tt.result)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %v", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.String() != tt.want {
				t.Errorf("got %s, want %s", got.String(), tt.want)
			}
		})
	}
}

// Ensure encodeAddressForABI + parseAddressFromWord round-trip cleanly.
func TestAddressWordRoundTrip(t *testing.T) {
	for _, addr := range []string{aliceAddr, bobAddr, opAddr} {
		encoded := encodeAddressForABI(addr)
		decoded, err := parseAddressFromWord("0x" + encoded)
		if err != nil {
			t.Fatalf("decode %s: %v", addr, err)
		}
		if !strings.EqualFold(decoded, addr) {
			t.Errorf("round-trip: got %s, want %s", decoded, addr)
		}
	}
}

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

func TestParseDynamicString(t *testing.T) {
	// Helper: build an ABI-encoded dynamic string from a Go string.
	// Layout: [offset=0x20 || length || data right-padded to 32B].
	encode := func(s string) string {
		raw := []byte(s)
		length := len(raw)
		// Right-pad to next 32-byte boundary.
		padLen := (32 - length%32) % 32
		padded := make([]byte, length+padLen)
		copy(padded, raw)
		offsetWord := word("20")
		lengthWord := word(new(big.Int).SetInt64(int64(length)).Text(16))
		return "0x" + offsetWord + lengthWord + hexEncode(padded)
	}

	tests := []struct {
		name    string
		result  string
		want    string
		wantErr bool
	}{
		{
			name:   "happy path short",
			result: encode("ipfs://Qm.../"),
			want:   "ipfs://Qm.../",
		},
		{
			name:   "happy path 32 bytes exactly",
			result: encode("0123456789abcdef0123456789abcdef"),
			want:   "0123456789abcdef0123456789abcdef",
		},
		{
			name:   "happy path long URI",
			result: encode("https://example.com/collections/my-nfts/metadata.json?v=2026"),
			want:   "https://example.com/collections/my-nfts/metadata.json?v=2026",
		},
		{
			name:   "empty string (method exists, returned empty)",
			result: "0x" + word("20") + word("0"),
			want:   "",
		},
		{
			name:   "all-zero payload returns empty (no error)",
			result: "0x" + strings.Repeat("0", 128),
			want:   "",
		},
		{
			name:    "too-short payload",
			result:  "0x" + word("20"),
			wantErr: true,
		},
		{
			name:    "offset points past payload",
			result:  "0x" + word("ff") + strings.Repeat("0", 64),
			wantErr: true,
		},
		{
			name:    "length exceeds payload",
			result:  "0x" + word("20") + word("ff") + strings.Repeat("00", 16),
			wantErr: true,
		},
		{
			// Adversarial: offset is filled with the maximum signed-int64
			// value (after the leading zero in the 64-hex-char word).
			// IsInt64 passes; without the explicit `offset > len(stripped)`
			// guard, `offset * 2` would overflow to a negative number and
			// the subsequent slice would panic. With the guard, we return
			// a clean error.
			name: "max-int64 offset rejected before multiplication overflow",
			result: "0x" + "0" + strings.Repeat("7", 1) + strings.Repeat("f", 62) +
				strings.Repeat("00", 32),
			wantErr: true,
		},
		{
			// uint256 max as offset trips the IsInt64() branch (>2^63-1).
			name:    "uint256-max offset rejected (overflows int64)",
			result:  "0x" + strings.Repeat("f", 64) + strings.Repeat("00", 32),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseDynamicString(tt.result)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

// hexEncode is an inline lower-case hex.EncodeToString without importing.
func hexEncode(b []byte) string {
	const hexd = "0123456789abcdef"
	out := make([]byte, len(b)*2)
	for i, v := range b {
		out[i*2] = hexd[v>>4]
		out[i*2+1] = hexd[v&0x0f]
	}
	return string(out)
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

func TestParseERC1155TransferBatch(t *testing.T) {
	// Construct data: offsets at words 0-1, ids array at offset 0x40,
	// values array at offset (0x40 + 32 + len_ids*32).
	makeBatchData := func(ids, values []string) string {
		idsOffset := word("40")
		valuesOffset := word(big.NewInt(64 + 32 + int64(len(ids))*32).Text(16))
		idsLen := word(big.NewInt(int64(len(ids))).Text(16))
		valuesLen := word(big.NewInt(int64(len(values))).Text(16))
		buf := idsOffset + valuesOffset + idsLen
		for _, id := range ids {
			buf += word(id)
		}
		buf += valuesLen
		for _, v := range values {
			buf += word(v)
		}
		return "0x" + buf
	}

	tests := []struct {
		name       string
		log        models.Log
		wantFrom   string
		wantTo     string
		wantIDs    []string
		wantValues []string
		wantErr    bool
	}{
		{
			name: "valid batch [1,2,3] / [10,20,30]",
			log: models.Log{
				Topics: []string{
					TransferBatchEventSignature,
					topic(opAddr),
					topic(aliceAddr),
					topic(bobAddr),
				},
				Data: makeBatchData([]string{"1", "2", "3"}, []string{"a", "14", "1e"}),
			},
			wantFrom:   aliceAddr,
			wantTo:     bobAddr,
			wantIDs:    []string{"1", "2", "3"},
			wantValues: []string{"10", "20", "30"},
		},
		{
			name: "valid empty batch",
			log: models.Log{
				Topics: []string{
					TransferBatchEventSignature,
					topic(opAddr),
					topic(aliceAddr),
					topic(bobAddr),
				},
				Data: makeBatchData([]string{}, []string{}),
			},
			wantFrom:   aliceAddr,
			wantTo:     bobAddr,
			wantIDs:    []string{},
			wantValues: []string{},
		},
		{
			name: "ids/values length mismatch (malformed log)",
			log: models.Log{
				Topics: []string{
					TransferBatchEventSignature,
					topic(opAddr),
					topic(aliceAddr),
					topic(bobAddr),
				},
				Data: makeBatchData([]string{"1", "2"}, []string{"a"}),
			},
			wantErr: true,
		},
		{
			name: "wrong topic count",
			log: models.Log{
				Topics: []string{TransferBatchEventSignature, topic(opAddr), topic(aliceAddr)},
				Data:   makeBatchData([]string{"1"}, []string{"a"}),
			},
			wantErr: true,
		},
		{
			name: "array length exceeds cap (adversarial)",
			log: models.Log{
				Topics: []string{
					TransferBatchEventSignature,
					topic(opAddr),
					topic(aliceAddr),
					topic(bobAddr),
				},
				// offset_ids = 0x40, offset_values = 0x60, length_ids = (cap+1), but no data
				Data: "0x" + word("40") + word("60") + word(big.NewInt(maxBatchArrayLen+1).Text(16)),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			from, to, ids, values, err := ParseERC1155TransferBatch(tt.log)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got from=%s to=%s ids=%v values=%v", from, to, ids, values)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.EqualFold(from, tt.wantFrom) {
				t.Errorf("from = %s, want %s", from, tt.wantFrom)
			}
			if !strings.EqualFold(to, tt.wantTo) {
				t.Errorf("to = %s, want %s", to, tt.wantTo)
			}
			if len(ids) != len(tt.wantIDs) {
				t.Fatalf("ids length = %d, want %d", len(ids), len(tt.wantIDs))
			}
			for i, id := range ids {
				if id.String() != tt.wantIDs[i] {
					t.Errorf("ids[%d] = %s, want %s", i, id.String(), tt.wantIDs[i])
				}
			}
			for i, v := range values {
				if v.String() != tt.wantValues[i] {
					t.Errorf("values[%d] = %s, want %s", i, v.String(), tt.wantValues[i])
				}
			}
		})
	}
}
