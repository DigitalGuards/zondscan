package routes

import (
	"backendAPI/cache"
	"backendAPI/db"
	"backendAPI/models"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/mongo"
)

// receiptLog mirrors the subset of qrl_getTransactionReceipt.logs[] the tx
// page actually consumes. address + topics + data + logIndex is enough for
// the frontend's Event Logs panel to decode all token-flavoured signatures
// (Transfer / TransferSingle / TransferBatch / Approval / ApprovalForAll)
// and render a raw fallback for everything else. Everything else on the
// receipt (block hash, tx index, etc.) is already known to the page.
type receiptLog struct {
	Address  string   `json:"address"`
	Topics   []string `json:"topics"`
	Data     string   `json:"data"`
	LogIndex string   `json:"logIndex"`
	Removed  bool     `json:"removed"`
}

// fetchReceiptLogs pulls a tx's receipt over JSON-RPC and returns the
// logs slice. Best-effort: if the node is unreachable or the receipt
// hasn't been indexed yet, returns an empty slice without an error so
// the tx page still renders. Errors are logged at the call site.
func fetchReceiptLogs(ctx context.Context, txHash string) []receiptLog {
	raw, rpcErr, transportErr := db.NodeRPC(ctx, "qrl_getTransactionReceipt", []interface{}{txHash})
	if transportErr != nil {
		log.Printf("receipt fetch %s: %v", txHash, transportErr)
		return nil
	}
	if rpcErr != nil {
		log.Printf("receipt fetch %s: rpc error: %s", txHash, rpcErr.Error())
		return nil
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}
	var receipt struct {
		Logs []receiptLog `json:"logs"`
	}
	if err := json.Unmarshal(raw, &receipt); err != nil {
		log.Printf("receipt decode %s: %v", txHash, err)
		return nil
	}
	return receipt.Logs
}

