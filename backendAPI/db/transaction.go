package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math"
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
		{Key: "amountWei", Value: 1},
		{Key: "paidFees", Value: 1},
		{Key: "paidFeesWei", Value: 1},
		{Key: "blockNumber", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "timeStamp", Value: -1}}).
		SetLimit(100)

	results, err := configs.TransactionByAddressCollection.Find(ctx, primitive.D{}, opts)
	if err != nil {
		log.Printf("error querying latest transactions: %v", err)
		return nil, err
	}

	defer results.Close(ctx)
	for results.Next(ctx) {
		var singleTransaction models.TransactionByAddress
		if err = results.Decode(&singleTransaction); err != nil {
			log.Printf("error decoding latest transaction: %v", err)
			continue
		}
		transactions = append(transactions, singleTransaction)
	}

	return transactions, nil
}

// ReturnAllInternalTransactionsByAddress returns one page (default 10, max
// 50) of internal transactions for the given address, sorted by
// blockTimestamp desc. Pagination was added to keep the worst-case
// $or+sort cost from blowing past the 10 s context budget on high-volume
// addresses (validators, contracts) where the unpaginated 200-row hit
// would routinely time out.
func ReturnAllInternalTransactionsByAddress(address string, page, limit int) ([]models.InternalTx, error) {
	page, limit = clampPage(page, limit, 10, 50)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var transactions []models.InternalTx

	// Normalize to canonical Q-prefix format used by the syncer.
	normalizedAddress := normalizeAddress(address)

	filter := addressOrFilter(normalizedAddress)

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
		SetSort(primitive.D{{Key: "blockTimestamp", Value: -1}}).
		SetSkip(int64(page-1) * int64(limit)).
		SetLimit(int64(limit))

	results, err := configs.InternalTransactionByAddressCollection.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer results.Close(ctx)

	for results.Next(ctx) {
		var singleTransaction models.InternalTx
		if err := results.Decode(&singleTransaction); err != nil {
			continue
		}

		transactions = append(transactions, singleTransaction)
	}

	return transactions, nil
}

// ReturnAllTransactionsByAddress returns one page (default 10, max 50) of
// transactions involving the address, sorted by timeStamp desc. Paginated
// to bound the $or+sort cost: high-volume addresses (validators,
// contracts) routinely timed out under the unpaginated 200-row hit.
func ReturnAllTransactionsByAddress(address string, page, limit int) ([]models.TransactionByAddress, error) {
	page, limit = clampPage(page, limit, 10, 50)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var transactions []models.TransactionByAddress

	// Normalize to the canonical Q-prefix form stored by the syncer.
	normalizedAddress := normalizeAddress(address)
	filter := addressOrFilter(normalizedAddress)

	projection := primitive.D{
		{Key: "timeStamp", Value: 1},
		{Key: "amount", Value: 1},
		{Key: "amountWei", Value: 1},
		{Key: "inOut", Value: 1},
		{Key: "txHash", Value: 1},
		{Key: "txType", Value: 1},
		{Key: "from", Value: 1},
		{Key: "to", Value: 1},
		{Key: "paidFees", Value: 1},
		{Key: "paidFeesWei", Value: 1},
		{Key: "blockNumber", Value: 1},
	}

	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "timeStamp", Value: -1}}).
		SetSkip(int64(page-1) * int64(limit)).
		SetLimit(int64(limit))

	results, err := configs.TransactionByAddressCollection.Find(ctx, filter, opts)
	if err != nil {
		log.Printf("error querying transactions: %v", err)
		return nil, err
	}
	defer results.Close(ctx)

	for results.Next(ctx) {
		var singleTransaction models.TransactionByAddress
		if err := results.Decode(&singleTransaction); err != nil {
			log.Printf("error decoding transaction: %v", err)
			continue
		}

		if strings.EqualFold(singleTransaction.From, normalizedAddress) {
			singleTransaction.InOut = 0 // Outgoing
			singleTransaction.Address = singleTransaction.To
		} else {
			singleTransaction.InOut = 1 // Incoming
			singleTransaction.Address = singleTransaction.From
		}

		transactions = append(transactions, singleTransaction)
	}

	if len(transactions) == 0 {
		log.Printf("no transactions found for address: %s", normalizedAddress)
	} else {
		log.Printf("found %d transactions for address: %s", len(transactions), normalizedAddress)
	}

	return transactions, nil
}

