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
	"reflect"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func ReturnSingleBlock(block uint64) (models.ZondUint64Version, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.ZondUint64Version

	// indexModel := mongo.IndexModel{Keys: bson.D{{"result.number", 1}}}

	// name, err := configs.BlocksCollection.Indexes().CreateOne(context.TODO(), indexModel)
	// if err != nil {
	// 	panic(err)
	// }

	// fmt.Println("Name of Index Created: " + name)

	err := configs.BlocksCollection.FindOne(ctx, bson.M{"result.number": block}).Decode(&result)
	if err != nil {
		fmt.Println(err)
	}

	return result, err
}

func ReturnContracts() ([]models.Transfer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var transactions []models.Transfer
	defer cancel()

	options := options.Find()

	results, err := configs.TransferCollections.Find(ctx, bson.M{"contractAddress": bson.M{"$exists": true}}, options)

	if err != nil {
		fmt.Println(err)
	}

	//reading from the db in an optimal way
	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleTransfer models.Transfer
		if err = results.Decode(&singleTransfer); err != nil {
			fmt.Println(err)
		}

		transactions = append(transactions, singleTransfer)
	}

	return transactions, nil
}

func ReturnLatestTransactions() ([]models.TransactionByAddress, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var transactions []models.TransactionByAddress
	defer cancel()

	options := options.Find().SetProjection(bson.M{"inOut": 1, "txType": 1, "address": 1, "txHash": 1, "timeStamp": 1, "amount": 1})

	options.SetSort(bson.D{{"timeStamp", -1}})

	results, err := configs.TransactionByAddressCollection.Find(ctx, bson.M{}, options)

	if err != nil {
		fmt.Println(err)
	}

	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleTransaction models.TransactionByAddress
		if err = results.Decode(&singleTransaction); err != nil {
			fmt.Println(err)
		}

		transactions = append(transactions, singleTransaction)
	}

	return transactions, nil
}

func ReturnLastSixTransactions() []models.TransactionByAddress {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var transactions []models.TransactionByAddress
	defer cancel()

	options := options.Find().SetProjection(bson.M{"inOut": 1, "txType": 1, "address": 1, "txHash": 1, "timeStamp": 1, "amount": 1})

	options.SetSort(bson.D{{"timeStamp", -1}})
	options.SetLimit(6)

	results, err := configs.TransactionByAddressCollection.Find(ctx, bson.M{}, options)

	if err != nil {
		fmt.Println(err)
	}

	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleTransaction models.TransactionByAddress
		if err = results.Decode(&singleTransaction); err != nil {
			fmt.Println(err)
		}

		transactions = append(transactions, singleTransaction)
	}

	return transactions
}

func ReturnAllInternalTransactionsByAddress(address string) ([]models.TraceResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var transactions []models.TraceResult

	decoded, err := hex.DecodeString(address[2:])
	if err != nil {
		return nil, err
	}

	query := bson.M{
		"$or": []bson.M{
			{"from": decoded},
			{"to": decoded},
		},
	}

	options := options.Find().
		SetProjection(bson.M{
			"type":                      1,
			"callType":                  1,
			"hash":                      1,
			"from":                      1,
			"to":                        1,
			"input":                     1,
			"output":                    1,
			"traceAddress":              1,
			"value":                     1,
			"gas":                       1,
			"gasUsed":                   1,
			"addressFunctionIdentifier": 1,
			"amountFunctionIdentifier":  1,
			"blockTimestamp":            1,
		}).
		SetSort(bson.D{{"blockTimestamp", -1}})

	results, err := configs.InternalTransactionByAddressCollection.Find(ctx, query, options)
	if err != nil {
		return nil, err // Return the error if the query fails
	}
	defer results.Close(ctx)

	for results.Next(ctx) {
		var singleTransaction models.TraceResult
		if err := results.Decode(&singleTransaction); err != nil {
			continue
		}

		from := hex.EncodeToString([]byte(singleTransaction.From))

		fmt.Println(singleTransaction.Output)

		if from == address[2:] {
			singleTransaction.InOut = 0
			singleTransaction.Address = []byte(singleTransaction.To)
		} else {
			singleTransaction.InOut = 1
			singleTransaction.Address = []byte(singleTransaction.From)
		}

		transactions = append(transactions, singleTransaction)
	}

	return transactions, nil
}

