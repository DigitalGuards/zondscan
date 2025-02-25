package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"os"
	"reflect"
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

	// Store address as-is without stripping prefixes
	addressHex := query

	// Try to find existing address
	filter := bson.D{{Key: "id", Value: addressHex}}
	err := configs.AddressesCollections.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			// Address not found, create new one
			balance, errMsg := GetBalance(query)
			if errMsg != "" {
				return result, fmt.Errorf("error getting balance: %s", errMsg)
			}

			result = models.Address{
				ObjectId: primitive.NewObjectID(),
				ID:       addressHex,
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
		fmt.Println(err)
	}

	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleAddress models.Address
		if err = results.Decode(&singleAddress); err != nil {
			fmt.Println(err)
		}
		addresses = append(addresses, singleAddress)
	}

	return addresses
}

func ReturnRankAddress(address string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var addresses []models.Address
	defer cancel()

	// Store address as-is without stripping prefixes
	addressHex := address

	query, err := hex.DecodeString(addressHex)
	if err != nil {
		fmt.Println(err)
	}

	projection := bson.D{
		{Key: "id", Value: 1},
		{Key: "balance", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(bson.D{{Key: "balance", Value: -1}})

	results, err := configs.AddressesCollections.Find(ctx, bson.D{}, opts)
	if err != nil {
		fmt.Println(err)
	}

	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleAddress models.Address
		if err = results.Decode(&singleAddress); err != nil {
			fmt.Println(err)
		}
		addresses = append(addresses, singleAddress)
	}

	var i int64
	i = 0
	for i = 0; i < GetWalletCount(); i++ {
		if reflect.DeepEqual(addresses[i].ID, query) {
			fmt.Println(query)
			break
		}
	}

	return i + 1, nil
}

func GetBalance(address string) (float64, string) {
	var result models.Balance

	group := models.JsonRPC{
		Jsonrpc: "2.0",
		Method:  "zond_getBalance",
		Params:  []interface{}{address, "latest"},
		ID:      1,
	}
	b, err := json.Marshal(group)
	if err != nil {
		fmt.Println("error:", err)
	}

	nodeURL := os.Getenv("NODE_URL")
	if nodeURL == "" {
		nodeURL = "http://127.0.0.1:8545" // fallback to default if not set
	}

	req, err := http.NewRequest("POST", nodeURL, bytes.NewBuffer([]byte(b)))
	if err != nil {
		fmt.Println("Error creating request:", err)
		return 0, "Error connecting to node"
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		fmt.Println("Error making request:", err)
		return 0, "Error connecting to node"
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("Error reading response:", err)
		return 0, "Error reading node response"
	}
	fmt.Println(string(body))

	err = json.Unmarshal([]byte(string(body)), &result)
	if err != nil {
		fmt.Println("Error unmarshaling response:", err)
		return 0, "Error parsing node response"
	}

	if result.Error.Message != "" {
		return 0, result.Error.Message
	} else {
		fmt.Println(result.Result[2:])

		balance := new(big.Int)
		balance, success := balance.SetString(result.Result[2:], 16)
		if !success {
			fmt.Println("Error converting hexadecimal string to big.Int")
		}

		balanceFloat := new(big.Float).SetInt(balance)
		divisor := new(big.Float).SetFloat64(1e18)
		result := new(big.Float).Quo(balanceFloat, divisor)
		float64Value, _ := result.Float64()
		return float64Value, ""
	}
}

func ReturnWalletDistribution(query uint64) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.D{{Key: "balance", Value: bson.D{
		{Key: "$gt", Value: (query * 1000000000000)},
	}}}

	results, err := configs.AddressesCollections.CountDocuments(ctx, filter)
	if err != nil {
		fmt.Println(err)
	}

	return results, err
}

func GetWalletCount() int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.WalletCount
	err := configs.WalletCountCollections.FindOne(ctx, bson.M{"_id": "current_count"}).Decode(&result)
	if err != nil {
		fmt.Printf("Error getting wallet count: %v\n", err)
		return 0
	}

	return result.Count
}