func ReturnTransactionsNetwork(page, limit int) ([]models.TransactionByAddress, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	var transactions []models.TransactionByAddress
	defer cancel()

	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	projection := primitive.D{
		{Key: "inOut", Value: 1},
		{Key: "txType", Value: 1},
		{Key: "from", Value: 1},
		{Key: "to", Value: 1},
		{Key: "txHash", Value: 1},
		{Key: "timeStamp", Value: 1},
		{Key: "amount", Value: 1},
		{Key: "amountWei", Value: 1},
		{Key: "paidFees", Value: 1},
		{Key: "paidFeesWei", Value: 1},
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

	results, err := configs.TransactionByAddressCollection.Find(ctx, primitive.D{}, opts)
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

// CountTransactionsNetwork returns the total transactionByAddress row count.
// Uses countDocumentsResilient (fast metadata read with an exact-count fallback
// when the metadata reads 0, which it currently does on this deployment).
// Returns int64 to match sibling count helpers and to avoid an implicit cap
// at 2^31-1 on long-running chains.
func CountTransactionsNetwork() (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	count, err := countDocumentsResilient(ctx, configs.TransactionByAddressCollection)
	if err != nil {
		return 0, fmt.Errorf("failed to count transactions: %v", err)
	}

	return count, nil
}

func CountTransactions(address string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Normalize to canonical Q-prefix, matches syncer write format.
	normalizedAddress := normalizeAddress(address)

	filter := addressOrFilter(normalizedAddress)

	count, err := configs.TransactionByAddressCollection.CountDocuments(ctx, filter)
	if err != nil {
		log.Printf("error counting transactions: %v", err)
		return 0, err
	}

	return int(count), nil
}

// CountInternalTransactionsByAddress mirrors CountTransactions for the
// internalTransactionByAddress collection, so the address page can size
// the Internal Txns pagination honestly instead of guessing from the
// clipped page it received.
func CountInternalTransactionsByAddress(address string) (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Normalize to canonical Q-prefix, matches syncer write format.
	normalizedAddress := normalizeAddress(address)

	filter := addressOrFilter(normalizedAddress)

	count, err := configs.InternalTransactionByAddressCollection.CountDocuments(ctx, filter)
	if err != nil {
		log.Printf("error counting internal transactions: %v", err)
		return 0, err
	}

	return int(count), nil
}

// ReturnAddressActivityRange returns the unix timestamps (seconds) of the
// oldest and newest native transactions involving the address, or (0, 0)
// when it has none. The address page's Activity card needs the true range;
// deriving it from whichever page of transactions happens to be loaded
// shows the oldest row of the newest page as "First Activity".
//
// timeStamp is persisted as a fixed-width "0x" + 8-hex-digit string, so a
// lexicographic sort orders it numerically (until 2106), the same
// assumption every timeStamp sort in this file already makes.
func ReturnAddressActivityRange(address string) (int64, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	normalizedAddress := normalizeAddress(address)

	filter := addressOrFilter(normalizedAddress)

	first, err := fetchActivityBoundary(ctx, filter, 1)
	if err == mongo.ErrNoDocuments {
		return 0, 0, nil
	}
	if err != nil {
		log.Printf("error fetching first activity: %v", err)
		return 0, 0, err
	}

	last, err := fetchActivityBoundary(ctx, filter, -1)
	if err != nil {
		log.Printf("error fetching last activity: %v", err)
		return 0, 0, err
	}

	return first, last, nil
}

// fetchActivityBoundary returns the unix timestamp (seconds) of the oldest
// (order 1) or newest (order -1) native transaction matching the address
// filter, or mongo.ErrNoDocuments when there are none. Index-backed: the
// sort rides the same timeStamp ordering assumption documented on
// ReturnAddressActivityRange.
func fetchActivityBoundary(ctx context.Context, filter bson.M, order int) (int64, error) {
	var doc struct {
		TimeStamp string `bson:"timeStamp"`
	}
	opts := options.FindOne().
		SetProjection(primitive.D{{Key: "timeStamp", Value: 1}}).
		SetSort(primitive.D{{Key: "timeStamp", Value: order}})
	err := configs.TransactionByAddressCollection.FindOne(ctx, filter, opts).Decode(&doc)
	if err != nil {
		return 0, err
	}
	ts, err := strconv.ParseInt(strings.TrimPrefix(doc.TimeStamp, "0x"), 16, 64)
	if err != nil {
		return 0, fmt.Errorf("malformed timeStamp %q: %w", doc.TimeStamp, err)
	}
	return ts, nil
}

// FirstSeen returns the unix timestamp (seconds) of the oldest native
// transaction involving the address, or 0 when it has none (the richlist
// renders that as a dash). Same first-boundary lookup the address page's
// Activity card uses via ReturnAddressActivityRange.
func FirstSeen(address string) int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	filter := addressOrFilter(normalizeAddress(address))

	first, err := fetchActivityBoundary(ctx, filter, 1)
	if err == mongo.ErrNoDocuments {
		return 0
	}
	if err != nil {
		log.Printf("error fetching first seen for %s: %v", address, err)
		return 0
	}

	return first
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
					Input:          tx.Data,
				}
				return result, nil
			}
		}
	}

	// The historical transfers-collection fallback was deleted: it filtered
	// the string txHash field with decoded []byte, so it could never match
	// (and models.Transfer's field types would fail the Decode anyway). The
	// blocks-collection lookup above, backed by the result_transactions_hash
	// index, is the real path.
	return result, err
}

