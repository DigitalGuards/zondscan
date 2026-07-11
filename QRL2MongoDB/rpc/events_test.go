package rpc

import (
	"QRL2MongoDB/models"
	"math/big"
	"strings"
	"testing"
)

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
