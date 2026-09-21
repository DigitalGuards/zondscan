package db

import (
	"context"
	"fmt"
	"time"

	"backendAPI/models"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// ContractExplainChallengeStore persists replay-protection state in MongoDB so
// every backend instance observes the same single-use challenge lifecycle.
type ContractExplainChallengeStore struct {
	collection *mongo.Collection
}

func NewContractExplainChallengeStore(collection *mongo.Collection) *ContractExplainChallengeStore {
	return &ContractExplainChallengeStore{collection: collection}
}

func (s *ContractExplainChallengeStore) Create(
	ctx context.Context,
	challenge models.ContractExplainChallenge,
) error {
	if s == nil || s.collection == nil {
		return fmt.Errorf("contract explanation challenge collection is unavailable")
	}
	_, err := s.collection.InsertOne(ctx, challenge)
	return err
}

func (s *ContractExplainChallengeStore) Load(
	ctx context.Context,
	id string,
) (models.ContractExplainChallenge, bool, error) {
	var challenge models.ContractExplainChallenge
	if s == nil || s.collection == nil {
		return challenge, false, fmt.Errorf("contract explanation challenge collection is unavailable")
	}
	err := s.collection.FindOne(ctx, bson.M{"_id": id}).Decode(&challenge)
	if err == mongo.ErrNoDocuments {
		return challenge, false, nil
	}
	if err != nil {
		return challenge, false, err
	}
	return challenge, true, nil
}

func (s *ContractExplainChallengeStore) Consume(
	ctx context.Context,
	challenge models.ContractExplainChallenge,
	now time.Time,
) (bool, error) {
	if s == nil || s.collection == nil {
		return false, fmt.Errorf("contract explanation challenge collection is unavailable")
	}
	result, err := s.collection.DeleteOne(ctx, contractExplainChallengeConsumeFilter(challenge, now))
	if err != nil {
		return false, err
	}
	return result.DeletedCount == 1, nil
}

func contractExplainChallengeConsumeFilter(
	challenge models.ContractExplainChallenge,
	now time.Time,
) bson.D {
	return bson.D{
		{Key: "_id", Value: challenge.ID},
		{Key: "version", Value: challenge.Version},
		{Key: "action", Value: challenge.Action},
		{Key: "origin", Value: challenge.Origin},
		{Key: "chainId", Value: challenge.ChainID},
		{Key: "method", Value: challenge.Method},
		{Key: "route", Value: challenge.Route},
		{Key: "contractAddress", Value: challenge.ContractAddress},
		{Key: "signerAddress", Value: challenge.SignerAddress},
		{Key: "message", Value: challenge.Message},
		{Key: "issuedAt", Value: challenge.IssuedAt},
		{Key: "expiresAt", Value: bson.M{"$eq": challenge.ExpiresAt, "$gt": now}},
	}
}
