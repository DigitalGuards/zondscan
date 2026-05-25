package db

import (
	"backendAPI/configs"
	"backendAPI/models"
	"context"
	"sort"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// GetTokenBalancesByAddress returns token balances for a given wallet address
// with token metadata (name, symbol, decimals) included.
//
// `standardFilter`, when non-nil and non-empty, narrows the result to one
// of "ERC-20" / "ERC-721" / "ERC-1155". A nil filter keeps the legacy
// behaviour of returning every standard, important because the wallet
// auto-discovery flow still defaults to "give me everything you know".
// The wallet's per-card flows pass an explicit filter via the route's
// `?standard=` query param.
func GetTokenBalancesByAddress(address string, standardFilter *string) ([]models.TokenBalance, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Normalize address to canonical Q-prefix form
	searchAddresses := normalizeAddressBoth(address)

	collection := configs.GetCollection(configs.DB, "tokenBalances")

	matchStage := bson.M{
		"holderAddress": bson.M{"$in": searchAddresses},
	}
	if standardFilter != nil && *standardFilter != "" {
		matchStage["tokenStandard"] = *standardFilter
	}

	// Aggregation pipeline to join with contractCode for token metadata
	pipeline := []bson.M{
		// Match token balances for this address (case-insensitive)
		{
			"$match": matchStage,
		},
		// Add lowercase version of contractAddress for case-insensitive lookup
		{
			"$addFields": bson.M{
				"contractAddressLower": bson.M{"$toLower": "$contractAddress"},
			},
		},
		// Join with contractCode collection using lowercase addresses
		{
			"$lookup": bson.M{
				"from": "contractCode",
				"let":  bson.M{"contractAddr": "$contractAddressLower"},
				"pipeline": []bson.M{
					{
						"$match": bson.M{
							"$expr": bson.M{
								"$eq": []interface{}{
									bson.M{"$toLower": "$address"},
									"$$contractAddr",
								},
							},
						},
					},
				},
				"as": "tokenInfo",
			},
		},
		// Unwind the tokenInfo array (should be single element)
		{
			"$unwind": bson.M{
				"path":                       "$tokenInfo",
				"preserveNullAndEmptyArrays": true,
			},
		},
		// Project final structure with token metadata. Phase 2: include
		// tokenID + tokenStandard so the address page can render NFT
		// holdings per (collection, tokenID) row.
		{
			"$project": bson.M{
				"contractAddress": 1,
				"holderAddress":   1,
				"balance":         1,
				"blockNumber":     1,
				"updatedAt":       1,
				"tokenID":         1,
				"tokenStandard":   1,
				"name":            "$tokenInfo.name",
				"symbol":          "$tokenInfo.symbol",
				"decimals":        "$tokenInfo.decimals",
			},
		},
		// Convert balance string to decimal for proper numeric sorting
		{
			"$addFields": bson.M{
				"balanceDecimal": bson.M{"$toDecimal": "$balance"},
			},
		},
		// Sort by balance descending (highest value tokens first)
		{
			"$sort": bson.M{"balanceDecimal": -1},
		},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var results []models.TokenBalance
	if err := cursor.All(ctx, &results); err != nil {
		return nil, err
	}

	// Return empty slice instead of nil
	if results == nil {
		results = make([]models.TokenBalance, 0)
	}

	return results, nil
}

// normalizeAddress converts any address format to the canonical Q-prefix
// form that the syncer stores in MongoDB (uppercase Q + lowercase hex).
func normalizeAddress(address string) string {
	if strings.HasPrefix(address, "0x") || strings.HasPrefix(address, "0X") {
		return "Q" + strings.ToLower(address[2:])
	}
	if strings.HasPrefix(address, "Q") || strings.HasPrefix(address, "q") {
		return "Q" + strings.ToLower(address[1:])
	}
	return "Q" + strings.ToLower(address)
}

// normalizeAddressBoth returns the canonical Q-prefix address as a slice.
// The slice form is kept so callers using "$in" can remain unchanged.
func normalizeAddressBoth(address string) []string {
	return []string{normalizeAddress(address)}
}

// GetTokenHolders returns all holders of a specific token contract with pagination.
//
// Phase 2: when `tokenID` is empty, holders are aggregated across all ids
// (one row per distinct holder; balance is the sum of per-id quantities,
// useful for ERC-1155 "how many copies in total" or ERC-721 "how many NFTs
// owned"). When `tokenID` is set, only rows for that specific id are
// returned (one per holder of that id).
//
// ERC-20 behaviour is unchanged: rows already have a null tokenID, so the
// aggregate-by-holder path produces the same shape as before, one row per
// (contract, holder) with the existing balance string.
func GetTokenHolders(contractAddress, tokenID string, page, limit int) ([]models.TokenBalance, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	contractVariants := normalizeAddressBoth(contractAddress)
	matchStage := bson.M{"contractAddress": bson.M{"$in": contractVariants}}
	if tokenID != "" {
		matchStage["tokenID"] = tokenID
	}

	collection := configs.GetCollection(configs.DB, "tokenBalances")

	// Pagination skip+limit (decimal-sorted by balance, descending).
	skip := int64(page * limit)
	lim := int64(limit)

	// uint256 balances can be up to 78 digits, well past Decimal128's
	// 34-digit limit. A raw $toDecimal would throw on extreme values; both
	// paths below avoid that. The tokenID-scoped path sorts by string
	// (length, lex) which gives correct numeric order for any uint256
	// without Decimal128 at all. The aggregated path needs $sum so it
	// keeps Decimal128 but wraps the $toDecimal in $convert with onError=0,
	// any single oversized balance falls back to 0 for sort purposes only
	// (the original string is still in the row).
	safeDecimalBalance := bson.M{
		"$convert": bson.M{
			"input":   "$balance",
			"to":      "decimal",
			"onError": 0,
			"onNull":  0,
		},
	}

	// tokenID-scoped path: rows already represent (holder, id) tuples, no
	// grouping needed.
	if tokenID != "" {
		totalCount, err := collection.CountDocuments(ctx, matchStage)
		if err != nil {
			return nil, 0, err
		}
		pipeline := []bson.M{
			{"$match": matchStage},
			{
				"$addFields": bson.M{
					"balanceLen": bson.M{"$strLenCP": bson.M{"$ifNull": []interface{}{"$balance", ""}}},
				},
			},
			{"$sort": bson.D{{Key: "balanceLen", Value: -1}, {Key: "balance", Value: -1}}},
			{"$skip": skip},
			{"$limit": lim},
			{"$project": bson.M{"balanceLen": 0}},
		}
		cursor, err := collection.Aggregate(ctx, pipeline)
		if err != nil {
			return nil, 0, err
		}
		defer cursor.Close(ctx)

		var holders []models.TokenBalance
		if err := cursor.All(ctx, &holders); err != nil {
			return nil, 0, err
		}
		if holders == nil {
			holders = make([]models.TokenBalance, 0)
		}
		return holders, int(totalCount), nil
	}

	// Aggregated-by-holder path (no tokenID filter).
	// $group collapses the per-id rows; balanceSum is the cross-id total.
	// For ERC-20 (no tokenID rows) each holder already has one row, so the
	// sum equals the row's balance.
	groupStage := bson.M{
		"$group": bson.M{
			"_id":             "$holderAddress",
			"contractAddress": bson.M{"$first": "$contractAddress"},
			"balanceDecimal":  bson.M{"$sum": safeDecimalBalance},
			"blockNumber":     bson.M{"$max": "$blockNumber"},
			"updatedAt":       bson.M{"$max": "$updatedAt"},
			"tokenStandard":   bson.M{"$first": "$tokenStandard"},
		},
	}

	// Distinct-holder count. Mongo doesn't have a clean "$count after $group"
	// alongside the paginated cursor, so issue a separate aggregation that
	// stops at $count.
	countPipeline := []bson.M{
		{"$match": matchStage},
		groupStage,
		{"$count": "total"},
	}
	countCursor, err := collection.Aggregate(ctx, countPipeline)
	if err != nil {
		return nil, 0, err
	}
	defer countCursor.Close(ctx)
	var countResult []struct {
		Total int `bson:"total"`
	}
	if err := countCursor.All(ctx, &countResult); err != nil {
		return nil, 0, err
	}
	totalCount := 0
	if len(countResult) > 0 {
		totalCount = countResult[0].Total
	}

	pipeline := []bson.M{
		{"$match": matchStage},
		groupStage,
		// Reshape grouped doc back into TokenBalance.
		{
			"$project": bson.M{
				"_id":             0,
				"contractAddress": 1,
				"holderAddress":   "$_id",
				"balance":         bson.M{"$toString": "$balanceDecimal"},
				"blockNumber":     1,
				"updatedAt":       1,
				"tokenStandard":   1,
				"balanceDecimal":  1,
			},
		},
		{"$sort": bson.M{"balanceDecimal": -1}},
		{"$skip": skip},
		{"$limit": lim},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var holders []models.TokenBalance
	if err := cursor.All(ctx, &holders); err != nil {
		return nil, 0, err
	}
	if holders == nil {
		holders = make([]models.TokenBalance, 0)
	}

	return holders, totalCount, nil
}

// GetTokenIDs returns the distinct list of tokenIDs minted on a given NFT
// contract, with the holder count for each id. Paginated. ERC-20 contracts
// have no tokenID column and will return an empty list.
//
// Holder count is the number of distinct (holder, id) rows for that id,
// i.e. how many addresses currently hold at least one copy of the id.
func GetTokenIDs(contractAddress string, page, limit int) ([]TokenIDSummary, int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	contractVariants := normalizeAddressBoth(contractAddress)
	matchStage := bson.M{
		"contractAddress": bson.M{"$in": contractVariants},
		"tokenID":         bson.M{"$exists": true, "$ne": ""},
	}

	collection := configs.GetCollection(configs.DB, "tokenBalances")

	// Two-stage group to count distinct (tokenID, holder) pairs per id
	// without holding the holder list in memory. First stage collapses
	// duplicate (id, holder) rows; second stage counts them per id. Avoids
	// the $addToSet array-build that would OOM on hot ERC-1155 collections.
	firstGroup := bson.M{
		"$group": bson.M{
			"_id": bson.M{
				"tokenID":       "$tokenID",
				"holderAddress": "$holderAddress",
			},
			"tokenStandard": bson.M{"$first": "$tokenStandard"},
			"blockNumber":   bson.M{"$max": "$blockNumber"},
			"updatedAt":     bson.M{"$max": "$updatedAt"},
		},
	}
	secondGroup := bson.M{
		"$group": bson.M{
			"_id":           "$_id.tokenID",
			"holderCount":   bson.M{"$sum": 1},
			"tokenStandard": bson.M{"$first": "$tokenStandard"},
			"blockNumber":   bson.M{"$max": "$blockNumber"},
			"updatedAt":     bson.M{"$max": "$updatedAt"},
		},
	}

	// Total count of distinct ids.
	countPipeline := []bson.M{
		{"$match": matchStage},
		firstGroup,
		secondGroup,
		{"$count": "total"},
	}
	countCursor, err := collection.Aggregate(ctx, countPipeline)
	if err != nil {
		return nil, 0, err
	}
	defer countCursor.Close(ctx)
	var countResult []struct {
		Total int `bson:"total"`
	}
	if err := countCursor.All(ctx, &countResult); err != nil {
		return nil, 0, err
	}
	totalCount := 0
	if len(countResult) > 0 {
		totalCount = countResult[0].Total
	}

	// Sort key: full uint256 IDs can be up to 78 digits, well past
	// Decimal128's 34-digit limit, so $convert-to-decimal will throw on
	// extreme values. Sort by (string length DESC of "1" -> "01" no, we
	// want ASC numeric order so shorter-len first, then lex). Lexicographic
	// sort on equal-length numeric strings matches numeric sort, so the
	// (len, lex) tuple gives correct numeric ascending order for the full
	// uint256 range without involving Decimal128.
	pipeline := []bson.M{
		{"$match": matchStage},
		firstGroup,
		secondGroup,
		{
			"$project": bson.M{
				"_id":           0,
				"tokenID":       "$_id",
				"holderCount":   1,
				"tokenStandard": 1,
				"blockNumber":   1,
				"updatedAt":     1,
				"tokenIDLen":    bson.M{"$strLenCP": bson.M{"$ifNull": []interface{}{"$_id", ""}}},
			},
		},
		{"$sort": bson.D{{Key: "tokenIDLen", Value: 1}, {Key: "tokenID", Value: 1}}},
		{"$skip": int64(page * limit)},
		{"$limit": int64(limit)},
		{"$project": bson.M{"tokenIDLen": 0}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var summaries []TokenIDSummary
	if err := cursor.All(ctx, &summaries); err != nil {
		return nil, 0, err
	}
	if summaries == nil {
		summaries = make([]TokenIDSummary, 0)
	}

	// Phase 3b enrichment: pull per-token name + image from tokenMetadata.
	// One $in query against the paginated id list, then a per-row merge.
	// Skipped silently if the lookup fails; the response is still useful
	// without metadata.
	if len(summaries) > 0 {
		ids := make([]string, 0, len(summaries))
		for _, s := range summaries {
			ids = append(ids, s.TokenID)
		}
		if meta, mErr := GetTokenMetadataMap(contractAddress, ids); mErr == nil {
			for i := range summaries {
				if m, ok := meta[summaries[i].TokenID]; ok && m != nil {
					summaries[i].Name = m.Name
					summaries[i].Description = m.Description
					summaries[i].Image = m.Image
				}
			}
		}
	}

	return summaries, totalCount, nil
}

// TokenIDSummary is one row in the GET /token/:addr/tokens response.
//
// Phase 3b adds the optional metadata fields (Name + Image + Description)
// pulled from the per-token tokenMetadata collection. They are populated
// inline by GetTokenIDs after the distinct-id aggregation completes, so
// the response is a single payload instead of a roundtrip per row.
type TokenIDSummary struct {
	TokenID       string `json:"tokenID" bson:"tokenID"`
	HolderCount   int    `json:"holderCount" bson:"holderCount"`
	TokenStandard string `json:"tokenStandard,omitempty" bson:"tokenStandard,omitempty"`
	BlockNumber   string `json:"blockNumber,omitempty" bson:"blockNumber,omitempty"`
	UpdatedAt     string `json:"updatedAt,omitempty" bson:"updatedAt,omitempty"`
	Name          string `json:"name,omitempty" bson:"name,omitempty"`
	Description   string `json:"description,omitempty" bson:"description,omitempty"`
	Image         string `json:"image,omitempty" bson:"image,omitempty"`
}

// GetTokenTransfers returns all transfers for a specific token contract with pagination
func GetTokenTransfers(contractAddress string, page, limit int) ([]models.TokenTransfer, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	contractVariants := normalizeAddressBoth(contractAddress)
	contractFilter := bson.M{"contractAddress": bson.M{"$in": contractVariants}}
	collection := configs.GetCollection(configs.DB, "tokenTransfers")

	// Count total transfers
	totalCount, err := collection.CountDocuments(ctx, contractFilter)
	if err != nil {
		return nil, 0, err
	}

	// Find with pagination, sorted by block number descending (most recent first)
	opts := options.Find().
		SetSort(bson.D{{Key: "blockNumber", Value: -1}}).
		SetSkip(int64(page * limit)).
		SetLimit(int64(limit))

	cursor, err := collection.Find(ctx, contractFilter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var transfers []models.TokenTransfer
	if err := cursor.All(ctx, &transfers); err != nil {
		return nil, 0, err
	}

	if transfers == nil {
		transfers = make([]models.TokenTransfer, 0)
	}

	return transfers, totalCount, nil
}

// GetTokenTransfersByAddress returns token transfer events touching the given
// holder address (either side: from or to), with pagination. Matches the
// shape of GetTokenTransfers so the address-page consumer can render the
// same row schema produced by /token/:address/transfers. Sorted by
// blockNumber desc so the most recent transfer is first.
//
// The address is normalised to the canonical Q-prefix lowercase form the
// syncer writes; we don't expand to a multi-variant $in here because every
// token transfer row is written via validation.ConvertToQAddress(), so a
// single canonical lookup hits the from_block_idx / to_block_idx indexes
// without forcing the planner onto a slower variant scan.
func GetTokenTransfersByAddress(address string, page, limit int) ([]models.TokenTransfer, int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if page < 0 {
		page = 0
	}
	if limit < 1 {
		limit = 25
	}
	// 250 matches the frontend's bulk fetch (TanStackTable lazy-loads the
	// full holder history once so the in-table search/pagination work
	// without per-page round-trips). A lower ceiling here silently
	// truncated active addresses, and the "(N)" tab badge then under-
	// reported their real activity. The `total` returned in the response
	// is still the unbounded CountDocuments, so the tab can render the
	// true number regardless of what was paginated in.
	if limit > 250 {
		limit = 250
	}

	canonical := normalizeAddress(address)
	filter := bson.M{"$or": []bson.M{
		{"from": canonical},
		{"to": canonical},
	}}
	collection := configs.GetCollection(configs.DB, "tokenTransfers")

	totalCount, err := collection.CountDocuments(ctx, filter)
	if err != nil {
		return nil, 0, err
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "blockNumber", Value: -1}}).
		SetSkip(int64(page * limit)).
		SetLimit(int64(limit))

	cursor, err := collection.Find(ctx, filter, opts)
	if err != nil {
		return nil, 0, err
	}
	defer cursor.Close(ctx)

	var transfers []models.TokenTransfer
	if err := cursor.All(ctx, &transfers); err != nil {
		return nil, 0, err
	}
	if transfers == nil {
		transfers = make([]models.TokenTransfer, 0)
	}
	return transfers, totalCount, nil
}

// GetTokenInfo returns summary information about a token
func GetTokenInfo(contractAddress string) (*models.TokenInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	contractVariants := normalizeAddressBoth(contractAddress)
	contractInFilter := bson.M{"$in": contractVariants}

	// Get contract info
	contractCollection := configs.GetCollection(configs.DB, "contractCode")
	var contract models.ContractInfo
	err := contractCollection.FindOne(ctx, bson.M{"address": contractInFilter}).Decode(&contract)
	if err != nil {
		return nil, err
	}

	// Count holders
	balanceCollection := configs.GetCollection(configs.DB, "tokenBalances")
	holderCount, err := balanceCollection.CountDocuments(ctx, bson.M{"contractAddress": contractInFilter})
	if err != nil {
		holderCount = 0
	}

	// Count transfers
	transferCollection := configs.GetCollection(configs.DB, "tokenTransfers")
	transferCount, err := transferCollection.CountDocuments(ctx, bson.M{"contractAddress": contractInFilter})
	if err != nil {
		transferCount = 0
	}

	return &models.TokenInfo{
		ContractAddress: contract.ContractAddress,
		Name:            contract.TokenName,
		Symbol:          contract.TokenSymbol,
		Decimals:        int(contract.TokenDecimals),
		TotalSupply:     contract.TotalSupply,
		HolderCount:     int(holderCount),
		TransferCount:   transferCount,
		CreatorAddress:  contract.ContractCreatorAddress,
		CreationTxHash:  contract.CreationTransaction,
		CreationBlock:   contract.CreationBlockNumber,
	}, nil
}

// CountTokenTransfersByTxHashes returns a map keyed by tx hash with the
// number of tokenTransfers rows persisted for each. Used by /block/:n
// to surface a per-tx "this tx moved N tokens" hint without making the
// client issue one /tx/:hash per row.
//
// Hashes are matched case-insensitively against the lowercase-with-0x
// canonical form the syncer stores. Missing hashes don't appear in the
// returned map.
func CountTokenTransfersByTxHashes(txHashes []string) (map[string]int, error) {
	out := make(map[string]int)
	if len(txHashes) == 0 {
		return out, nil
	}

	normalized := make([]string, 0, len(txHashes))
	for _, h := range txHashes {
		n := strings.ToLower(h)
		if !strings.HasPrefix(n, "0x") {
			n = "0x" + n
		}
		normalized = append(normalized, n)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pipeline := []bson.M{
		{"$match": bson.M{"txHash": bson.M{"$in": normalized}}},
		{"$group": bson.M{"_id": "$txHash", "count": bson.M{"$sum": 1}}},
	}
	cursor, err := configs.GetCollection(configs.DB, "tokenTransfers").Aggregate(ctx, pipeline)
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

// GetTokenTransfersByTxHash returns every token transfer row associated with
// a given transaction hash. A single tx can produce multiple rows:
//
//   - DEX swaps emit several ERC-20 Transfer events (router, pool, recipient).
//   - ERC-1155 TransferBatch is fanned out one row per (id, value) tuple by
//     the syncer.
//
// FindOne-style readers silently dropped everything past the first row, so
// only the first transfer ever rendered on the tx page. Returns an empty
// slice (not nil) when nothing matches.
//
// The syncer stores txHash lowercase with a 0x prefix; we normalize input
// the same way before querying.
func GetTokenTransfersByTxHash(txHash string) ([]models.TokenTransfer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	collection := configs.GetCollection(configs.DB, "tokenTransfers")

	normalizedHash := strings.ToLower(txHash)
	if !strings.HasPrefix(normalizedHash, "0x") {
		normalizedHash = "0x" + normalizedHash
	}

	cursor, err := collection.Find(ctx, bson.M{"txHash": normalizedHash})
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return []models.TokenTransfer{}, nil
		}
		return nil, err
	}
	defer cursor.Close(ctx)

	var transfers []models.TokenTransfer
	if err := cursor.All(ctx, &transfers); err != nil {
		return nil, err
	}
	if transfers == nil {
		transfers = []models.TokenTransfer{}
	}

	// Sort by logIndex ascending so emission order is preserved within the
	// tx. logIndex is persisted as a "0x..." hex string, sorting it as a
	// BSON string would put "0xa" after "0x10" because string comparison is
	// lexicographic. Sort numerically here instead. Legacy direct-calldata
	// rows have an empty logIndex and we pin them to the front (sort key -1)
	// so a synthetic row and any later event rows for the same call stay
	// in a stable, intuitive order.
	sort.SliceStable(transfers, func(i, j int) bool {
		return logIndexSortKey(transfers[i].LogIndex) < logIndexSortKey(transfers[j].LogIndex)
	})

	return transfers, nil
}

// logIndexSortKey turns a "0x..." hex string into an int64 for ascending
// sort. Empty input maps to -1 to keep legacy synthetic rows first.
// Malformed input falls back to 0, the worst case is a stable but slightly
// out-of-order row (never a panic, the syncer's own data path is the only
// writer of this field and always emits a clean hex string).
func logIndexSortKey(s string) int64 {
	if s == "" {
		return -1
	}
	trimmed := strings.TrimPrefix(strings.ToLower(s), "0x")
	v, err := strconv.ParseInt(trimmed, 16, 64)
	if err != nil {
		return 0
	}
	return v
}

// GetNFTBalancesByAddress returns per-(contract, tokenID) NFT holdings for
// a given wallet address, joined with both the collection-level
// `contractCode` row and the per-tokenID `tokenMetadata` row so the wallet
// can render a names+thumbnails picker without a round-trip per token.
//
// `standardFilter`, when non-nil, scopes to ERC-721 or ERC-1155. Default
// is "both NFT standards". The caller is expected to have already
// rejected ERC-20 (the route handler does this) since this function
// only matches on NFT standards.
//
// Sort: collection, then tokenID by (string length, lex) so uint256 ids
// past Decimal128's 34-digit limit still sort correctly without a
// $toDecimal cast.
func GetNFTBalancesByAddress(address string, standardFilter *string) ([]models.NFTBalance, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	searchAddresses := normalizeAddressBoth(address)

	standards := []string{"ERC-721", "ERC-1155"}
	if standardFilter != nil && *standardFilter != "" {
		standards = []string{*standardFilter}
	}

	matchStage := bson.M{
		"holderAddress": bson.M{"$in": searchAddresses},
		"tokenStandard": bson.M{"$in": standards},
	}

	collection := configs.GetCollection(configs.DB, "tokenBalances")

	pipeline := []bson.M{
		{"$match": matchStage},
		// Join the collection-level row (name/symbol/metadataName/etc) from
		// contractCode. Case-insensitive lookup on the lowercase address to
		// match the existing GetTokenBalancesByAddress pattern.
		{
			"$addFields": bson.M{
				"contractAddressLower": bson.M{"$toLower": "$contractAddress"},
			},
		},
		{
			"$lookup": bson.M{
				"from": "contractCode",
				"let":  bson.M{"contractAddr": "$contractAddressLower"},
				"pipeline": []bson.M{
					{"$match": bson.M{"$expr": bson.M{"$eq": []interface{}{
						bson.M{"$toLower": "$address"}, "$$contractAddr",
					}}}},
				},
				"as": "contractInfo",
			},
		},
		{"$unwind": bson.M{"path": "$contractInfo", "preserveNullAndEmptyArrays": true}},
		// Join the per-tokenID metadata row (Phase 3b). $lookup on the
		// (contract, tokenID) compound key.
		{
			"$lookup": bson.M{
				"from": "tokenMetadata",
				"let":  bson.M{"contractAddr": "$contractAddress", "tokenID": "$tokenID"},
				"pipeline": []bson.M{
					{"$match": bson.M{"$expr": bson.M{"$and": []interface{}{
						bson.M{"$eq": []interface{}{"$contractAddress", "$$contractAddr"}},
						bson.M{"$eq": []interface{}{"$tokenID", "$$tokenID"}},
					}}}},
				},
				"as": "tokenMeta",
			},
		},
		{"$unwind": bson.M{"path": "$tokenMeta", "preserveNullAndEmptyArrays": true}},
		// Project the final NFTBalance shape. Prefer the off-chain
		// metadataName for the collection label when present; falls back
		// to the on-chain name().
		{
			"$project": bson.M{
				"_id":             0,
				"contractAddress": 1,
				"holderAddress":   1,
				"tokenID":         1,
				"tokenStandard":   1,
				"balance":         1,
				"blockNumber":     1,
				"updatedAt":       1,
				// MongoDB quirk: missing fields read as MISSING (distinct
				// from null). `$ne: [MISSING, null]` returns true, so the
				// naive non-null check fires the truthy branch, projects
				// an undefined value, and the field gets dropped from the
				// document entirely. $ifNull coerces both missing AND null
				// to "" so we can test for non-empty in a single comparison.
				// Same treatment on the fallback so a contract with neither
				// metadataName nor name doesn't drop the field.
				"collectionName": bson.M{
					"$cond": []interface{}{
						bson.M{"$ne": []interface{}{
							bson.M{"$ifNull": []interface{}{"$contractInfo.metadataName", ""}},
							"",
						}},
						"$contractInfo.metadataName",
						bson.M{"$ifNull": []interface{}{"$contractInfo.name", ""}},
					},
				},
				"collectionSymbol": bson.M{"$ifNull": []interface{}{"$contractInfo.symbol", ""}},
				"name":             "$tokenMeta.name",
				"description":      "$tokenMeta.description",
				"image":            "$tokenMeta.image",
				"externalURL":      "$tokenMeta.externalURL",
				"attributes":       "$tokenMeta.attributes",
				"tokenIDLen":       bson.M{"$strLenCP": bson.M{"$ifNull": []interface{}{"$tokenID", ""}}},
			},
		},
		// (length, lex) sort handles uint256 ids past Decimal128's 34-digit
		// limit. Same pattern as Phase 2's GetTokenIDs.
		{"$sort": bson.D{
			{Key: "contractAddress", Value: 1},
			{Key: "tokenIDLen", Value: 1},
			{Key: "tokenID", Value: 1},
		}},
		{"$project": bson.M{"tokenIDLen": 0}},
	}

	cursor, err := collection.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var out []models.NFTBalance
	if err := cursor.All(ctx, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = make([]models.NFTBalance, 0)
	}
	return out, nil
}