func ReturnAllTransactionsByAddress(address string) ([]models.TransactionByAddress, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var transactions []models.TransactionByAddress

	decoded, err := hex.DecodeString(address[2:])
	if err != nil {
		return nil, err
	}

	query := bson.M{
		"$or": []bson.M{
			{"from": decoded},
			{"to": decoded},
		},
	}

	options := options.Find().
		SetProjection(bson.M{
			"timeStamp": 1,
			"amount":    1,
			"inOut":     1,
			"txHash":    1,
			"txType":    1,
			"from":      1,
			"to":        1,
			"paidFees":  1,
		}).
		SetSort(bson.D{{"timeStamp", -1}})

	results, err := configs.TransactionByAddressCollection.Find(ctx, query, options)
	if err != nil {
		return nil, err
	}
	defer results.Close(ctx)

	for results.Next(ctx) {
		var singleTransaction models.TransactionByAddress
		if err := results.Decode(&singleTransaction); err != nil {
			continue
		}

		from := hex.EncodeToString(singleTransaction.From)

		if from == address[2:] && singleTransaction.To != nil {
			singleTransaction.InOut = 0
			singleTransaction.Address = singleTransaction.To
		} else {
			if singleTransaction.To == nil {
				singleTransaction.InOut = 0
				singleTransaction.Address = singleTransaction.From
			} else {
				singleTransaction.InOut = 1
				singleTransaction.Address = singleTransaction.From
			}
		}

		transactions = append(transactions, singleTransaction)
	}

	return transactions, nil
}

func ReturnLatestBlock() ([]models.ZondUint64Version, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var blocks []models.ZondUint64Version
	defer cancel()

	indexModel := mongo.IndexModel{Keys: bson.D{{"result.number", -1}, {"result.timestamp", 1}}}

	name, err := configs.BlocksCollection.Indexes().CreateOne(context.TODO(), indexModel)
	if err != nil {
		panic(err)
	}
	fmt.Println("Name of Index Created: " + name)

	options := options.Find().SetProjection(bson.M{"result.number": 1, "result.timestamp": 1})

	options.SetSort(bson.D{{"result.number", -1}}).SetLimit(1)

	results, err := configs.BlocksCollection.Find(ctx, bson.M{}, options)

	if err != nil {
		fmt.Println(err)
	}

	//reading from the db in an optimal way
	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleBlock models.ZondUint64Version
		if err = results.Decode(&singleBlock); err != nil {
			fmt.Println(err)
		}

		blocks = append(blocks, singleBlock)
	}

	return blocks, nil
}

func ReturnLatestBlocks(page int, limit int) ([]models.Result, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var blocks []models.Result
	defer cancel()

	if limit <= 0 {
		limit = 5 // Default to 5 blocks per page
	}

	options := options.Find().
		SetProjection(bson.M{
			"result.number":       1,
			"result.timestamp":    1,
			"result.hash":         1,
			"result.transactions": 1,
		}).
		SetSort(bson.D{{"result.timestamp", -1}})

	if page == 0 {
		page = 1
	}
	options.SetSkip(int64((page - 1) * limit))
	options.SetLimit(int64(limit))

	results, err := configs.BlocksCollection.Find(ctx, bson.D{}, options)
	if err != nil {
		return nil, err
	}

	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleBlock models.ZondUint64Version
		if err = results.Decode(&singleBlock); err != nil {
			continue
		}
		blocks = append(blocks, singleBlock.Result)
	}

	return blocks, nil
}

func ReturnTransactionsNetwork(page int) ([]models.TransactionByAddress, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var transactions []models.TransactionByAddress
	defer cancel()

	limit := 15

	options := options.Find().SetProjection(bson.M{"inOut": 1, "txType": 1, "address": 1, "txHash": 1, "timeStamp": 1, "amount": 1})

	if limit != 0 {
		if page == 0 {
			page = 1
		}
		options.SetSkip(int64((page - 1) * limit))
		options.SetLimit(int64(limit))
	}

	options.SetSort(bson.D{{"timeStamp", -1}})

	results, err := configs.TransactionByAddressCollection.Find(ctx, bson.D{}, options)

	if err != nil {
		fmt.Println(err)
	}

	//reading from the db in an optimal way
	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleTransaction models.TransactionByAddress
		if err = results.Decode(&singleTransaction); err != nil {
			fmt.Println(err)
		}

		transactions = append(transactions, singleTransaction)
	}

	return transactions, nil
}

