package routes

import (
	"backendAPI/db"
	"backendAPI/models"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

// handleGetBalance serves POST /getBalance.
func handleGetBalance(c *gin.Context) {
	address := c.PostForm("address")

	balance, message := db.GetBalance(address)
	if message == "" {
		c.JSON(http.StatusOK, gin.H{
			"balance": balance,
		})
	} else {
		c.JSON(http.StatusOK, gin.H{
			"balance": message,
		})
	}
}

// handleWalletDistribution serves GET /walletdistribution/:query.
func handleWalletDistribution(c *gin.Context) {
	value := c.Param("query")
	wallets, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		log.Printf("error parsing wallet distribution query: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid count: %v", err)})
		return
	}
	query, err := db.ReturnWalletDistribution(wallets)
	if err != nil {
		log.Printf("error fetching wallet distribution: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch wallet distribution"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"response": query})
}

// handleAddressAggregate serves GET /address/aggregate/:query, the
// address detail aggregation.
func handleAddressAggregate(c *gin.Context) {
	param, ok := requireAddressParam(c, "query")
	if !ok {
		return
	}
	// db functions normalize the address to canonical q-prefix internally.

	// Pagination for the tx-list aggregations. Defaults match the old
	// implicit page-1 view; getPaginationParams caps limit at 100 so a
	// hostile caller can't push limit=10000.
	page, limit := getPaginationParams(c, 1, 10)

	key := fmt.Sprintf("addr:%s:%d:%d", strings.ToLower(param), page, limit)
	v, err := routeCache.GetOrCompute(key, 10*time.Second, func() (interface{}, error) {
		addressData, err := db.ReturnSingleAddress(param)
		if err != nil && err != mongo.ErrNoDocuments {
			return nil, fmt.Errorf("error querying address: %w", err)
		}

		countTransactions, err := db.CountTransactions(param)
		if err != nil {
			log.Printf("error counting transactions: %v", err)
		}

		countInternalTransactions, err := db.CountInternalTransactionsByAddress(param)
		if err != nil {
			log.Printf("error counting internal transactions: %v", err)
		}

		// True activity range across the whole history; the paginated
		// tx list only covers one page, so deriving first/last seen
		// from it is wrong for any address with more than one page.
		firstSeen, lastSeen, err := db.ReturnAddressActivityRange(param)
		if err != nil {
			log.Printf("error fetching activity range: %v", err)
		}

		rank, err := db.ReturnRankAddress(param)
		if err != nil {
			log.Printf("error getting rank: %v", err)
		}

		transactionsByAddress, err := db.ReturnAllTransactionsByAddress(param, page, limit)
		if err != nil {
			log.Printf("error getting transactions: %v", err)
		}

		internalTxs, err := db.ReturnAllInternalTransactionsByAddress(param, page, limit)
		if err != nil {
			log.Printf("error getting internal transactions: %v", err)
		}
		// Map the BSON-string internal-tx rows onto the capitalized JSON
		// keys the frontend's InternalTransaction type consumes. The
		// previous code returned models.TraceResult ([]byte fields), which
		// the driver silently dropped to null because the syncer persists
		// these as strings. Sourcing from models.InternalTx keeps the data
		// non-null while preserving the existing response field names.
		internalTransactionsByAddress := make([]gin.H, 0, len(internalTxs))
		for _, t := range internalTxs {
			internalTransactionsByAddress = append(internalTransactionsByAddress, gin.H{
				"Type":                      t.Type,
				"CallType":                  t.CallType,
				"Hash":                      t.Hash,
				"From":                      t.From,
				"To":                        t.To,
				"Input":                     t.Input,
				"Output":                    t.Output,
				"Value":                     t.Value,
				"Gas":                       t.Gas,
				"GasUsed":                   t.GasUsed,
				"AddressFunctionIdentifier": t.AddressFunctionIdentifier,
				"AmountFunctionIdentifier":  t.AmountFunctionIdentifier,
				"BlockTimestamp":            t.BlockTimestamp,
				"TraceAddress":              t.TraceAddress,
			})
		}

		contractCodeData, err := db.ReturnContractCode(param)
		if err != nil {
			log.Printf("error getting contract code: %v", err)
		}

		latestBlockNumber, err := db.GetLatestBlockFromSyncState()
		if err != nil {
			return nil, fmt.Errorf("failed to get latest block: %w", err)
		}
		latestBlockNum, err := parseHexBlockNumber(latestBlockNumber)
		if err != nil {
			return nil, fmt.Errorf("failed to parse block number: %w", err)
		}

		return gin.H{
			"address":                          addressData,
			"transactions_count":               countTransactions,
			"internal_transactions_count":      countInternalTransactions,
			"first_seen":                       firstSeen,
			"last_seen":                        lastSeen,
			"rank":                             rank,
			"transactions_by_address":          transactionsByAddress,
			"internal_transactions_by_address": internalTransactionsByAddress,
			"contract_code":                    contractCodeData,
			"latestBlock":                      latestBlockNum,
			"page":                             page,
			"limit":                            limit,
		}, nil
	})
	if err != nil {
		log.Printf("error aggregating address %s: %v", param, err)
		respondInternal(c)
		return
	}
	c.JSON(http.StatusOK, v)
}

