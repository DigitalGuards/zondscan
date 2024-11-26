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

func ReturnSingleBlock(block uint64) (models.ZondUint64Version, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.ZondUint64Version

	filter := primitive.D{{Key: "result.number", Value: block}}
	err := configs.BlocksCollection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		fmt.Println(err)
	}

	return result, err
}

func ReturnContracts() ([]models.Transfer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var transactions []models.Transfer
	defer cancel()

	filter := primitive.D{{Key: "contractAddress", Value: primitive.D{{Key: "$exists", Value: true}}}}
	results, err := configs.TransferCollections.Find(ctx, filter, options.Find())

	if err != nil {
		fmt.Println(err)
	}

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

	projection := primitive.D{
		{Key: "inOut", Value: 1},
		{Key: "txType", Value: 1},
		{Key: "address", Value: 1},
		{Key: "txHash", Value: 1},
		{Key: "timeStamp", Value: 1},
		{Key: "amount", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "timeStamp", Value: -1}})

	results, err := configs.TransactionByAddressCollection.Find(ctx, primitive.D{}, opts)
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

	projection := primitive.D{
		{Key: "inOut", Value: 1},
		{Key: "txType", Value: 1},
		{Key: "address", Value: 1},
		{Key: "txHash", Value: 1},
		{Key: "timeStamp", Value: 1},
		{Key: "amount", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "timeStamp", Value: -1}}).
		SetLimit(6)

	results, err := configs.TransactionByAddressCollection.Find(ctx, primitive.D{}, opts)
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

	filter := primitive.D{{Key: "$or", Value: []primitive.D{
		{{Key: "from", Value: decoded}},
		{{Key: "to", Value: decoded}},
	}}}

	projection := primitive.D{
		{Key: "type", Value: 1},
		{Key: "callType", Value: 1},
		{Key: "hash", Value: 1},
		{Key: "from", Value: 1},
		{Key: "to", Value: 1},
		{Key: "input", Value: 1},
		{Key: "output", Value: 1},
		{Key: "traceAddress", Value: 1},
		{Key: "value", Value: 1},
		{Key: "gas", Value: 1},
		{Key: "gasUsed", Value: 1},
		{Key: "addressFunctionIdentifier", Value: 1},
		{Key: "amountFunctionIdentifier", Value: 1},
		{Key: "blockTimestamp", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "blockTimestamp", Value: -1}})

	results, err := configs.InternalTransactionByAddressCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
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

	filter := primitive.D{{Key: "$or", Value: []primitive.D{
		{{Key: "from", Value: decoded}},
		{{Key: "to", Value: decoded}},
	}}}

	projection := primitive.D{
		{Key: "timeStamp", Value: 1},
		{Key: "amount", Value: 1},
		{Key: "inOut", Value: 1},
		{Key: "txHash", Value: 1},
		{Key: "txType", Value: 1},
		{Key: "from", Value: 1},
		{Key: "to", Value: 1},
		{Key: "paidFees", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "timeStamp", Value: -1}})

	results, err := configs.TransactionByAddressCollection.Find(ctx, filter, opts)
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

	projection := primitive.D{
		{Key: "result.number", Value: 1},
		{Key: "result.timestamp", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "result.number", Value: -1}}).
		SetLimit(1)

	results, err := configs.BlocksCollection.Find(ctx, primitive.D{}, opts)
	if err != nil {
		fmt.Println(err)
	}

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

	projection := primitive.D{
		{Key: "result.number", Value: 1},
		{Key: "result.timestamp", Value: 1},
		{Key: "result.hash", Value: 1},
		{Key: "result.transactions", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "result.timestamp", Value: -1}})

	if page == 0 {
		page = 1
	}
	opts.SetSkip(int64((page - 1) * limit))
	opts.SetLimit(int64(limit))

	results, err := configs.BlocksCollection.Find(ctx, primitive.D{}, opts)
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

	projection := primitive.D{
		{Key: "inOut", Value: 1},
		{Key: "txType", Value: 1},
		{Key: "address", Value: 1},
		{Key: "txHash", Value: 1},
		{Key: "timeStamp", Value: 1},
		{Key: "amount", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "timeStamp", Value: -1}})

	if limit != 0 {
		if page == 0 {
			page = 1
		}
		opts.SetSkip(int64((page - 1) * limit))
		opts.SetLimit(int64(limit))
	}

	results, err := configs.TransactionByAddressCollection.Find(ctx, primitive.D{}, opts)
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

