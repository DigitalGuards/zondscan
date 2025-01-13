package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

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

func ReturnTransactionsNetwork(page int) ([]models.TransactionByAddress, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var transactions []models.TransactionByAddress
	defer cancel()

	limit := 5

	projection := primitive.D{
		{Key: "inOut", Value: 1},
		{Key: "txType", Value: 1},
		{Key: "address", Value: 1},
		{Key: "txHash", Value: 1},
		{Key: "timeStamp", Value: 1},
		{Key: "amount", Value: 1},
		{Key: "blockNumber", Value: 1},
		{Key: "from", Value: 1},
		{Key: "to", Value: 1},
		{Key: "paidFees", Value: 1},
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
		return nil, err
	}

	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleTransaction models.TransactionByAddress
		if err = results.Decode(&singleTransaction); err != nil {
			fmt.Println(err)
			continue
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

	// First try to find the transaction in the blocks collection
	var block models.ZondDatabaseBlock
	blockFilter := bson.M{
		"result.transactions": bson.M{
			"$elemMatch": bson.M{
				"hash": query,
			},
		},
	}

	err := configs.BlocksCollection.FindOne(ctx, blockFilter).Decode(&block)
	if err == nil {
		// Found in blocks collection, convert to Transfer model
		for _, tx := range block.Result.Transactions {
			if tx.Hash == query {
				// Convert from address
				from, err := hex.DecodeString(tx.From[2:])
				if err != nil {
					fmt.Println("Error decoding from address:", err)
				}

				// Convert to address if it exists
				var to []byte
				if tx.To != "" {
					to, err = hex.DecodeString(tx.To[2:])
					if err != nil {
						fmt.Println("Error decoding to address:", err)
					}
				}

				// Convert transaction hash
				txHash, err := hex.DecodeString(tx.Hash[2:])
				if err != nil {
					fmt.Println("Error decoding transaction hash:", err)
				}

				// Store original hex value
				valueStr := tx.Value
				if valueStr == "" || valueStr == "0x0" {
					valueStr = "0x0"
				}

				// Store original gas values
				gasUsedStr := tx.Gas
				if gasUsedStr == "" || gasUsedStr == "0x0" {
					gasUsedStr = "0x0"
				}

				gasPriceStr := tx.GasPrice
				if gasPriceStr == "" || gasPriceStr == "0x0" {
					gasPriceStr = "0x0"
				}

				// Convert value from hex string to uint64 (for backward compatibility)
				value := uint64(0)
				if valueStr != "0x0" {
					valueHex := valueStr
					if valueHex[:2] == "0x" {
						valueHex = valueHex[2:]
					}
					value, err = strconv.ParseUint(valueHex, 16, 64)
					if err != nil {
						fmt.Println("Error parsing value:", err)
					}
				}

				// Convert gas values from hex string to uint64
				gasUsed := uint64(0)
				if gasUsedStr != "0x0" {
					gasHex := gasUsedStr
					if gasHex[:2] == "0x" {
						gasHex = gasHex[2:]
					}
					gasUsed, err = strconv.ParseUint(gasHex, 16, 64)
					if err != nil {
						fmt.Println("Error parsing gas used:", err)
					}
				}

				gasPrice := uint64(0)
				if gasPriceStr != "0x0" {
					gasPriceHex := gasPriceStr
					if gasPriceHex[:2] == "0x" {
						gasPriceHex = gasPriceHex[2:]
					}
					gasPrice, err = strconv.ParseUint(gasPriceHex, 16, 64)
					if err != nil {
						fmt.Println("Error parsing gas price:", err)
					}
				}

				// Convert nonce from hex string to uint64
				nonce := uint64(0)
				if tx.Nonce != "" {
					nonceStr := tx.Nonce
					if nonceStr[:2] == "0x" {
						nonceStr = nonceStr[2:]
					}
					nonce, err = strconv.ParseUint(nonceStr, 16, 64)
					if err != nil {
						fmt.Println("Error parsing nonce:", err)
					}
				}

				// Convert signature and public key if available
				var signature, pk []byte
				if tx.Signature != "" {
					signature, err = hex.DecodeString(tx.Signature[2:])
					if err != nil {
						fmt.Println("Error decoding signature:", err)
					}
				}
				if tx.PublicKey != "" {
					pk, err = hex.DecodeString(tx.PublicKey[2:])
					if err != nil {
						fmt.Println("Error decoding public key:", err)
					}
				}

				result = models.Transfer{
					ID:             primitive.NewObjectID(),
					BlockNumber:    block.Result.Number,
					BlockTimestamp: block.Result.Timestamp,
					From:           from,
					To:             to,
					TxHash:         txHash,
					Value:          value,
					GasUsed:        gasUsed,
					GasPrice:       gasPrice,
					Nonce:          nonce,
					Signature:      signature,
					Pk:             pk,
					Size:           block.Result.Size,
					FromStr:        tx.From,
					ToStr:          tx.To,
				}
				return result, nil
			}
		}
	}

	// If not found in blocks, try the transfers collection (fallback)
	decoded, err := hex.DecodeString(query[2:])
	if err != nil {
		fmt.Println(err)
	}

	filter := primitive.D{{Key: "txHash", Value: decoded}}
	err = configs.TransferCollections.FindOne(ctx, filter).Decode(&result)
	if err != nil {
		fmt.Println(err)
	}

	// Add hex string representations for the fallback case too
	if result.From != nil {
		result.FromStr = "0x" + hex.EncodeToString(result.From)
	}
	if result.To != nil {
		result.ToStr = "0x" + hex.EncodeToString(result.To)
	}

	return result, err
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

func GetTransactionByHash(hash string) (*models.Transaction, error) {
    collection := configs.GetCollection(configs.DB, "transfer")
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()

    // Remove "0x" prefix if present and decode hex to bytes
    if strings.HasPrefix(hash, "0x") {
        hash = hash[2:]
    }
    hashBytes, err := hex.DecodeString(hash)
    if err != nil {
        return nil, fmt.Errorf("invalid hash format: %v", err)
    }

    var transfer models.Transfer
    err = collection.FindOne(ctx, bson.M{"txhash": hashBytes}).Decode(&transfer)
    if err != nil {
        if err == mongo.ErrNoDocuments {
            return nil, nil // Return nil if not found
        }
        return nil, err
    }

    // Convert back to hex string with "0x" prefix
    txHashHex := "0x" + hex.EncodeToString(transfer.TxHash)
    blockNumberStr := strconv.FormatUint(transfer.BlockNumber, 10)

    // Convert Transfer to Transaction
    return &models.Transaction{
        Hash:        txHashHex,
        BlockNumber: blockNumberStr,
    }, nil
}
