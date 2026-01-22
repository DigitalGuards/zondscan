package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"regexp"
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
		{Key: "from", Value: 1},
		{Key: "to", Value: 1},
		{Key: "txHash", Value: 1},
		{Key: "timeStamp", Value: 1},
		{Key: "amount", Value: 1},
		{Key: "paidFees", Value: 1},
		{Key: "blockNumber", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "timeStamp", Value: -1}})

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

func ReturnAllInternalTransactionsByAddress(address string) ([]models.TraceResult, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var transactions []models.TraceResult

	// Format the address for query
	formattedAddress := address
	if !strings.HasPrefix(formattedAddress, "Z") {
		formattedAddress = "Z" + formattedAddress
	}

	// For internal transactions, we need to strip the Z prefix
	addressWithoutPrefix := strings.TrimPrefix(formattedAddress, "Z")

	// Try different case variants of the address
	addressVariants := []string{
		addressWithoutPrefix,
		strings.ToLower(addressWithoutPrefix),
		strings.ToUpper(addressWithoutPrefix),
	}

	for _, addrVariant := range addressVariants {
		decoded, err := hex.DecodeString(addrVariant)
		if err != nil {
			continue // Skip invalid variants
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
			continue // Try next variant
		}

		for results.Next(ctx) {
			var singleTransaction models.TraceResult
			if err := results.Decode(&singleTransaction); err != nil {
				continue
			}

			from := hex.EncodeToString([]byte(singleTransaction.From))

			// Determine transaction direction based on matching from/to
			if strings.EqualFold(from, addressWithoutPrefix) {
				singleTransaction.InOut = 0 // Outgoing
				singleTransaction.Address = []byte(singleTransaction.To)
			} else {
				singleTransaction.InOut = 1 // Incoming
				singleTransaction.Address = []byte(singleTransaction.From)
			}

			// Check if this transaction is already in our list (to avoid duplicates)
			isDuplicate := false
			for _, tx := range transactions {
				// Use bytes.Equal to compare byte slices properly
				if bytes.Equal(tx.Hash, singleTransaction.Hash) {
					isDuplicate = true
					break
				}
			}

			if !isDuplicate {
				transactions = append(transactions, singleTransaction)
			}
		}
		results.Close(ctx)

		// If we found transactions, no need to try other case variants
		if len(transactions) > 0 {
			break
		}
	}

	return transactions, nil
}

func ReturnAllTransactionsByAddress(address string) ([]models.TransactionByAddress, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var transactions []models.TransactionByAddress

	// Ensure address has Z prefix
	formattedAddress := address
	if !strings.HasPrefix(formattedAddress, "Z") {
		formattedAddress = "Z" + formattedAddress
	}

	// Use regex with case insensitivity for address matching
	// This will find addresses regardless of case
	fromRegex := primitive.Regex{Pattern: "^" + regexp.QuoteMeta(formattedAddress) + "$", Options: "i"}
	toRegex := primitive.Regex{Pattern: "^" + regexp.QuoteMeta(formattedAddress) + "$", Options: "i"}

	// Query for transactions where the address is either the sender or receiver
	filter := primitive.D{{Key: "$or", Value: []primitive.D{
		{{Key: "from", Value: fromRegex}},
		{{Key: "to", Value: toRegex}},
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
		{Key: "blockNumber", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "timeStamp", Value: -1}})

	results, err := configs.TransactionByAddressCollection.Find(ctx, filter, opts)
	if err != nil {
		fmt.Printf("Error querying transactions: %v\n", err)
		return nil, err
	}
	defer results.Close(ctx)

	for results.Next(ctx) {
		var singleTransaction models.TransactionByAddress
		if err := results.Decode(&singleTransaction); err != nil {
			fmt.Printf("Error decoding transaction: %v\n", err)
			continue
		}

		// Use case-insensitive comparison for determining transaction direction
		if strings.EqualFold(singleTransaction.From, formattedAddress) {
			singleTransaction.InOut = 0 // Outgoing
			singleTransaction.Address = singleTransaction.To
		} else {
			singleTransaction.InOut = 1 // Incoming
			singleTransaction.Address = singleTransaction.From
		}

		transactions = append(transactions, singleTransaction)
	}

	if len(transactions) == 0 {
		fmt.Printf("No transactions found for address: %s\n", formattedAddress)
	} else {
		fmt.Printf("Found %d transactions for address: %s\n", len(transactions), formattedAddress)
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
		{Key: "from", Value: 1},
		{Key: "to", Value: 1},
		{Key: "txHash", Value: 1},
		{Key: "timeStamp", Value: 1},
		{Key: "amount", Value: 1},
		{Key: "paidFees", Value: 1},
		{Key: "blockNumber", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "timeStamp", Value: -1}})

	if page == 0 {
		page = 1
	}
	opts.SetSkip(int64((page - 1) * limit))
	opts.SetLimit(int64(limit))

	results, err := configs.GetCollection(configs.DB, "transactionByAddress").Find(ctx, primitive.D{}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to query transactions: %v", err)
	}

	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleTransaction models.TransactionByAddress
		if err = results.Decode(&singleTransaction); err != nil {
			return nil, fmt.Errorf("failed to decode transaction: %v", err)
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

	// Normalize address to handle both uppercase and lowercase Z prefix
	normalizedAddress := strings.TrimPrefix(strings.TrimPrefix(address, "Z"), "z")
	decoded, err := hex.DecodeString(normalizedAddress)
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
	defer cancel()

	count, err := configs.GetCollection(configs.DB, "transactionByAddress").CountDocuments(ctx, primitive.D{})
	if err != nil {
		return 0, fmt.Errorf("failed to count transactions: %v", err)
	}

	return int(count), nil
}

func CountTransactions(address string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Ensure address has Z prefix
	formattedAddress := address
	if !strings.HasPrefix(formattedAddress, "Z") {
		formattedAddress = "Z" + formattedAddress
	}

	// Use regex with case insensitivity for address matching
	fromRegex := primitive.Regex{Pattern: "^" + regexp.QuoteMeta(formattedAddress) + "$", Options: "i"}
	toRegex := primitive.Regex{Pattern: "^" + regexp.QuoteMeta(formattedAddress) + "$", Options: "i"}

	filter := primitive.D{{Key: "$or", Value: []primitive.D{
		{{Key: "from", Value: fromRegex}},
		{{Key: "to", Value: toRegex}},
	}}}

	count, err := configs.TransactionByAddressCollection.CountDocuments(ctx, filter)
	if err != nil {
		fmt.Printf("Error counting transactions: %v\n", err)
		return 0, err
	}

	return int(count), nil
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
				// Use hex strings directly
				from := tx.From
				to := tx.To
				txHash := tx.Hash

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

				ensureHexPrefix := func(s string) string {
					if s == "" || s == "0x" || s == "0x0" {
						return "0x0"
					}
					if !strings.HasPrefix(s, "0x") {
						return "0x" + s
					}
					return s
				}

				result = models.Transfer{
					ID:             primitive.NewObjectID(),
					BlockNumber:    ensureHexPrefix(block.Result.Number),
					BlockTimestamp: ensureHexPrefix(block.Result.Timestamp),
					From:           from,
					To:             to,
					TxHash:         txHash,
					Value:          ensureHexPrefix(valueStr),
					GasUsed:        ensureHexPrefix(gasUsedStr),
					GasPrice:       ensureHexPrefix(gasPriceStr),
					Nonce:          ensureHexPrefix(tx.Nonce),
					Signature:      tx.Signature,
					Pk:             tx.PublicKey,
					Size:           ensureHexPrefix(block.Result.Size),
				}
				return result, nil
			}
		}
	}

	// If not found in blocks, try the transfers collection (fallback)
	decoded, err := hex.DecodeString(strings.TrimPrefix(query, "0x"))
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

