package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	DEFAULT_PAGE_SIZE  = 10
	PENDING_COLLECTION = "pending_transactions"
)

func GetPendingTransactions(page, limit int) (*models.PaginatedPendingTransactions, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = DEFAULT_PAGE_SIZE
	}

	collection := configs.GetCollection(configs.DB, PENDING_COLLECTION)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Get total count for all non-mined transactions
	filter := bson.M{"status": bson.M{"$ne": "mined"}}
	total, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	totalPages := (int(total) + limit - 1) / limit
	skip := (page - 1) * limit

	// Get paginated transactions
	opts := options.Find().
		SetSort(bson.M{"createdAt": -1}).
		SetSkip(int64(skip)).
		SetLimit(int64(limit))

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var transactions []models.PendingTransaction
	if err = cursor.All(ctx, &transactions); err != nil {
		return nil, err
	}

	// Ensure timestamps are in UTC
	for i := range transactions {
		transactions[i].LastSeen = transactions[i].LastSeen.UTC()
		transactions[i].CreatedAt = transactions[i].CreatedAt.UTC()
	}

	return &models.PaginatedPendingTransactions{
		Transactions: transactions,
		Total:        int(total),
		Page:         page,
		Limit:        limit,
		TotalPages:   totalPages,
	}, nil
}

func GetPendingTransactionByHash(hash string) (*models.PendingTransaction, error) {
	collection := configs.GetCollection(configs.DB, PENDING_COLLECTION)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var transaction models.PendingTransaction
	err := collection.FindOne(ctx, bson.M{"_id": hash}).Decode(&transaction)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}
		return nil, err
	}

	// Ensure timestamps are in UTC
	transaction.LastSeen = transaction.LastSeen.UTC()
	transaction.CreatedAt = transaction.CreatedAt.UTC()

	return &transaction, nil
}

// DeleteMinedTransaction removes a transaction from the pending_transactions collection once it's mined
func DeleteMinedTransaction(hash string) error {
    collection := configs.GetCollection(configs.DB, PENDING_COLLECTION)
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    _, err := collection.DeleteOne(ctx, bson.M{"_id": hash})
    return err
}
