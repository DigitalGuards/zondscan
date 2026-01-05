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

func UserRoute(router *gin.Engine) {
	// Add pending transactions endpoint with pagination
	router.GET("/pending-transactions", func(c *gin.Context) {
		// Parse pagination parameters
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

		result, err := db.GetPendingTransactions(page, limit)
		if err != nil {
			log.Printf("Error fetching pending transactions: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to fetch pending transactions: %v", err),
			})
			return
		}

		// Log the result for debugging
		log.Printf("Found %d pending transactions", len(result.Transactions))

		// Return empty array instead of null for transactions
		if result.Transactions == nil {
			result.Transactions = make([]models.PendingTransaction, 0)
		}

		c.JSON(http.StatusOK, result)
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
			// Check if transaction exists in the transactions collection
			tx, err := db.GetTransactionByHash(hash)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				return
			}

			if tx != nil {
				// Transaction is mined - return formatted response
				c.JSON(http.StatusOK, gin.H{
					"transaction": gin.H{
						"hash":        tx.Hash, // Already has 0x prefix
						"status":      "mined",
						"blockNumber": tx.BlockNumber,
						"timestamp":   time.Now().Unix(),
					},
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

		// If transaction is mined, delete it from pending collection and redirect to mined tx
		if transaction.Status == "mined" {
			if err := db.DeleteMinedTransaction(hash); err != nil {
				// Log error but don't fail the request
				log.Printf("Error deleting mined transaction %s: %v\n", hash, err)
			}
			// Don't return the pending transaction - let frontend fetch from /tx endpoint
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
		// Get market cap with default value
		marketCap := db.GetMarketCap()

		// Get current price with default value
		currentPrice := db.GetCurrentPrice()

		// Get wallet count with default value
		walletCount := db.GetWalletCount()

		// Get circulating supply with default value
		circulating := db.ReturnTotalCirculatingSupply()
		if circulating == "" {
			circulating = "65000000" // Default value when no data is available
		}

		// Get daily transaction volume with default value
		volume := db.ReturnDailyTransactionsVolume()

		// Get validator count
		validatorCount, err := db.CountValidators()
		if err != nil {
			validatorCount = 0
		}

		// Get contract count
		contractCount, err := db.CountContracts()
		if err != nil {
			contractCount = 0
		}

		// Return response with default values if data isn't available
		c.JSON(http.StatusOK, gin.H{
			"marketcap":      marketCap,      // Returns 0 if not available
			"currentPrice":   currentPrice,   // Returns 0 if not available
			"countwallets":   walletCount,    // Returns 0 if not available
			"circulating":    circulating,    // Returns "0" if not available
			"volume":         volume,         // Returns 0 if not available
			"validatorCount": validatorCount, // Returns 0 if not available
			"contractCount":  contractCount,  // Returns 0 if not available
			"status": gin.H{
				"syncing":         true, // Indicate that data is still being synced
				"dataInitialized": marketCap > 0 || currentPrice > 0 || walletCount > 0 || circulating != "0" || volume > 0,
			},
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

		txs, err := db.ReturnTransactionsNetwork(page)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to fetch transactions: %v", err),
			})
			return
		}

		// Transaction count for the address
		countTransactions, err := db.CountTransactionsNetwork()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to count transactions: %v", err),
			})
			return
		}

		latestBlockNumber, err := db.GetLatestBlockFromSyncState()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to get latest block: %v", err),
			})
			return
		}

		var latestBlockNum uint64
		if strings.HasPrefix(latestBlockNumber, "0x") {
			latestBlockNum, err = strconv.ParseUint(latestBlockNumber[2:], 16, 64)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("Failed to parse block number: %v", err),
				})
				return
			}
		}

		// Return empty array instead of null if no transactions
		if txs == nil {
			txs = make([]models.TransactionByAddress, 0)
		}

		c.JSON(http.StatusOK, gin.H{
			"txs":         txs,
			"total":       countTransactions,
			"latestBlock": latestBlockNum,
		})
	})

	router.GET("/walletdistribution/:query", func(c *gin.Context) {
		value := c.Param("query")
		wallets, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			fmt.Println(err)
		}
		query, err := db.ReturnWalletDistribution(wallets)
		if err != nil {
			fmt.Println(err)
		}
		c.JSON(http.StatusOK, gin.H{"response": query})
	})

	router.GET("/address/aggregate/:query", func(c *gin.Context) {
		param := c.Param("query")

		// Single Address data
		addressData, err := db.ReturnSingleAddress(param)
		if err != nil && err != mongo.ErrNoDocuments {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Error querying address: %v", err)})
			return
		}

		// Transaction count for the address
		countTransactions, err := db.CountTransactions(param)
		if err != nil {
			fmt.Printf("Error counting transactions: %v\n", err)
		}

		// Rank of the address
		rank, err := db.ReturnRankAddress(param)
		if err != nil {
			fmt.Printf("Error getting rank: %v\n", err)
		}

		// Get all transactions by the address
		transactionsByAddress, err := db.ReturnAllTransactionsByAddress(param)
		if err != nil {
			fmt.Printf("Error getting transactions: %v\n", err)
		}

		// Get all internal transactions by the address
		internalTransactionsByAddress, err := db.ReturnAllInternalTransactionsByAddress(param)
		if err != nil {
			fmt.Printf("Error getting internal transactions: %v\n", err)
		}

		// Get contract code data
		contractCodeData, err := db.ReturnContractCode(param)
		if err != nil {
			fmt.Printf("Error getting contract code: %v\n", err)
		}

		// Get latest block number
		latestBlockNumber, err := db.GetLatestBlockFromSyncState()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to get latest block: %v", err),
			})
			return
		}

		var latestBlockNum uint64
		if strings.HasPrefix(latestBlockNumber, "0x") {
			latestBlockNum, err = strconv.ParseUint(latestBlockNumber[2:], 16, 64)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("Failed to parse block number: %v", err),
				})
				return
			}
		}

		// Response aggregation
		c.JSON(http.StatusOK, gin.H{
			"address":                          addressData,
			"transactions_count":               countTransactions,
			"rank":                             rank,
			"transactions_by_address":          transactionsByAddress,
			"internal_transactions_by_address": internalTransactionsByAddress,
			"contract_code":                    contractCodeData,
			"latestBlock":                      latestBlockNum,
		})
	})

	router.GET("/tx/:query", func(c *gin.Context) {
		value := c.Param("query")
		query, err := db.ReturnSingleTransfer(value)
		if err != nil {
			fmt.Println(err)
		}

		latestBlockNumber, err := db.GetLatestBlockFromSyncState()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to get latest block: %v", err),
			})
			return
		}

		var latestBlockNum uint64
		if strings.HasPrefix(latestBlockNumber, "0x") {
			latestBlockNum, err = strconv.ParseUint(latestBlockNumber[2:], 16, 64)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("Failed to parse block number: %v", err),
				})
				return
			}
		}

		// Check if this transaction created a contract
		contractCreated, err := db.GetContractByCreationTx(value)
		if err != nil {
			log.Printf("Error checking for contract creation tx %s: %v", value, err)
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

		c.JSON(http.StatusOK, response)
	})

	router.GET("/latestblock", func(c *gin.Context) {
		blockNumber, err := db.GetLatestBlockFromSyncState()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": fmt.Sprintf("Failed to fetch latest block: %v", err),
			})
			return
		}

		// Convert hex to decimal
		if strings.HasPrefix(blockNumber, "0x") {
			num, err := strconv.ParseUint(blockNumber[2:], 16, 64)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("Failed to parse block number: %v", err),
				})
				return
			}
			c.JSON(http.StatusOK, gin.H{
				"blockNumber": num,
			})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error": "Invalid block number format in sync state",
			})
		}
	})

	router.GET("/coinbase/:query", func(c *gin.Context) {
		value := c.Param("query")
		query, err := db.ReturnSingleTransfer(value)
		if err != nil {
			fmt.Println(err)
		}
		c.JSON(http.StatusOK, gin.H{"response": query})
	})

	router.GET("/richlist", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"richlist": db.ReturnRichlist()})
	})

	router.GET("/blocks", func(c *gin.Context) {
		pageStr := c.Query("page")
		limitStr := c.Query("limit")

		page, err := strconv.Atoi(pageStr)
		if err != nil {
			page = 1
		}

		limit, err := strconv.Atoi(limitStr)
		if err != nil || limit <= 0 {
			limit = 5 // Default to 5 blocks per page
		}

		blocks, err := db.ReturnLatestBlocks(page, limit)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch blocks"})
			return
		}

		countBlocks, err := db.CountBlocksNetwork()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to count blocks"})
			return
		}

		// Limit total pages to 300
		maxPages := int64(300)
		maxBlocks := maxPages * int64(limit)
		if countBlocks > maxBlocks {
			countBlocks = maxBlocks
		}

		c.JSON(http.StatusOK, gin.H{
			"blocks": blocks,
			"total":  countBlocks,
		})
	})

	router.GET("/blocksizes", func(c *gin.Context) {
		query, err := db.ReturnBlockSizes()
		if err != nil {
			fmt.Println(err)
		}
		c.JSON(http.StatusOK, gin.H{"response": query})
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
		limitStr := c.DefaultQuery("limit", "100")
		limit := 100
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}

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
			fmt.Println(err)
		}
		c.JSON(http.StatusOK, gin.H{"response": query})
	})

	router.GET("/contracts", func(c *gin.Context) {
		// Parse pagination parameters
		page, _ := strconv.ParseInt(c.DefaultQuery("page", "0"), 10, 64)
		limit, _ := strconv.ParseInt(c.DefaultQuery("limit", "10"), 10, 64)
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
		c.JSON(http.StatusOK, gin.H{
			"response": query,
			"total":    total,
		})
	})

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

		var latestBlockNum uint64
		if strings.HasPrefix(latestBlockNumber, "0x") {
			latestBlockNum, err = strconv.ParseUint(latestBlockNumber[2:], 16, 64)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"error": fmt.Sprintf("Failed to parse block number: %v", err),
				})
				return
			}
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
		page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5")) // Default to 5

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
		page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "25"))

		if limit > 100 {
			limit = 100
		}

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
		page, _ := strconv.Atoi(c.DefaultQuery("page", "0"))
		limit, _ := strconv.Atoi(c.DefaultQuery("limit", "25"))

		if limit > 100 {
			limit = 100
		}

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
