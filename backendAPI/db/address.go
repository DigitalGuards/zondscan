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

	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ReturnSingleAddress(query string) (models.Address, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var result models.Address
	defer cancel()

	address, err := hex.DecodeString(query[2:])
	if err != nil {
		fmt.Printf("Error decoding address %s: %v\n", query, err)
		return result, err
	}

	filter := primitive.D{{Key: "id", Value: address}}
	err = configs.AddressesCollection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			fmt.Printf("No address found for %s\n", query)
		} else {
			fmt.Printf("Error querying address %s: %v\n", query, err)
		}
		return result, err
	}

	return result, nil
}

func ReturnRichlist() []models.Address {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var addresses []models.Address
	defer cancel()

	projection := primitive.D{
		{Key: "id", Value: 1},
		{Key: "balance", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "balance", Value: -1}}).
		SetLimit(50)

	results, err := configs.AddressesCollection.Find(ctx, primitive.D{}, opts)
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

	query, err := hex.DecodeString(address[2:])
	if err != nil {
		fmt.Println(err)
	}

	projection := primitive.D{
		{Key: "id", Value: 1},
		{Key: "balance", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "balance", Value: -1}})

	results, err := configs.AddressesCollection.Find(ctx, primitive.D{}, opts)
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
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	fmt.Println(string(body))

	err = json.Unmarshal([]byte(string(body)), &result)

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

	filter := primitive.D{{Key: "balance", Value: primitive.D{
		{Key: "$gt", Value: (query * 1000000000000)},
	}}}

	results, err := configs.AddressesCollection.CountDocuments(ctx, filter)
	if err != nil {
		fmt.Println(err)
	}

	return results, err
}

func GetWalletCount() int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.WalletCount

	err := configs.WalletCountCollection.FindOne(ctx, primitive.D{}).Decode(&result)
	if err != nil {
		fmt.Println(err)
	}

	return result.Count
}