func ReturnTransactions(address string, page, limit int) ([]models.TransactionByAddress, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var transactions []models.TransactionByAddress
	defer cancel()

	fmt.Println(address, page, limit)

	options := options.Find().SetProjection(bson.M{"inOut": 1, "txType": 1, "address": 1, "txHash": 1, "timeStamp": 1, "amount": 1})

	if limit != 0 {
		if page == 0 {
			page = 1
		}
		options.SetSkip(int64((page - 1) * limit))
		options.SetLimit(int64(limit))
	}

	options.SetSort(bson.D{{"timeStamp", -1}})

	decoded, err := hex.DecodeString(address[2:])
	if err != nil {
		fmt.Println(err)
	}

	results, err := configs.TransactionByAddressCollection.Find(ctx, bson.M{"address": decoded}, options)

	if err != nil {
		fmt.Println(err)
	}

	//reading from the db in an optimal way
	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleTransaction models.TransactionByAddress
		if err = results.Decode(&singleTransaction); err != nil {
			fmt.Println(err)
		}

		transactions = append(transactions, singleTransaction)
	}

	return transactions, nil
}

func CountBlocksNetwork() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Use CountDocuments instead of loading all blocks
	count, err := configs.BlocksCollection.CountDocuments(ctx, bson.D{})
	if err != nil {
		return 0, err
	}

	return count, nil
}

func CountTransactionsNetwork() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var transactions []models.TransactionByAddress
	defer cancel()

	options := options.Find().SetProjection(bson.M{"inOut": 1, "txType": 1, "address": 1, "txHash": 1, "timeStamp": 1, "amount": 1})

	options.SetSort(bson.D{{"timeStamp", -1}})

	results, err := configs.TransactionByAddressCollection.Find(ctx, bson.D{}, options)

	if err != nil {
		fmt.Println(err)
	}

	//reading from the db in an optimal way
	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleTransaction models.TransactionByAddress
		if err = results.Decode(&singleTransaction); err != nil {
			fmt.Println(err)
		}

		transactions = append(transactions, singleTransaction)
	}

	return len(transactions), nil
}

func CountTransactions(address string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var transactions []models.TransactionByAddress
	defer cancel()

	options := options.Find().SetProjection(bson.M{"inOut": 1, "txType": 1, "address": 1, "txHash": 1, "timeStamp": 1, "amount": 1})

	options.SetSort(bson.D{{"timeStamp", -1}})

	decoded, err := hex.DecodeString(address[2:])
	if err != nil {
		fmt.Println(err)
	}

	results, err := configs.TransactionByAddressCollection.Find(ctx, bson.M{"address": decoded}, options)

	if err != nil {
		fmt.Println(err)
	}

	//reading from the db in an optimal way
	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleTransaction models.TransactionByAddress
		if err = results.Decode(&singleTransaction); err != nil {
			fmt.Println(err)
		}

		transactions = append(transactions, singleTransaction)
	}

	return len(transactions), nil
}

func ReturnSingleTransfer(query string) (models.Transfer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.Transfer

	decoded, err := hex.DecodeString(query[2:])
	if err != nil {
		fmt.Println(err)
	}

	err = configs.TransferCollections.FindOne(ctx, bson.M{"txHash": decoded}).Decode(&result)
	if err != nil {
		fmt.Println(err)
	}

	return result, err
}

func ReturnWalletDistribution(query uint64) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.M{"balance": bson.M{"$gt": (query * 1000000000000)}}

	results, err := configs.AddressesCollection.CountDocuments(ctx, filter)
	if err != nil {
		fmt.Println(err)
	}

	return results, err
}