// CountInternalTxsByTxHashes returns a map keyed by parent tx hash with
// the number of internalTransactionByAddress rows persisted for each.
// Used by /block/:n alongside CountTokenTransfersByTxHashes to surface
// a per-tx activity hint in the block's tx table.
//
// Hashes are matched case-insensitively against whatever the syncer
// stored (it writes them verbatim from the RPC response; we don't
// re-normalise here).
func CountInternalTxsByTxHashes(txHashes []string) (map[string]int, error) {
	out := make(map[string]int)
	if len(txHashes) == 0 {
		return out, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pipeline := []bson.M{
		{"$match": bson.M{"hash": bson.M{"$in": txHashes}}},
		{"$group": bson.M{"_id": "$hash", "count": bson.M{"$sum": 1}}},
	}
	cursor, err := configs.InternalTransactionByAddressCollection.Aggregate(ctx, pipeline)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return out, nil
		}
		return nil, err
	}
	defer cursor.Close(ctx)

	for cursor.Next(ctx) {
		var row struct {
			ID    string `bson:"_id"`
			Count int    `bson:"count"`
		}
		if err := cursor.Decode(&row); err != nil {
			continue
		}
		out[row.ID] = row.Count
	}
	return out, nil
}

// GetInternalTransactionsByTxHash returns every internal-call entry the
// syncer captured under a given outer tx hash. Used by /tx/:hash to
// render the Internal Transactions panel. Returns an empty slice (not
// nil) when nothing matches so the response shape stays predictable.
//
// The syncer keys these docs by `hash` (the parent tx hash), so this is
// a flat collection lookup. Ordered by traceAddress to preserve the
// EVM-emitted call order; an empty traceAddress sorts first (the
// top-level call, when one was recorded).
func GetInternalTransactionsByTxHash(txHash string) ([]models.InternalTx, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := configs.InternalTransactionByAddressCollection.Find(
		ctx,
		bson.M{"hash": txHash},
		options.Find().SetSort(bson.D{{Key: "traceAddress", Value: 1}}),
	)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return []models.InternalTx{}, nil
		}
		return nil, err
	}
	defer cursor.Close(ctx)

	var out []models.InternalTx
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = []models.InternalTx{}
	}
	return out, nil
}

func ReturnDailyTransactionsVolume() int64 {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var result models.TransactionsVolume

	err := configs.DailyTransactionsVolumeCollection.FindOne(ctx, primitive.D{}).Decode(&result)
	if err != nil {
		log.Printf("error fetching daily transactions volume: %v", err)
		return 0
	}

	// Round to nearest whole number
	return int64(math.Round(result.Volume))
}

func GetTransactionByHash(hash string) (*models.Transaction, error) {
	collection := configs.TransferCollections
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
		{Key: "amountWei", Value: 1},
		{Key: "from", Value: 1},
		{Key: "to", Value: 1},
		{Key: "blockNumber", Value: 1},
	}

	// Sort by timestamp, newest first
	opts := options.Find().
		SetProjection(projection).
		SetSort(primitive.D{{Key: "timeStamp", Value: -1}})

	// Normalize to canonical Q-prefix form stored by the syncer.
	normalizedAddress := normalizeAddress(address)
	filter := bson.M{
		"$and": []bson.M{
			addressOrFilter(normalizedAddress),
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
		if strings.EqualFold(singleTransaction.From, normalizedAddress) {
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