func ReturnSingleCoinbaseTransfer(query string) (models.Transfer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.Transfer

	decoded, err := hex.DecodeString(strings.TrimPrefix(query, "0x"))
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

	// Round to nearest whole number
	return int64(result.Volume + 0.5)
}

func GetTransactionByHash(hash string) (*models.Transaction, error) {
	collection := configs.GetCollection(configs.DB, "transfer")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Remove "0x" prefix if present and decode hex to bytes
	hash = strings.TrimPrefix(hash, "0x")
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

	// Convert hex string to decimal string for display
	blockNum := transfer.BlockNumber
	if strings.HasPrefix(blockNum, "0x") {
		// Remove 0x prefix and parse as hex
		num, err := strconv.ParseUint(blockNum[2:], 16, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid block number format: %v", err)
		}
		blockNum = strconv.FormatUint(num, 10)
	}

	// Transfer.TxHash is already in hex string format
	return &models.Transaction{
		Hash:        transfer.TxHash,
		BlockNumber: blockNum,
	}, nil
}

func ReturnNonZeroTransactions(address string, page, limit int) ([]models.TransactionByAddress, error) {
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
		{Key: "from", Value: 1},
		{Key: "to", Value: 1},
		{Key: "blockNumber", Value: 1},
	}

	// Sort by timestamp, newest first
	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "timeStamp", Value: -1}})

	// Format the address for query
	formattedAddress := address
	if !strings.HasPrefix(formattedAddress, "Z") {
		formattedAddress = "Z" + formattedAddress
	}

	// Use regex with case insensitivity for address matching
	fromRegex := primitive.Regex{Pattern: "^" + regexp.QuoteMeta(formattedAddress) + "$", Options: "i"}
	toRegex := primitive.Regex{Pattern: "^" + regexp.QuoteMeta(formattedAddress) + "$", Options: "i"}

	// Create a filter for both from and to with this address and non-zero amount
	filter := bson.M{
		"$and": []bson.M{
			{
				"$or": []bson.M{
					{"from": fromRegex},
					{"to": toRegex},
				},
			},
			{"amount": bson.M{"$gt": 0}}, // Only return transactions with amount > 0
		},
	}

	// Apply pagination
	if limit != 0 {
		if page == 0 {
			page = 1
		}
		opts.SetSkip(int64((page - 1) * limit))
		opts.SetLimit(int64(limit))
	}

	// Execute the query
	results, err := configs.TransactionByAddressCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer results.Close(ctx)

	// Process the results
	for results.Next(ctx) {
		var singleTransaction models.TransactionByAddress
		if err = results.Decode(&singleTransaction); err != nil {
			return nil, err
		}

		// Set the inOut flag based on the address's relation to the transaction
		if strings.EqualFold(singleTransaction.From, formattedAddress) {
			singleTransaction.InOut = 0 // Outgoing
			singleTransaction.Address = singleTransaction.To
		} else {
			singleTransaction.InOut = 1 // Incoming
			singleTransaction.Address = singleTransaction.From
		}
		transactions = append(transactions, singleTransaction)
	}

	// Check for cursor errors
	if err = results.Err(); err != nil {
		return nil, err
	}

	return transactions, nil
}