func ReturnSingleCoinbaseTransfer(query string) (models.Transfer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.Transfer

	decoded, err := hex.DecodeString(query[2:])
	if err != nil {
		fmt.Println(err)
	}

	err = configs.CoinbaseCollection.FindOne(ctx, bson.M{"txHash": decoded}).Decode(&result)
	if err != nil {
		fmt.Println(err)
	}

	return result, err
}

func ReturnHashToBlockNumber(query string) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.ZondUint64Version

	err := configs.BlocksCollection.FindOne(ctx, bson.M{"result.hash": query}).Decode(&result)
	if err != nil {
		fmt.Println(err)
	}

	return result.Result.Number, err
}

func ReturnRichlist() []models.Address {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var addresses []models.Address
	defer cancel()

	options := options.Find().SetProjection(bson.M{"id": 1, "balance": 1}).SetSort(bson.D{{"balance", -1}}).SetLimit(50)

	results, err := configs.AddressesCollection.Find(ctx, bson.D{}, options)

	if err != nil {
		fmt.Println(err)
	}

	//reading from the db in an optimal way
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

func ReturnValidators() (models.AutoGenerated, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.AutoGenerated

	err := configs.ValidatorsCollections.FindOne(ctx, bson.M{}).Decode(&result)
	if err != nil {
		fmt.Println(err)
	}

	fmt.Println(result)

	return result, err
}

func ReturnSingleAddress(query string) (models.Address, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var result models.Address
	defer cancel()

	address, err := hex.DecodeString(query[2:])
	if err != nil {
		fmt.Println(err)
	}

	err = configs.AddressesCollection.FindOne(ctx, bson.M{"id": address}).Decode(&result)
	if err != nil {
		fmt.Println(err)
	}

	return result, err
}

func ReturnRankAddress(address string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var addresses []models.Address
	defer cancel()

	query, err := hex.DecodeString(address[2:])
	if err != nil {
		fmt.Println(err)
	}

	options := options.Find().SetProjection(bson.M{"id": 1, "balance": 1}).SetSort(bson.D{{"balance", -1}})

	results, err := configs.AddressesCollection.Find(ctx, bson.M{}, options)

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

func ReturnDateTime() string {
	currentTime := time.Now()

	return currentTime.Format("2006-01-02 3:4:5 PM")
}

func ReturnContractCode(query string) (models.ContractCode, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.ContractCode

	address, err := hex.DecodeString(query[2:])
	if err != nil {
		fmt.Println(err)
	}

	err = configs.ContractCodeCollection.FindOne(ctx, bson.M{"contractAddress": address}).Decode(&result)
	if err != nil {
		fmt.Println(err)
	}

	return result, err
}

func ReturnBlockSizes() ([]primitive.M, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := bson.D{}
	opts := options.Find().SetSort(bson.D{{"timestamp", 1}})

	cursor, err := configs.BlockSizesCollection.Find(ctx, filter, opts)
	if err != nil {
		panic(err)
	}

	var episodes []bson.M

	if err = cursor.All(ctx, &episodes); err != nil {
		fmt.Println(err)
	}

	return episodes, err
}

func ReturnTotalCirculatingSupply() string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.CirculatingSupply

	err := configs.TotalCirculatingSupplyCollection.FindOne(ctx, bson.M{}).Decode(&result)
	if err != nil {
		fmt.Println(err)
		return ""
	}

	return result.Circulating
}

func ReturnDailyTransactionsVolume() int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.TransactionsVolume

	err := configs.DailyTransactionsVolumeCollection.FindOne(ctx, bson.M{}).Decode(&result)
	if err != nil {
		fmt.Println(err)
		return 0
	}

	return result.Volume
}

func GetMarketCap() float64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.CoinGecko

	err := configs.CoinGeckoCollection.FindOne(ctx, bson.M{}).Decode(&result)
	if err != nil {
		fmt.Println(err)
		return 0
	}

	return result.MarketCapUSD
}

func GetWalletCount() int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.WalletCount

	err := configs.WalletCountCollection.FindOne(ctx, bson.M{}).Decode(&result)
	if err != nil {
		fmt.Println(err)
		return 0
	}

	return result.Count
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

	req, err := http.NewRequest("POST", "http://127.0.0.1:8545", bytes.NewBuffer([]byte(b)))
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
