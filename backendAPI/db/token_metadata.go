// Package db / token_metadata.go (backend)
//
// Read helpers for the per-tokenID metadata persisted by the Phase 3b
// metadata fetcher service. The syncer is the only writer; this backend
// only reads.
package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

// GetTokenMetadata returns the metadata document for a specific
// (contract, tokenID). Returns nil, nil if no stub exists.
func GetTokenMetadata(contractAddress, tokenID string) (*models.TokenMetadata, error) {
	contractAddress = normalizeAddress(contractAddress)
	tokenID = strings.TrimSpace(tokenID)
	if tokenID == "" {
		return nil, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	collection := configs.TokenMetadataCollection
	var out models.TokenMetadata
	err := collection.FindOne(ctx, bson.M{
		"contractAddress": contractAddress,
		"tokenID":         tokenID,
	}).Decode(&out)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &out, nil
}

// GetTokenMetadataMap returns a (contract, tokenID) -> metadata map for the
// list of tokenIDs in one collection. Used to enrich /token/:addr/tokens
// rows with per-token name + image without a second roundtrip per row.
func GetTokenMetadataMap(contractAddress string, tokenIDs []string) (map[string]*models.TokenMetadata, error) {
	contractAddress = normalizeAddress(contractAddress)
	if len(tokenIDs) == 0 {
		return map[string]*models.TokenMetadata{}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := configs.TokenMetadataCollection
	cur, err := collection.Find(ctx, bson.M{
		"contractAddress": contractAddress,
		"tokenID":         bson.M{"$in": tokenIDs},
	})
	if err != nil {
		return nil, err
	}
	defer cur.Close(ctx)

	out := make(map[string]*models.TokenMetadata, len(tokenIDs))
	for cur.Next(ctx) {
		var m models.TokenMetadata
		if err := cur.Decode(&m); err != nil {
			continue
		}
		copyOf := m
		out[m.TokenID] = &copyOf
	}
	return out, nil
}