// handleRichlist serves GET /richlist.
func handleRichlist(c *gin.Context) {
	// Richlist (top wallets by balance) changes slowly, 30 s TTL.
	v, err := routeCache.GetOrCompute("richlist", 30*time.Second, func() (interface{}, error) {
		richlist, err := db.ReturnRichlist()
		if err != nil {
			return nil, err
		}
		return gin.H{"richlist": richlist}, nil
	})
	if err != nil {
		log.Printf("error fetching richlist: %v", err)
		respondInternal(c)
		return
	}
	c.JSON(http.StatusOK, v)
}

// handleAddressTransactions serves GET /address/:address/transactions,
// the limited non-zero transactions list for an address.
func handleAddressTransactions(c *gin.Context) {
	address, ok := requireAddressParam(c, "address")
	if !ok {
		return
	}
	page, limit := getPaginationParams(c, 1, 5)

	transactions, err := db.ReturnNonZeroTransactions(address, page, limit)
	if err != nil {
		log.Printf("Error fetching non-zero transactions: %v", err)
		respondInternal(c)
		return
	}

	// Count total non-zero transactions for this address (for pagination info)
	total, err := db.CountTransactions(address)
	if err != nil {
		log.Printf("Error counting transactions: %v", err)
		// Continue anyway, just won't have total count
	}

	// Return empty array instead of null if no transactions
	if transactions == nil {
		transactions = make([]models.TransactionByAddress, 0)
	}

	c.JSON(http.StatusOK, gin.H{
		"transactions": transactions,
		"total":        total,
		"page":         page,
		"limit":        limit,
	})
}

// handleAddressTokenTransfers serves GET /address/:address/token-transfers.
//
// Token / NFT transfer events touching this wallet (sender OR recipient),
// across every contract. Powers the "Token Transfers" tab on the address
// page, which previously had no data source: /address/aggregate only
// returns native-value transactions, and /token/:addr/transfers is keyed
// by contract not by holder. Paginated like /address/:addr/transactions.
func handleAddressTokenTransfers(c *gin.Context) {
	address, ok := requireAddressParam(c, "address")
	if !ok {
		return
	}
	// Match the address-page Transactions table's 5-rows-per-page default
	// (paginated server-side, page-1-indexed). Cap matches the DB layer.
	page, limit := getPaginationParams(c, 1, 5)
	// db helper is 0-indexed; getPaginationParams returns the 1-indexed
	// form so the query-string contract stays consistent with siblings.
	transfers, total, err := db.GetTokenTransfersByAddress(address, page-1, limit)
	if err != nil {
		log.Printf("Error fetching token transfers for %s: %v", address, err)
		respondInternal(c)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"address":   address,
		"transfers": transfers,
		"total":     total,
		"page":      page,
		"limit":     limit,
	})
}

// handleAddressTokens serves GET /address/:address/tokens, all token
// balances for a wallet address.
//
// Default behaviour returns every standard the explorer indexes for
// this account (ERC-20 + ERC-721 + ERC-1155 per-id rows), useful for
// wallet auto-discovery and unchanged from the original contract so
// existing third-party callers don't break.
//
// Optional `?standard=` (ERC-20|ERC-721|ERC-1155) scopes the response
// to one token standard. The wallet's "Tokens" card uses this with
// `standard=ERC-20` so its ERC-20-aware renderer (which divides by
// `decimals`) doesn't accidentally render NFT rows as zero-balance
// fungibles.
func handleAddressTokens(c *gin.Context) {
	address, ok := requireAddressParam(c, "address")
	if !ok {
		return
	}

	standard, ok := parseStandardFilter(c, "ERC-20", "ERC-721", "ERC-1155")
	if !ok {
		return
	}
	var standardFilter *string
	if standard != "" {
		standardFilter = &standard
	}

	tokens, err := db.GetTokenBalancesByAddress(address, standardFilter)
	if err != nil {
		log.Printf("Error fetching token balances for %s (standard=%v): %v", address, standardFilter, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch token balances",
		})
		return
	}

	c.JSON(http.StatusOK, models.TokenBalancesResponse{
		Address: address,
		Tokens:  tokens,
		Count:   len(tokens),
	})
}

// handleAddressNFTs serves GET /address/:address/nfts, the per-(contract,
// tokenID) NFT holdings for a wallet address. Joins both the
// collection-level contractCode row and the Phase 3b tokenMetadata row so
// the wallet's "Add NFT" picker can render names + thumbnails +
// attributes in one call. Optional ?standard= scopes to ERC-721 or
// ERC-1155; default returns both NFT standards. ERC-20 is rejected as
// invalid here so the route's intent is explicit; ERC-20 holdings live
// on the sibling /tokens?standard=ERC-20.
func handleAddressNFTs(c *gin.Context) {
	address, ok := requireAddressParam(c, "address")
	if !ok {
		return
	}

	standard, ok := parseStandardFilter(c, "ERC-721", "ERC-1155")
	if !ok {
		return
	}
	var standardFilter *string
	if standard != "" {
		standardFilter = &standard
	}

	nfts, err := db.GetNFTBalancesByAddress(address, standardFilter)
	if err != nil {
		log.Printf("Error fetching NFT balances for %s (standard=%v): %v", address, standardFilter, err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "Failed to fetch NFT balances",
		})
		return
	}

	c.JSON(http.StatusOK, models.NFTBalancesResponse{
		Address: address,
		NFTs:    nfts,
		Count:   len(nfts),
	})
}
