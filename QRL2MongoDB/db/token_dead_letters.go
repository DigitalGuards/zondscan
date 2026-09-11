package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/rpc"
	"QRL2MongoDB/validation"
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const tokenEventDeadLetterReasonLimit = 1024

func tokenEventDeadLetterCollection() *mongo.Collection {
	return configs.GetTokenEventDeadLettersCollection()
}

func boundedTokenEventDeadLetterReason(reason string) string {
	if len(reason) <= tokenEventDeadLetterReasonLimit {
		return reason
	}
	reason = reason[:tokenEventDeadLetterReasonLimit]
	for !utf8.ValidString(reason) {
		reason = reason[:len(reason)-1]
	}
	return reason
}

func normalizeTokenEventDeadLetter(deadLetter models.TokenEventDeadLetter) (models.TokenEventDeadLetter, error) {
	var err error
	deadLetter.BlockNumber, err = canonicalHexQuantity(
		deadLetter.BlockNumber,
		"token event dead-letter block number",
	)
	if err != nil {
		return models.TokenEventDeadLetter{}, err
	}
	blockNumber := new(big.Int)
	if _, ok := blockNumber.SetString(strings.TrimPrefix(deadLetter.BlockNumber, "0x"), 16); !ok ||
		!blockNumber.IsInt64() {
		return models.TokenEventDeadLetter{}, fmt.Errorf(
			"token event dead-letter block number exceeds int64: %s",
			deadLetter.BlockNumber,
		)
	}
	deadLetter.BlockNumberInt = blockNumber.Int64()
	deadLetter.BlockHash, err = canonicalFixedHash(
		deadLetter.BlockHash,
		"token event dead-letter block hash",
	)
	if err != nil {
		return models.TokenEventDeadLetter{}, err
	}
	deadLetter.TxHash, err = canonicalFixedHash(
		deadLetter.TxHash,
		"token event dead-letter transaction hash",
	)
	if err != nil {
		return models.TokenEventDeadLetter{}, err
	}
	deadLetter.LogIndex, err = canonicalHexQuantity(
		deadLetter.LogIndex,
		"token event dead-letter log index",
	)
	if err != nil {
		return models.TokenEventDeadLetter{}, err
	}
	deadLetter.Emitter = validation.ConvertToQAddress(deadLetter.Emitter)
	if !validation.IsValidAddress(deadLetter.Emitter) {
		return models.TokenEventDeadLetter{}, fmt.Errorf(
			"invalid token event dead-letter emitter: %s",
			deadLetter.Emitter,
		)
	}
	deadLetter.Topic0 = strings.ToLower(deadLetter.Topic0)
	if err := validation.ValidateHexString(deadLetter.Topic0, validation.AddressLength); err != nil {
		return models.TokenEventDeadLetter{}, fmt.Errorf("invalid token event dead-letter topic0: %w", err)
	}
	switch deadLetter.Topic0 {
	case rpc.TransferEventSignature, rpc.TransferSingleEventSignature, rpc.TransferBatchEventSignature:
	default:
		return models.TokenEventDeadLetter{}, fmt.Errorf(
			"unexpected token event dead-letter topic0: %s",
			deadLetter.Topic0,
		)
	}
	switch deadLetter.TokenStandard {
	case rpc.StandardERC20, rpc.StandardERC721, rpc.StandardERC1155:
	default:
		return models.TokenEventDeadLetter{}, fmt.Errorf(
			"unexpected token event dead-letter standard: %s",
			deadLetter.TokenStandard,
		)
	}
	deadLetter.Reason = boundedTokenEventDeadLetterReason(deadLetter.Reason)
	if deadLetter.Reason == "" {
		return models.TokenEventDeadLetter{}, fmt.Errorf("token event dead-letter reason is empty")
	}
	return deadLetter, nil
}

// StoreTokenEventDeadLetter durably quarantines one immutable malformed token
// event. MongoDB server time owns both audit timestamps. Replays update the
// same exact identity and preserve its original first-seen time.
func StoreTokenEventDeadLetter(deadLetter models.TokenEventDeadLetter) error {
	deadLetter, err := normalizeTokenEventDeadLetter(deadLetter)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	filter := bson.M{
		"blockHash": deadLetter.BlockHash,
		"txHash":    deadLetter.TxHash,
		"logIndex":  deadLetter.LogIndex,
	}
	update := mongo.Pipeline{
		bson.D{{Key: "$set", Value: bson.M{
			"blockNumber":    deadLetter.BlockNumber,
			"blockNumberInt": deadLetter.BlockNumberInt,
			"blockHash":      deadLetter.BlockHash,
			"txHash":         deadLetter.TxHash,
			"logIndex":       deadLetter.LogIndex,
			"emitter":        deadLetter.Emitter,
			"topic0":         deadLetter.Topic0,
			"tokenStandard":  deadLetter.TokenStandard,
			"reason":         deadLetter.Reason,
			"firstSeenAt": bson.M{
				"$ifNull": bson.A{"$firstSeenAt", "$$NOW"},
			},
			"lastSeenAt": "$$NOW",
		}}},
	}
	if _, err := tokenEventDeadLetterCollection().UpdateOne(
		ctx,
		filter,
		update,
		options.Update().SetUpsert(true),
	); err != nil {
		return fmt.Errorf(
			"store token event dead-letter %s %s %s: %w",
			deadLetter.BlockHash,
			deadLetter.TxHash,
			deadLetter.LogIndex,
			err,
		)
	}
	return nil
}

func tokenEventDeadLetterIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "blockHash", Value: 1},
				{Key: "txHash", Value: 1},
				{Key: "logIndex", Value: 1},
			},
			Options: options.Index().
				SetName("token_event_dead_letter_identity_idx").
				SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "blockNumberInt", Value: 1},
				{Key: "blockHash", Value: 1},
			},
			Options: options.Index().SetName("token_event_dead_letter_rollback_idx"),
		},
	}
}

// InitializeTokenEventDeadLettersCollection is a fail-closed readiness gate:
// ingestion cannot start unless the idempotency and rollback indexes exist.
func InitializeTokenEventDeadLettersCollection() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if _, err := tokenEventDeadLetterCollection().Indexes().CreateMany(
		ctx,
		tokenEventDeadLetterIndexModels(),
	); err != nil {
		return fmt.Errorf("initialize token event dead-letter indexes: %w", err)
	}
	return nil
}