// fetchTxInput pulls a tx's calldata from the node. The historical block
// docs in MongoDB carry an empty `data` field because the syncer's
// Transaction struct uses the wrong JSON tag (`data`, where the node
// returns `input`), so we resolve it over RPC instead. Best-effort:
// returns "" when the node is unreachable so the page still renders.
func fetchTxInput(ctx context.Context, txHash string) string {
	raw, rpcErr, transportErr := db.NodeRPC(ctx, "qrl_getTransactionByHash", []interface{}{txHash})
	if transportErr != nil {
		log.Printf("tx input fetch %s: %v", txHash, transportErr)
		return ""
	}
	if rpcErr != nil {
		log.Printf("tx input fetch %s: rpc error: %s", txHash, rpcErr.Error())
		return ""
	}
	if len(raw) == 0 || string(raw) == "null" {
		return ""
	}
	var tx struct {
		Input string `json:"input"`
	}
	if err := json.Unmarshal(raw, &tx); err != nil {
		log.Printf("tx input decode %s: %v", txHash, err)
		return ""
	}
	return tx.Input
}

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

	// Contract-verification endpoints (POST /contract/verify, GET
	// /contract/verify/:jobId, GET /contract/compiler-info). The
	// individual handlers return 503 when verification.Default() is nil,
	// so this is safe to wire unconditionally.
	RegisterVerificationRoutes(router)

	// Contract-read endpoint (POST /contract/call). Open eth_call proxy
	// scoped to known contract addresses, with rate-limit + size-cap +
	// gas-cap + per-call timeout.
	RegisterContractCallRoute(router)

	// Contract AI explainer (POST /contract/explain/:address). Hard gate:
	// only verified contracts are analysed. Returns 503 when the
	// Anthropic key isn't configured. Per-IP rate-limited (10/min).
	RegisterContractExplainRoute(router)

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

			// Mined tx whose tombstone was already swept, return 404 in the
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

		// If the row is a tombstone for a mined tx, return 404, let the
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
		// /overview is the homepage hero data, 8 separate Mongo round-trips
		// per request. Cache the assembled payload for 10 s so N concurrent
		// pageviews fan into 1 backend computation.
		v, err := routeCache.GetOrCompute("overview", 10*time.Second, func() (interface{}, error) {
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
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
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
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

		key := fmt.Sprintf("txs:%d:%d", page, limit)
		v, err := routeCache.GetOrCompute(key, 10*time.Second, func() (interface{}, error) {
			txs, err := db.ReturnTransactionsNetwork(page, limit)
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

		// Check if this transaction is a token transfer. A single tx can
		// emit multiple Transfer events (DEX swaps, ERC-1155 TransferBatch
		// fan-out), so we surface them all as an array.
		tokenTransfers, err := db.GetTokenTransfersByTxHash(value)
		if err != nil {
			log.Printf("Error checking for token transfers tx %s: %v", value, err)
		}

		// Receipt logs power the Event Logs panel on the tx page; tx input
		// powers the Input Data card. Best-effort RPC fetches in parallel
		// with a shared 6s budget so the page still renders if the node is
		// briefly unreachable. Tx input has to come from the node because
		// the syncer's Transaction struct uses the wrong JSON tag (`data`
		// where the node returns `input`), leaving the persisted field
		// empty for every historical tx.
		rpcCtx, cancelRPC := context.WithTimeout(c.Request.Context(), 6*time.Second)
		defer cancelRPC()
		var logs []receiptLog
		var txInput string
		var rpcWG sync.WaitGroup
		rpcWG.Add(2)
		go func() {
			defer rpcWG.Done()
			logs = fetchReceiptLogs(rpcCtx, value)
		}()
		go func() {
			defer rpcWG.Done()
			txInput = fetchTxInput(rpcCtx, value)
		}()
		rpcWG.Wait()

		response := gin.H{
			"response":    query,
			"latestBlock": latestBlockNum,
		}
		if len(logs) > 0 {
			response["logs"] = logs
		}
		if txInput != "" && txInput != "0x" {
			response["input"] = txInput
		}

		if contractCreated != nil {
			response["contractCreated"] = gin.H{
				"address":       contractCreated.ContractAddress,
				"isToken":       contractCreated.IsToken,
				"name":          contractCreated.TokenName,
				"symbol":        contractCreated.TokenSymbol,
				"decimals":      contractCreated.TokenDecimals,
				"tokenStandard": contractCreated.TokenStandard,
			}
		}

		if len(tokenTransfers) > 0 {
			rows := make([]gin.H, 0, len(tokenTransfers))
			for _, t := range tokenTransfers {
				rows = append(rows, gin.H{
					"contractAddress": t.ContractAddress,
					"from":            t.From,
					"to":              t.To,
					"amount":          t.Amount,
					"tokenName":       t.TokenName,
					"tokenSymbol":     t.TokenSymbol,
					"tokenDecimals":   t.TokenDecimals,
					"tokenStandard":   t.TokenStandard,
					"tokenID":         t.TokenID,
					"logIndex":        t.LogIndex,
				})
			}
			response["tokenTransfers"] = rows
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
		// Richlist (top wallets by balance) changes slowly, 30 s TTL.
		v, err := routeCache.GetOrCompute("richlist", 30*time.Second, func() (interface{}, error) {
			return gin.H{"richlist": db.ReturnRichlist()}, nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
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
		// syncer's periodic task, safe to cache for longer.
		v, err := routeCache.GetOrCompute("blocksizes", 30*time.Second, func() (interface{}, error) {
			query, err := db.ReturnBlockSizes()
			if err != nil {
				log.Printf("error fetching block sizes: %v", err)
			}
			return gin.H{"response": query}, nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
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
		// Network-wide latest-transactions feed. Hot path on the homepage ,
		// every visitor's 30s refresh hits this. Cache for 5s so the burst
		// fans into one Mongo round-trip.
		v, err := routeCache.GetOrCompute("latest-txs", 5*time.Second, func() (interface{}, error) {
			query, qerr := db.ReturnLatestTransactions()
			if qerr != nil {
				// Return the error so the cache doesn't store an empty
				// payload, the next caller will retry against Mongo
				// instead of being served stale junk for 5s.
				log.Printf("error fetching latest transactions: %v", qerr)
				return nil, qerr
			}
			return gin.H{"response": query}, nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, v)
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

		// Parse standard filter (optional). Validates against the known
		// set to keep the BSON query well-typed and prevent arbitrary
		// strings from being passed as a filter predicate.
		var standardFilter *string
		if standardParam := c.Query("standard"); standardParam != "" {
			switch standardParam {
			case "ERC-20", "ERC-721", "ERC-1155":
				standardFilter = &standardParam
			default:
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "invalid standard; expected one of ERC-20, ERC-721, ERC-1155",
				})
				return
			}
		}

		query, total, err := db.ReturnContracts(page, limit, search, isTokenFilter, standardFilter)
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

	// Get all token balances for a wallet address.
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
	router.GET("/address/:address/tokens", func(c *gin.Context) {
		address := c.Param("address")

		var standardFilter *string
		if s := c.Query("standard"); s != "" {
			switch s {
			case "ERC-20", "ERC-721", "ERC-1155":
				standardFilter = &s
			default:
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "invalid standard; expected one of ERC-20, ERC-721, ERC-1155",
				})
				return
			}
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
	})

	// Per-(contract, tokenID) NFT holdings for a wallet address. Joins both
	// the collection-level contractCode row and the Phase 3b tokenMetadata
	// row so the wallet's "Add NFT" picker can render names + thumbnails +
	// attributes in one call. Optional ?standard= scopes to ERC-721 or
	// ERC-1155; default returns both NFT standards. ERC-20 is rejected as
	// invalid here so the route's intent is explicit; ERC-20 holdings live
	// on the sibling /tokens?standard=ERC-20.
	router.GET("/address/:address/nfts", func(c *gin.Context) {
		address := c.Param("address")

		var standardFilter *string
		if s := c.Query("standard"); s != "" {
			switch s {
			case "ERC-721", "ERC-1155":
				standardFilter = &s
			default:
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "invalid standard; expected ERC-721 or ERC-1155",
				})
				return
			}
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

	// Get token holders with pagination. Phase 2: optional `?tokenID=`
	// query param filters to holders of a specific NFT tokenID. When
	// absent, holders are aggregated across all ids (one row per distinct
	// holder, balance is the cross-id total). ERC-20 behaviour unchanged.
	router.GET("/token/:address/holders", func(c *gin.Context) {
		address := c.Param("address")
		tokenID := c.Query("tokenID")
		page, limit := getPaginationParams(c, 0, 25)

		holders, totalCount, err := db.GetTokenHolders(address, tokenID, page, limit)
		if err != nil {
			log.Printf("Error fetching token holders for %s (tokenID=%q): %v", address, tokenID, err)
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

	// Phase 2: list distinct tokenIDs minted on an NFT contract, with the
	// number of holders for each id. Paginated by `?page=&limit=` like the
	// other token endpoints. Returns an empty list for ERC-20 contracts.
	// Phase 3b: each row also carries `name` + `image` + `description`
	// when off-chain metadata has been fetched.
	router.GET("/token/:address/tokens", func(c *gin.Context) {
		address := c.Param("address")
		page, limit := getPaginationParams(c, 0, 25)

		tokens, totalCount, err := db.GetTokenIDs(address, page, limit)
		if err != nil {
			log.Printf("Error fetching tokenIDs for %s: %v", address, err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Failed to fetch tokenIDs",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"contractAddress": address,
			"tokens":          tokens,
			"totalTokenIDs":   totalCount,
			"page":            page,
			"limit":           limit,
		})
	})

	// Phase 3b: per-token off-chain metadata. Returns the full metadata
	// document for one (contract, tokenID), including OpenSea-style
	// attributes. 404 if no stub exists (e.g. the contract isn't an NFT
	// or that id has never been transferred). 400 if `id` isn't a
	// decimal integer, this also guards against accidental collision
	// with the sibling static-segment routes (/info, /holders, etc),
	// although Gin's router already gives them priority by tree shape.
	router.GET("/token/:address/:id", func(c *gin.Context) {
		address := c.Param("address")
		tokenID := c.Param("id")

		if _, ok := new(big.Int).SetString(tokenID, 10); !ok {
			c.JSON(http.StatusBadRequest, gin.H{"error": "tokenID must be a decimal integer"})
			return
		}

		meta, err := db.GetTokenMetadata(address, tokenID)
		if err != nil {
			log.Printf("Error fetching token metadata for %s/%s: %v", address, tokenID, err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch token metadata"})
			return
		}
		if meta == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Token not found"})
			return
		}
		c.JSON(http.StatusOK, meta)
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

	// ---- Gas / mempool stats ---------------------------------------------
	//
	// /pending-tx-eta/:hash returns a best-effort inclusion ETA for one
	// pending tx, plus the supporting numbers (avg block time, avg gas
	// usage, mempool size, median gasPrice, gas-ahead sum). The math is in
	// db/gas.go; the route just plumbs it together.
	router.GET("/pending-tx-eta/:hash", func(c *gin.Context) {
		hash := c.Param("hash")
		v, err := routeCache.GetOrCompute("eta:"+hash, 5*time.Second, func() (interface{}, error) {
			tx, err := db.GetPendingTransactionByHash(hash)
			if err != nil {
				return nil, err
			}
			if tx == nil || tx.Status != "pending" {
				return nil, nil
			}

			samples, err := db.GetRecentBlockSamples(30)
			if err != nil {
				return nil, err
			}
			blockStats := db.ComputeBlockStats(samples)

			mempool, err := db.GetPendingMempoolSnapshot()
			if err != nil {
				return nil, err
			}
			mp := db.ComputeMempoolStatsFor(mempool, tx.GasPrice)

			// ETA = ceil(gasAhead / avgGasUsed) * avgBlockTime.
			avgGas := blockStats.AvgGasUsed
			if avgGas.Sign() <= 0 {
				avgGas = big.NewInt(1)
			}
			// Integer ceil-division: (a + b - 1) / b.
			num := new(big.Int).Add(mp.GasAhead, new(big.Int).Sub(avgGas, big.NewInt(1)))
			blocksAhead := new(big.Int).Quo(num, avgGas).Int64()
			etaSec := float64(blocksAhead) * blockStats.AvgBlockTimeSec
			// At minimum one block-time of wait, gasAhead = 0 still means we
			// wait for the next block to be produced.
			if etaSec < blockStats.AvgBlockTimeSec {
				etaSec = blockStats.AvgBlockTimeSec
			}

			return gin.H{
				"etaSec":            etaSec,
				"avgBlockTimeSec":   blockStats.AvgBlockTimeSec,
				"avgGasUsedHex":     "0x" + blockStats.AvgGasUsed.Text(16),
				"pendingCount":      mp.Count,
				"gasAheadHex":       "0x" + mp.GasAhead.Text(16),
				"medianGasPriceHex": "0x" + mp.MedianGasPrice.Text(16),
				"yourGasPriceHex":   tx.GasPrice,
			}, nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if v == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "Pending transaction not found"})
			return
		}
		c.JSON(http.StatusOK, v)
	})

	// /gas/summary feeds the top stat cards on the /gas page and the home
	// "Avg Gas Price" tile. Combines the same block + mempool snapshots used
	// by the ETA endpoint.
	router.GET("/gas/summary", func(c *gin.Context) {
		v, err := routeCache.GetOrCompute("gas:summary", 5*time.Second, func() (interface{}, error) {
			samples, err := db.GetRecentBlockSamples(30)
			if err != nil {
				return nil, err
			}
			blockStats := db.ComputeBlockStats(samples)

			mempool, err := db.GetPendingMempoolSnapshot()
			if err != nil {
				return nil, err
			}
			// Pass an empty target so GasAhead is just total mempool gas
			// (callers ignore that field for the summary).
			mp := db.ComputeMempoolStatsFor(mempool, "0x0")
			histogram := db.MempoolGasPriceHistogram(mempool, 12)

			// Headline gas price: median across the last N transfers
			// independent of when they happened. This stays meaningful on a
			// quiet testnet where 30 recent blocks may contain zero txs. The
			// 30-block window median and mempool median are still computed
			// and surfaced for callers that care.
			recentTxPrices, err := db.GetRecentTransactionGasPrices(20, 500)
			if err != nil {
				// Don't fail the whole summary, fall back to the 30-block
				// window median if the wider walk fails.
				recentTxPrices = nil
			}
			recentTxMedian := db.MedianBigHex(recentTxPrices)

			headlineHex := "0x" + recentTxMedian.Text(16)
			if recentTxMedian.Sign() == 0 {
				switch {
				case blockStats.MedianTxGasPrice.Sign() > 0:
					headlineHex = "0x" + blockStats.MedianTxGasPrice.Text(16)
				case mp.MedianGasPrice.Sign() > 0:
					headlineHex = "0x" + mp.MedianGasPrice.Text(16)
				}
			}

			return gin.H{
				"avgGasPriceHex":             headlineHex,
				"recentTxMedianHex":          "0x" + recentTxMedian.Text(16),
				"recentTxSampleSize":         len(recentTxPrices),
				"recentMedianGasPriceHex":    "0x" + blockStats.MedianTxGasPrice.Text(16),
				"mempoolMedianGasPriceHex":   "0x" + mp.MedianGasPrice.Text(16),
				"recentTxCount":              blockStats.TxCount,
				// QRL/USD spot price (last coingecko sample). Used by the
				// frontend to convert gas prices into USD transfer costs.
				"qrlUsdPrice":                db.GetCurrentPrice(),
				"avgGasUsedHex":      "0x" + blockStats.AvgGasUsed.Text(16),
				"avgGasLimitHex":     "0x" + blockStats.AvgGasLimit.Text(16),
				"avgBlockTimeSec":    blockStats.AvgBlockTimeSec,
				"pendingCount":       mp.Count,
				"lastBlockNumberHex": blockStats.LastNumberHex,
				"lastGasUsedHex":     blockStats.LastGasUsedHex,
				"lastGasLimitHex":    blockStats.LastGasLimitHex,
				"gasPriceHistogram":  histogram,
			}, nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, v)
	})

	// /gas/history?range=24h|7d serves the timeseries used by the /gas page
	// charts. Data is sourced from the `gasHistory` collection populated by
	// the syncer's periodic task.
	router.GET("/gas/history", func(c *gin.Context) {
		rng := c.DefaultQuery("range", "24h")
		var sinceSec, bucketSec int64
		switch rng {
		case "7d":
			sinceSec = time.Now().Unix() - 7*24*3600
			bucketSec = 3600 // 1 row per hour
		default:
			rng = "24h"
			sinceSec = time.Now().Unix() - 24*3600
			bucketSec = 0 // raw per-block rows
		}
		key := "gas:history:" + rng
		v, err := routeCache.GetOrCompute(key, 30*time.Second, func() (interface{}, error) {
			rows, err := db.GetGasHistory(sinceSec, bucketSec)
			if err != nil {
				return nil, err
			}
			if rows == nil {
				rows = []db.GasHistoryRow{}
			}
			return gin.H{"range": rng, "data": rows}, nil
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, v)
	})
}
