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
	"go.mongodb.org/mongo-driver/mongo/options"
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

func ReturnRichlist() []models.Address {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var addresses []models.Address
	defer cancel()

	projection := bson.D{
		{Key: "id", Value: 1},
		{Key: "balance", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(bson.D{{Key: "balance", Value: -1}}).
		SetLimit(50)

	results, err := configs.AddressesCollections.Find(ctx, bson.D{}, opts)
	if err != nil {
		log.Printf("error querying richlist: %v", err)
	}

	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleAddress models.Address
		if err = results.Decode(&singleAddress); err != nil {
			log.Printf("error decoding richlist address: %v", err)
		}
		addresses = append(addresses, singleAddress)
	}

	return addresses
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
			// Address not found — return 0 to signal unknown rank
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
