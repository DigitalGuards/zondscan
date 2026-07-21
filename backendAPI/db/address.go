package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func ReturnSingleAddress(query string) (models.Address, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var result models.Address
	defer cancel()

	// Normalize address to canonical Q-prefix form
	addressHex := normalizeAddress(query)

	// Try to find existing address
	filter := bson.D{{Key: "id", Value: addressHex}}
	err := configs.AddressesCollections.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// Address not found, create new one
			balance, errMsg := GetBalance(query) // Use original query for RPC call
			if errMsg != "" {
				return result, fmt.Errorf("error getting balance: %s", errMsg)
			}

			result = models.Address{
				ObjectId: primitive.NewObjectID(),
				ID:       addressHex, // Store normalized address
				Balance:  balance,
				Nonce:    0, // Default nonce for new address
			}

			_, err = configs.AddressesCollections.InsertOne(ctx, result)
			if err != nil {
				return result, fmt.Errorf("error creating new address: %v", err)
			}
		} else {
			return result, fmt.Errorf("error querying address: %v", err)
		}
	}

	return result, nil
}

func ReturnRichlist() ([]models.RichlistEntry, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Contract detection joins contractCode by address instead of trusting
	// addresses.isContract (known-poisoned data). Both collections store
	// canonical Q+lowercase addresses and contractCode has a unique index
	// on address, so the equality lookup is safe. A row only counts as a
	// contract when at least one joined doc carries real bytecode
	// (non-empty and not "0x", which guards failed deploys).
	pipeline := []bson.M{
		{"$sort": bson.M{"balance": -1}},
		{"$limit": 50},
		{"$lookup": bson.M{
			"from":         "contractCode",
			"localField":   "id",
			"foreignField": "address",
			"as":           "contract",
		}},
		{"$project": bson.M{
			"_id":     0,
			"id":      1,
			"balance": 1,
			"isContract": bson.M{"$gt": []interface{}{
				bson.M{"$size": bson.M{"$filter": bson.M{
					"input": "$contract",
					"as":    "c",
					"cond": bson.M{"$not": bson.M{"$in": []interface{}{
						bson.M{"$ifNull": []interface{}{"$$c.contractCode", ""}},
						[]interface{}{"", "0x"},
					}}},
				}}},
				0,
			}},
		}},
	}

	cursor, err := configs.AddressesCollections.Aggregate(ctx, pipeline)
	if err != nil {
		// Early return: proceeding onto the nil cursor used to panic on
		// every request whenever Mongo errored here.
		log.Printf("error querying richlist: %v", err)
		return nil, err
	}
	defer cursor.Close(ctx)

	var entries []models.RichlistEntry
	if err := cursor.All(ctx, &entries); err != nil {
		log.Printf("error decoding richlist: %v", err)
		return nil, err
	}

	// Percent-of-supply uses the same balances the ranking sorts on so the
	// column stays consistent with the ordering. On error the column
	// degrades to 0 rather than failing the whole richlist.
	total, err := totalAddressBalance(ctx)
	if err != nil {
		log.Printf("error summing total balance for richlist: %v", err)
	}

	for i := range entries {
		if total > 0 {
			entries[i].SupplyPercent = entries[i].Balance / total * 100
		}
		// One index-backed FindOne per row; the /richlist route caches the
		// result for 30 s, so this runs at most once per cache window.
		entries[i].FirstSeen = FirstSeen(entries[i].ID)
	}

	return entries, nil
}

// totalAddressBalance sums every address balance, the denominator for the
// richlist's percent-of-supply column.
func totalAddressBalance(ctx context.Context) (float64, error) {
	pipeline := []bson.M{
		{"$group": bson.M{"_id": nil, "total": bson.M{"$sum": "$balance"}}},
	}

	cursor, err := configs.AddressesCollections.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, err
	}
	defer cursor.Close(ctx)

	var row struct {
		Total float64 `bson:"total"`
	}
	if cursor.Next(ctx) {
		if err := cursor.Decode(&row); err != nil {
			return 0, err
		}
	}
	if err := cursor.Err(); err != nil {
		return 0, err
	}

	return row.Total, nil
}

func ReturnRankAddress(address string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Normalize address to canonical Q-prefix form (matches storage format)
	addressHex := normalizeAddress(address)

	// Look up the target address to get its balance
	var target models.Address
	err := configs.AddressesCollections.FindOne(ctx, bson.D{{Key: "id", Value: addressHex}}).Decode(&target)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// Address not found, return 0 to signal unknown rank
			return 0, nil
		}
		return 0, fmt.Errorf("error looking up address for rank: %v", err)
	}

	// Count how many addresses have a strictly higher balance; rank = that count + 1
	count, err := configs.AddressesCollections.CountDocuments(ctx, bson.M{"balance": bson.M{"$gt": target.Balance}})
	if err != nil {
		return 0, fmt.Errorf("error counting addresses for rank: %v", err)
	}

	return count + 1, nil
}

func GetBalance(address string) (float64, string) {
	// Ensure address has Q prefix for RPC calls
	rpcAddress := address
	if strings.HasPrefix(rpcAddress, "0x") {
		rpcAddress = "Q" + rpcAddress[2:]
	} else if !strings.HasPrefix(rpcAddress, "Q") {
		rpcAddress = "Q" + rpcAddress
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	raw, rpcErr, err := NodeRPC(ctx, "qrl_getBalance", []interface{}{rpcAddress, "latest"})
	if err != nil {
		log.Printf("GetBalance(%s): %v", address, err)
		return 0, "Error connecting to node"
	}
	if rpcErr != nil {
		return 0, rpcErr.Message
	}

	var hexResult string
	if err := json.Unmarshal(raw, &hexResult); err != nil {
		log.Printf("GetBalance(%s): unmarshal result: %v", address, err)
		return 0, "Error parsing node response"
	}
	if len(hexResult) < 3 || !strings.HasPrefix(hexResult, "0x") {
		log.Printf("GetBalance(%s): unexpected balance shape %q", address, hexResult)
		return 0, "Error parsing node response"
	}

	balance := new(big.Int)
	balance, success := balance.SetString(hexResult[2:], 16)
	if !success {
		log.Printf("error converting hex balance to big.Int for address %s", address)
	}

	balanceFloat := new(big.Float).SetInt(balance)
	divisor := new(big.Float).SetFloat64(1e18)
	res := new(big.Float).Quo(balanceFloat, divisor)
	float64Value, _ := res.Float64()
	return float64Value, ""
}

func ReturnWalletDistribution(query uint64) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.D{{Key: "balance", Value: bson.D{
		{Key: "$gt", Value: (query * 1000000000000)},
	}}}

	results, err := configs.AddressesCollections.CountDocuments(ctx, filter)
	if err != nil {
		log.Printf("error counting wallet distribution: %v", err)
	}

	return results, err
}

func GetWalletCount() int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.WalletCount
	err := configs.WalletCountCollections.FindOne(ctx, bson.M{"_id": "current_count"}).Decode(&result)
	if err != nil {
		log.Printf("error getting wallet count: %v", err)
		return 0
	}

	return result.Count
}
