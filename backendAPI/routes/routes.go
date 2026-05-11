package routes

import (
	"backendAPI/cache"
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

// routeCache absorbs concurrent traffic on read endpoints with a small TTL
// (5–30 s depending on freshness needs). Singleflight inside the cache
// guarantees only one goroutine recomputes a key when it expires, so
// MongoDB never sees a "thundering herd" on cache miss.
var routeCache = cache.New()

func init() {
	routeCache.StartJanitor(time.Minute)
}

// parseHexBlockNumber parses a "0x"-prefixed hex string into a uint64.
func parseHexBlockNumber(hexStr string) (uint64, error) {
	if !strings.HasPrefix(hexStr, "0x") {
		return 0, fmt.Errorf("invalid hex prefix")
	}
	return strconv.ParseUint(hexStr[2:], 16, 64)
}

// getPaginationParams extracts page and limit query parameters with defaults and bounds.
func getPaginationParams(c *gin.Context, defaultPage, defaultLimit int) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", strconv.Itoa(defaultPage)))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", strconv.Itoa(defaultLimit)))
	if page < 1 {
		page = defaultPage
	}
	if limit < 1 {
		limit = defaultLimit
	}
	if limit > 100 {
		limit = 100
	}
	return page, limit
}

func UserRoute(router *gin.Engine) {
	// Health check endpoint for Kubernetes probes
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// Add pending transactions endpoint with pagination
	router.GET("/pending-transactions", func(c *gin.Context) {
		page, limit := getPaginationParams(c, 1, 10)
		key := fmt.Sprintf("pending-tx:%d:%d", page, limit)
		// Frontend polls this every 30 s × every user. 5 s TTL absorbs the
		// herd; mempool freshness only suffers by at most 5 s.
		v, err := routeCache.GetOrCompute(key, 5*time.Second, func() (interface{}, error) {
			result, err := db.GetPendingTransactions(page, limit)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch pending transactions: %w", err)
			}
			if result.Transactions == nil {
				result.Transactions = make([]models.PendingTransaction, 0)
			}
			return result, nil
		})
		if err != nil {
			log.Printf("Error fetching pending transactions: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, v)
	})

	// Add endpoint for fetching a specific pending transaction
	router.GET("/pending-transaction/:hash", func(c *gin.Context) {
		hash := c.Param("hash")
		transaction, err := db.GetPendingTransactionByHash(hash)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		// If not found in pending, check if it's in the transactions collection
		if transaction == nil {
			tx, err := db.GetTransactionByHash(hash)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			// Mined tx whose tombstone was already swept — return 404 in the
			// same shape as the tombstone branch below. The frontend treats
			// any non-200 from this endpoint as "not pending" and fetches
			// /tx/:hash for the mined payload, so returning the mined details
			// here just duplicates work and creates a state-dependent API.
			if tx != nil {
				c.JSON(http.StatusNotFound, gin.H{
					"error":   "Transaction has been mined",
					"status":  "mined",
					"details": "This transaction has been confirmed. Please view it as a confirmed transaction.",
				})
				return
			}

			// Transaction not found in either collection
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Transaction not found",
				"details": "This transaction is no longer in the mempool. It may have been dropped or replaced.",
			})
			return
		}

		// If the row is a tombstone for a mined tx, return 404 — let the
		// frontend fall back to /tx/<hash>. Don't delete here: the tombstone
		// protects against a stale mempool poll re-inserting the row as
		// "pending". CleanupOldPendingTransactions sweeps it after maxAge.
		if transaction.Status == "mined" {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "Transaction has been mined",
				"status":  "mined",
				"details": "This transaction has been confirmed. Please view it as a confirmed transaction.",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{"transaction": transaction})
	})

	router.GET("/overview", func(c *gin.Context) {
		// /overview is the homepage hero data — 8 separate Mongo round-trips
		// per request. Cache the assembled payload for 10 s so N concurrent
		// pageviews fan into 1 backend computation.
		v, _ := routeCache.GetOrCompute("overview", 10*time.Second, func() (interface{}, error) {
			marketCap := db.GetMarketCap()
			currentPrice := db.GetCurrentPrice()
			walletCount := db.GetWalletCount()

			circulating := db.ReturnTotalCirculatingSupply()
			if circulating == "" {
				circulating = "65000000" // default when no data is available
			}

			volume := db.ReturnDailyTransactionsVolume()

			validatorCount, err := db.CountValidators()
			if err != nil {
				validatorCount = 0
			}

			contractCount, err := db.CountContracts()
			if err != nil {
				contractCount = 0
			}

			tradingVolume := db.GetCurrentVolume()

			return gin.H{
				"marketcap":      marketCap,
				"currentPrice":   currentPrice,
				"countwallets":   walletCount,
				"circulating":    circulating,
				"volume":         volume,
				"tradingVolume":  tradingVolume,
				"validatorCount": validatorCount,
				"contractCount":  contractCount,
				"status": gin.H{
					"syncing":         true,
					"dataInitialized": marketCap > 0 || currentPrice > 0 || walletCount > 0 || circulating != "0" || volume > 0,
				},
			}, nil
		})
		c.JSON(http.StatusOK, v)
	})

	// Price history endpoint for wallet apps and charts
	// Supports intervals: 4h, 12h, 24h, 7d, 30d, all
	router.GET("/price-history", func(c *gin.Context) {
		interval := c.DefaultQuery("interval", "24h")

		// Validate interval
		validIntervals := map[string]bool{
			"4h": true, "12h": true, "24h": true,
			"7d": true, "30d": true, "all": true,
		}
		if !validIntervals[interval] {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":           "Invalid interval. Valid options: 4h, 12h, 24h, 7d, 30d, all",
				"requestInterval": interval,
			})
			return
		}

		history, err := db.GetPriceHistory(interval)
		if err != nil {
			log.Printf("Error fetching price history: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to fetch price history: %v", err),
			})
			return
		}

		// Return empty array instead of null
		if history == nil {
			history = make([]models.PriceHistory, 0)
		}

		c.JSON(http.StatusOK, models.PriceHistoryResponse{
			Data:     history,
			Interval: interval,
			Count:    len(history),
		})
	})

	router.POST("/getBalance", func(c *gin.Context) {
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
	})

	router.GET("/txs", func(c *gin.Context) {
		pageStr := c.Query("page")
		page, err := strconv.Atoi(pageStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": fmt.Sprintf("Invalid page number: %v", err),
			})
			return
		}

		key := fmt.Sprintf("txs:%d", page)
		v, err := routeCache.GetOrCompute(key, 10*time.Second, func() (interface{}, error) {
			txs, err := db.ReturnTransactionsNetwork(page)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch transactions: %w", err)
			}
			countTransactions, err := db.CountTransactionsNetwork()
			if err != nil {
				return nil, fmt.Errorf("failed to count transactions: %w", err)
			}
			latestBlockNumber, err := db.GetLatestBlockFromSyncState()
			if err != nil {
				return nil, fmt.Errorf("failed to get latest block: %w", err)
			}
			latestBlockNum, err := parseHexBlockNumber(latestBlockNumber)
			if err != nil {
				return nil, fmt.Errorf("failed to parse block number: %w", err)
			}
			if txs == nil {
				txs = make([]models.TransactionByAddress, 0)
			}
			return gin.H{
				"txs":         txs,
				"total":       countTransactions,
				"latestBlock": latestBlockNum,
			}, nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, v)
	})

	router.GET("/walletdistribution/:query", func(c *gin.Context) {
		value := c.Param("query")
		wallets, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			log.Printf("error parsing wallet distribution query: %v", err)
		}
		query, err := db.ReturnWalletDistribution(wallets)
		if err != nil {
			log.Printf("error fetching wallet distribution: %v", err)
		}
		c.JSON(http.StatusOK, gin.H{"response": query})
	})

	router.GET("/address/aggregate/:query", func(c *gin.Context) {
		param := c.Param("query")
		// db functions normalize the address to canonical q-prefix internally.

		// Pagination for the tx-list aggregations. Defaults match the old
		// implicit page-1 view; cap at 50 (the same cap the DB layer
		// enforces) so a hostile caller can't push limit=10000.
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

			rank, err := db.ReturnRankAddress(param)
			if err != nil {
				log.Printf("error getting rank: %v", err)
			}

			transactionsByAddress, err := db.ReturnAllTransactionsByAddress(param, page, limit)
			if err != nil {
				log.Printf("error getting transactions: %v", err)
			}

			internalTransactionsByAddress, err := db.ReturnAllInternalTransactionsByAddress(param, page, limit)
			if err != nil {
				log.Printf("error getting internal transactions: %v", err)
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
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, v)
	})

	router.GET("/tx/:query", func(c *gin.Context) {
		value := c.Param("query")
		query, err := db.ReturnSingleTransfer(value)
		if err != nil {
			log.Printf("error fetching transfer %s: %v", value, err)
		}

		latestBlockNumber, err := db.GetLatestBlockFromSyncState()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to get latest block: %v", err),
			})
			return
		}

		latestBlockNum, err := parseHexBlockNumber(latestBlockNumber)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to parse block number: %v", err),
			})
			return
		}

		// Check if this transaction created a contract
		contractCreated, err := db.GetContractByCreationTx(value)
		if err != nil {
			log.Printf("Error checking for contract creation tx %s: %v", value, err)
		}

		// Check if this transaction is a token transfer
		tokenTransfer, err := db.GetTokenTransferByTxHash(value)
		if err != nil {
			log.Printf("Error checking for token transfer tx %s: %v", value, err)
		}

		response := gin.H{
			"response":    query,
			"latestBlock": latestBlockNum,
		}

		if contractCreated != nil {
			response["contractCreated"] = gin.H{
				"address":  contractCreated.ContractAddress,
				"isToken":  contractCreated.IsToken,
				"name":     contractCreated.TokenName,
				"symbol":   contractCreated.TokenSymbol,
				"decimals": contractCreated.TokenDecimals,
			}
		}

		if tokenTransfer != nil {
			response["tokenTransfer"] = gin.H{
				"contractAddress": tokenTransfer.ContractAddress,
				"from":            tokenTransfer.From,
				"to":              tokenTransfer.To,
				"amount":          tokenTransfer.Amount,
				"tokenName":       tokenTransfer.TokenName,
				"tokenSymbol":     tokenTransfer.TokenSymbol,
				"tokenDecimals":   tokenTransfer.TokenDecimals,
			}
		}

		c.JSON(http.StatusOK, response)
	})

	router.GET("/latestblock", func(c *gin.Context) {
		// Several frontend pages and external pollers hit this very often;
		// a 5 s cache trims the load by orders of magnitude with no
		// visible staleness (chain block time is much longer).
		v, err := routeCache.GetOrCompute("latestblock", 5*time.Second, func() (interface{}, error) {
			blockNumber, err := db.GetLatestBlockFromSyncState()
			if err != nil {
				return nil, fmt.Errorf("failed to fetch latest block: %w", err)
			}
			num, err := parseHexBlockNumber(blockNumber)
			if err != nil {
				return nil, fmt.Errorf("invalid block number format in sync state: %w", err)
			}
			return gin.H{"blockNumber": num}, nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, v)
	})

	router.GET("/coinbase/:query", func(c *gin.Context) {
		value := c.Param("query")
		query, err := db.ReturnSingleTransfer(value)
		if err != nil {
			log.Printf("error fetching coinbase transfer %s: %v", value, err)
		}
		c.JSON(http.StatusOK, gin.H{"response": query})
	})

	router.GET("/richlist", func(c *gin.Context) {
		// Richlist (top wallets by balance) changes slowly — 30 s TTL.
		v, _ := routeCache.GetOrCompute("richlist", 30*time.Second, func() (interface{}, error) {
			return gin.H{"richlist": db.ReturnRichlist()}, nil
		})
		c.JSON(http.StatusOK, v)
	})

	router.GET("/blocks", func(c *gin.Context) {
		page, limit := getPaginationParams(c, 1, 5)
		key := fmt.Sprintf("blocks:%d:%d", page, limit)
		v, err := routeCache.GetOrCompute(key, 10*time.Second, func() (interface{}, error) {
			blocks, err := db.ReturnLatestBlocks(page, limit)
			if err != nil {
				return nil, fmt.Errorf("failed to fetch blocks: %w", err)
			}
			countBlocks, err := db.CountBlocksNetwork()
			if err != nil {
				return nil, fmt.Errorf("failed to count blocks: %w", err)
			}
			// Limit total pages to 300
			maxPages := int64(300)
			maxBlocks := maxPages * int64(limit)
			if countBlocks > maxBlocks {
				countBlocks = maxBlocks
			}
			return gin.H{"blocks": blocks, "total": countBlocks}, nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, v)
	})

	router.GET("/blocksizes", func(c *gin.Context) {
		// Block-size aggregate is a precomputed collection refreshed by the
		// syncer's periodic task — safe to cache for longer.
		v, _ := routeCache.GetOrCompute("blocksizes", 30*time.Second, func() (interface{}, error) {
			query, err := db.ReturnBlockSizes()
			if err != nil {
				log.Printf("error fetching block sizes: %v", err)
			}
			return gin.H{"response": query}, nil
		})
		c.JSON(http.StatusOK, v)
	})

	// Paginated epochs listing
	router.GET("/epochs", func(c *gin.Context) {
		page, limit := getPaginationParams(c, 1, 15)

		result, err := db.GetEpochs(page, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to fetch epochs: %v", err),
			})
			return
		}

		if result.Epochs == nil {
			result.Epochs = make([]models.EpochListItem, 0)
		}

		c.JSON(http.StatusOK, result)
	})

	router.GET("/validators", func(c *gin.Context) {
		pageToken := c.Query("page_token")
		validatorResponse, err := db.ReturnValidators(pageToken)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to fetch validators: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, validatorResponse)
	})

	// Get epoch detail by ID
	router.GET("/epoch/:id", func(c *gin.Context) {
		epochId := c.Param("id")
		epochDetail, err := db.GetEpochDetail(epochId)
		if err != nil {
			if strings.Contains(err.Error(), "not yet occurred") || strings.Contains(err.Error(), "invalid epoch") {
				c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to fetch epoch detail: %v", err),
			})
			return
		}
		c.JSON(http.StatusOK, epochDetail)
	})

	// Get current epoch information
	router.GET("/epoch", func(c *gin.Context) {
		epochInfo, err := db.GetEpochInfo()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to fetch epoch info: %v", err),
			})
			return
		}
		c.JSON(http.StatusOK, epochInfo)
	})

	// Get validator history for charts
	router.GET("/validators/history", func(c *gin.Context) {
		_, limit := getPaginationParams(c, 1, 100)

		history, err := db.GetValidatorHistory(limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to fetch validator history: %v", err),
			})
			return
		}
		c.JSON(http.StatusOK, history)
	})

	// Get validator statistics
	router.GET("/validators/stats", func(c *gin.Context) {
		stats, err := db.GetValidatorStats()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to fetch validator stats: %v", err),
			})
			return
		}
		c.JSON(http.StatusOK, stats)
	})

	// Get individual validator details
	router.GET("/validator/:id", func(c *gin.Context) {
		id := c.Param("id")
		validator, err := db.GetValidatorByID(id)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": fmt.Sprintf("Validator not found: %v", err),
			})
			return
		}
		c.JSON(http.StatusOK, validator)
	})

	router.GET("/transactions", func(c *gin.Context) {
		query, err := db.ReturnLatestTransactions()
		if err != nil {
			log.Printf("error fetching latest transactions: %v", err)
		}
		c.JSON(http.StatusOK, gin.H{"response": query})
	})

	router.GET("/contracts", func(c *gin.Context) {
		pageInt, limitInt := getPaginationParams(c, 0, 10)
		page := int64(pageInt)
		limit := int64(limitInt)
		search := c.Query("search")

		// Parse isToken filter (optional)
		var isTokenFilter *bool
		if isTokenParam := c.Query("isToken"); isTokenParam != "" {
			isToken := isTokenParam == "true"
			isTokenFilter = &isToken
		}

		query, total, err := db.ReturnContracts(page, limit, search, isTokenFilter)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to fetch contracts: %v", err),
			})
			return
		}

		// Addresses are stored and returned as q-prefix (canonical form).
		// No presentation-layer conversion needed.

		c.JSON(http.StatusOK, gin.H{
			"response": query,
			"total":    total,
		})
	})

	// NOTE: /debug/blocks exposes internal sync state. In production this endpoint
	// MUST be placed behind authentication middleware or removed entirely to prevent
	// information disclosure to unauthenticated callers.
	router.GET("/debug/blocks", func(c *gin.Context) {
		count, err := db.CountBlocksNetwork()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to count blocks: %v", err),
				"step":  "count_blocks",
			})
			return
		}

		latestBlockNumber, err := db.GetLatestBlockFromSyncState()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":       fmt.Sprintf("Failed to get latest block: %v", err),
				"step":        "get_latest",
				"block_count": count,
			})
			return
		}

		latestBlockNum, err := parseHexBlockNumber(latestBlockNumber)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to parse block number: %v", err),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"total_blocks": count,
			"latest_block": latestBlockNum,
		})
	})

	router.GET("/block/:query", func(c *gin.Context) {
		blockStr := c.Param("query")
		var blockNum uint64
		var err error

		if strings.HasPrefix(blockStr, "0x") {
			// Handle hex format by removing 0x prefix
			blockNum, err = strconv.ParseUint(blockStr[2:], 16, 64)
		} else {
			// Handle decimal format
			blockNum, err = strconv.ParseUint(blockStr, 10, 64)
		}

		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "Invalid block number. Please provide a decimal number or hex with 0x prefix: " + err.Error(),
			})
			return
		}

		block, err := db.ReturnSingleBlock(blockNum)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{
				"error": err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"block": block,
		})
	})

	// Add a new endpoint to get limited non-zero transactions for an address
	router.GET("/address/:address/transactions", func(c *gin.Context) {
		address := c.Param("address")
		page, limit := getPaginationParams(c, 1, 5)

		transactions, err := db.ReturnNonZeroTransactions(address, page, limit)
		if err != nil {
			log.Printf("Error fetching non-zero transactions: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to fetch transactions: %v", err),
			})
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
	})

	// Get all token balances for a wallet address
	// This endpoint is designed for wallet integration (e.g., qrlwallet)
	// to auto-discover tokens held by an address on import
	router.GET("/address/:address/tokens", func(c *gin.Context) {
		address := c.Param("address")

		tokens, err := db.GetTokenBalancesByAddress(address)
		if err != nil {
			log.Printf("Error fetching token balances for %s: %v", address, err)
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
	})

	// Get token info (summary stats for a token contract)
	router.GET("/token/:address/info", func(c *gin.Context) {
		address := c.Param("address")

		info, err := db.GetTokenInfo(address)
		if err != nil {
			log.Printf("Error fetching token info for %s: %v", address, err)
			c.JSON(http.StatusNotFound, gin.H{
				"error": "Token not found",
			})
			return
		}

		c.JSON(http.StatusOK, info)
	})

	// Get token holders with pagination
	router.GET("/token/:address/holders", func(c *gin.Context) {
		address := c.Param("address")
		page, limit := getPaginationParams(c, 0, 25)

		holders, totalCount, err := db.GetTokenHolders(address, page, limit)
		if err != nil {
			log.Printf("Error fetching token holders for %s: %v", address, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to fetch token holders",
			})
			return
		}

		c.JSON(http.StatusOK, models.TokenHoldersResponse{
			ContractAddress: address,
			Holders:         holders,
			TotalHolders:    totalCount,
			Page:            page,
			Limit:           limit,
		})
	})

	// Get token transfers with pagination
	router.GET("/token/:address/transfers", func(c *gin.Context) {
		address := c.Param("address")
		page, limit := getPaginationParams(c, 0, 25)

		transfers, totalCount, err := db.GetTokenTransfers(address, page, limit)
		if err != nil {
			log.Printf("Error fetching token transfers for %s: %v", address, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to fetch token transfers",
			})
			return
		}

		c.JSON(http.StatusOK, models.TokenTransfersResponse{
			ContractAddress: address,
			Transfers:       transfers,
			TotalTransfers:  totalCount,
			Page:            page,
			Limit:           limit,
		})
	})
}