func ReturnTransactions(address string, page, limit int) ([]models.TransactionByAddress, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var transactions []models.TransactionByAddress
	defer cancel()

	fmt.Println(address, page, limit)

	projection := primitive.D{
		{Key: "inOut", Value: 1},
		{Key: "txType", Value: 1},
		{Key: "address", Value: 1},
		{Key: "txHash", Value: 1},
		{Key: "timeStamp", Value: 1},
		{Key: "amount", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "timeStamp", Value: -1}})

	if limit != 0 {
		if page == 0 {
			page = 1
		}
		opts.SetSkip(int64((page - 1) * limit))
		opts.SetLimit(int64(limit))
	}

	decoded, err := hex.DecodeString(address[2:])
	if err != nil {
		fmt.Println(err)
	}

	filter := primitive.D{{Key: "address", Value: decoded}}
	results, err := configs.TransactionByAddressCollection.Find(ctx, filter, opts)
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

func CountBlocksNetwork() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	count, err := configs.BlocksCollection.CountDocuments(ctx, primitive.D{})
	if err != nil {
		return 0, err
	}

	return count, nil
}

func CountTransactionsNetwork() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var transactions []models.TransactionByAddress
	defer cancel()

	projection := primitive.D{
		{Key: "inOut", Value: 1},
		{Key: "txType", Value: 1},
		{Key: "address", Value: 1},
		{Key: "txHash", Value: 1},
		{Key: "timeStamp", Value: 1},
		{Key: "amount", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "timeStamp", Value: -1}})

	results, err := configs.TransactionByAddressCollection.Find(ctx, primitive.D{}, opts)
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

	return len(transactions), nil
}

func CountTransactions(address string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var transactions []models.TransactionByAddress
	defer cancel()

	projection := primitive.D{
		{Key: "inOut", Value: 1},
		{Key: "txType", Value: 1},
		{Key: "address", Value: 1},
		{Key: "txHash", Value: 1},
		{Key: "timeStamp", Value: 1},
		{Key: "amount", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "timeStamp", Value: -1}})

	decoded, err := hex.DecodeString(address[2:])
	if err != nil {
		fmt.Println(err)
	}

	filter := primitive.D{{Key: "address", Value: decoded}}
	results, err := configs.TransactionByAddressCollection.Find(ctx, filter, opts)
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

	filter := primitive.D{{Key: "txHash", Value: decoded}}
	err = configs.TransferCollections.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		fmt.Println(err)
	}

	return result, err
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

func ReturnSingleCoinbaseTransfer(query string) (models.Transfer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.Transfer

	decoded, err := hex.DecodeString(query[2:])
	if err != nil {
		fmt.Println(err)
	}

	filter := primitive.D{{Key: "txHash", Value: decoded}}
	err = configs.CoinbaseCollection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		fmt.Println(err)
	}

	return result, err
}

func ReturnHashToBlockNumber(query string) (uint64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.ZondUint64Version

	filter := primitive.D{{Key: "result.hash", Value: query}}
	err := configs.BlocksCollection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		fmt.Println(err)
	}

	return result.Result.Number, err
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

func ReturnValidators() (models.AutoGenerated, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.AutoGenerated

	err := configs.ValidatorsCollections.FindOne(ctx, primitive.D{}).Decode(&result)
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
		fmt.Printf("Error decoding contract address %s: %v\n", query, err)
		return result, err
	}

	filter := primitive.D{{Key: "contractAddress", Value: address}}
	err = configs.ContractCodeCollection.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			fmt.Printf("No contract code found for %s\n", query)
		} else {
			fmt.Printf("Error querying contract code %s: %v\n", query, err)
		}
		return result, err
	}

	return result, nil
}
func ReturnBlockSizes() ([]primitive.M, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	opts := options.Find().SetSort(primitive.D{{Key: "timestamp", Value: 1}})

	cursor, err := configs.BlockSizesCollection.Find(ctx, primitive.D{}, opts)
	if err != nil {
		panic(err)
	}

	var episodes []primitive.M
	if err = cursor.All(ctx, &episodes); err != nil {
		fmt.Println(err)
	}

	return episodes, err
}

func ReturnTotalCirculatingSupply() string {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.CirculatingSupply

	err := configs.TotalCirculatingSupplyCollection.FindOne(ctx, primitive.D{}).Decode(&result)
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

	err := configs.DailyTransactionsVolumeCollection.FindOne(ctx, primitive.D{}).Decode(&result)
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

	err := configs.CoinGeckoCollection.FindOne(ctx, primitive.D{}).Decode(&result)
	if err != nil {
		fmt.Println(err)
		return 0
	}

	return result.MarketCapUSD
}

func GetCurrentPrice() float64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.CoinGecko

	err := configs.CoinGeckoCollection.FindOne(ctx, primitive.D{}).Decode(&result)
	if err != nil {
		fmt.Println(err)
		return 0
	}

	return result.PriceUSD
}

func GetWalletCount() int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.WalletCount

	err := configs.WalletCountCollection.FindOne(ctx, primitive.D{}).Decode(&result)
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
