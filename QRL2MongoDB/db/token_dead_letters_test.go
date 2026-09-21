package db

import (
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"strings"
	"testing"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/bson"
)

func tokenDeadLetterFixture() models.TokenEventDeadLetter {
	return models.TokenEventDeadLetter{
		BlockNumber:   "0x2a",
		BlockHash:     "0x" + strings.Repeat("a", 64),
		TxHash:        "0x" + strings.Repeat("b", 64),
		LogIndex:      "0x0",
		Emitter:       "Q" + strings.Repeat("c", 128),
		Topic0:        rpc.TransferEventSignature,
		TokenStandard: rpc.StandardERC20,
		Reason:        "deterministic token event decode rejected: malformed ABI payload",
	}
}

func TestNormalizeTokenEventDeadLetterCanonicalizesAndBoundsReason(t *testing.T) {
	deadLetter := tokenDeadLetterFixture()
	deadLetter.BlockNumber = "0x02A"
	deadLetter.BlockHash = strings.ToUpper(deadLetter.BlockHash[:2]) + strings.ToUpper(deadLetter.BlockHash[2:])
	deadLetter.LogIndex = "0x00"
	deadLetter.Emitter = strings.ToLower(deadLetter.Emitter)
	deadLetter.Reason = strings.Repeat("x", tokenEventDeadLetterReasonLimit-1) + "é"
	got, err := normalizeTokenEventDeadLetter(deadLetter)
	if err != nil {
		t.Fatal(err)
	}
	if got.BlockNumber != "0x2a" || got.BlockNumberInt != 42 ||
		got.BlockHash != strings.ToLower(deadLetter.BlockHash) || got.LogIndex != "0x0" ||
		got.Emitter != "Q"+strings.Repeat("c", 128) {
		t.Fatalf("normalized dead-letter = %+v", got)
	}
	if len(got.Reason) > tokenEventDeadLetterReasonLimit || !utf8.ValidString(got.Reason) {
		t.Fatalf("bounded reason length=%d valid=%v", len(got.Reason), utf8.ValidString(got.Reason))
	}
}

func TestTokenEventDeadLetterIndexesCoverIdentityAndRollback(t *testing.T) {
	models := tokenEventDeadLetterIndexModels()
	if len(models) != 2 {
		t.Fatalf("dead-letter index count = %d, want 2", len(models))
	}
	identity := models[0]
	if identity.Options == nil || identity.Options.Name == nil ||
		*identity.Options.Name != "token_event_dead_letter_identity_idx" ||
		identity.Options.Unique == nil || !*identity.Options.Unique {
		t.Fatalf("identity index options = %#v", identity.Options)
	}
	identityKeys, ok := identity.Keys.(bson.D)
	if !ok || len(identityKeys) != 3 ||
		identityKeys[0].Key != "blockHash" ||
		identityKeys[1].Key != "txHash" ||
		identityKeys[2].Key != "logIndex" {
		t.Fatalf("identity index keys = %#v", identity.Keys)
	}
	rollback := models[1]
	if rollback.Options == nil || rollback.Options.Name == nil ||
		*rollback.Options.Name != "token_event_dead_letter_rollback_idx" {
		t.Fatalf("rollback index options = %#v", rollback.Options)
	}
	rollbackKeys, ok := rollback.Keys.(bson.D)
	if !ok || len(rollbackKeys) != 2 ||
		rollbackKeys[0].Key != "blockNumberInt" || rollbackKeys[1].Key != "blockHash" {
		t.Fatalf("rollback index keys = %#v", rollback.Keys)
	}
}
