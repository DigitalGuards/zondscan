package routes

import (
	"backendAPI/db"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func UserRoute(router *gin.Engine) {
	router.GET("/overview", func(c *gin.Context) {
		// Get market cap with default value
		marketCap := db.GetMarketCap()

		// Get wallet count with default value
		walletCount := db.GetWalletCount()

		// Get circulating supply with default value
		circulating := db.ReturnTotalCirculatingSupply()
		if circulating == "" {
			circulating = "0" // Default value when no data is available
		}

		// Get daily transaction volume with default value
		volume := db.ReturnDailyTransactionsVolume()

		// Return response with default values if data isn't available
		c.JSON(http.StatusOK, gin.H{
			"marketcap":    marketCap,   // Returns 0 if not available
			"countwallets": walletCount, // Returns 0 if not available
			"circulating":  circulating, // Returns "0" if not available
			"volume":       volume,      // Returns 0 if not available
			"status": gin.H{
				"syncing":         true, // Indicate that data is still being synced
				"dataInitialized": marketCap > 0 || walletCount > 0 || circulating != "0" || volume > 0,
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
			fmt.Println(err)
		}

		txs, err := db.ReturnTransactionsNetwork(page)
		if err != nil {
			fmt.Println(err)
		}

		// Transaction count for the address
		countTransactions, err := db.CountTransactionsNetwork()
		if err != nil {
			fmt.Println(err)
		}

		c.JSON(http.StatusOK, gin.H{
			"txs":   txs,
			"total": countTransactions,
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
		if err != nil {
			fmt.Println(err)
		}

		// Transaction count for the address
		countTransactions, err := db.CountTransactions(param)
		if err != nil {
			fmt.Println(err)
		}

		// Rank of the address
		rank, err := db.ReturnRankAddress(param)
		if err != nil {
			fmt.Println(err)
		}

		// All transactions by the address
		TransactionsByAddress, err := db.ReturnAllTransactionsByAddress(param)
		if err != nil {
			fmt.Println(err)
		}

		// All transactions by the address
		InternalTransactionsByAddress, err := db.ReturnAllInternalTransactionsByAddress(param)
		if err != nil {
			fmt.Println(err)
		}

		// Contract code (if applicable)
		contractCodeData, err := db.ReturnContractCode(param)
		if err != nil {
			fmt.Println(err)
		}

		// Response aggregation
		c.JSON(http.StatusOK, gin.H{
			"address":                          addressData,
			"transactions_count":               countTransactions,
			"rank":                             rank,
			"transactions_by_address":          TransactionsByAddress,
			"internal_transactions_by_address": InternalTransactionsByAddress,
			"contract_code":                    contractCodeData,
		})
	})

	router.GET("/tx/:query", func(c *gin.Context) {
		value := c.Param("query")
		query, err := db.ReturnSingleTransfer(value)
		if err != nil {
			fmt.Println(err)
		}

		latestBlock, err := db.ReturnLatestBlock()
		if err != nil {
			fmt.Println(err)
		}
		c.JSON(http.StatusOK, gin.H{
			"response":    query,
			"latestBlock": latestBlock,
		})
	})

	router.GET("/latestblock", func(c *gin.Context) {
		latestBlock, err := db.ReturnLatestBlock()
		if err != nil {
			fmt.Println(err)
		}
		c.JSON(http.StatusOK, gin.H{
			"response": latestBlock,
		})
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
		query, err := db.ReturnValidators()
		if err != nil {
			fmt.Println(err)
		}
		c.JSON(http.StatusOK, gin.H{"response": query})
	})

	router.GET("/transactions", func(c *gin.Context) {
		query, err := db.ReturnLatestTransactions()
		if err != nil {
			fmt.Println(err)
		}
		c.JSON(http.StatusOK, gin.H{"response": query})
	})

	router.GET("/contracts", func(c *gin.Context) {
		query, err := db.ReturnContracts()
		if err != nil {
			fmt.Println(err)
		}
		c.JSON(http.StatusOK, gin.H{"response": query})
	})

	router.GET("/block/:query", func(c *gin.Context) {
		value := c.Param("query")
		intValue, err := strconv.ParseUint(value, 10, 64)
		if err != nil {
			fmt.Println(err)
		}
		query, err := db.ReturnSingleBlock(intValue)
		if err != nil {
			fmt.Println(err)
		}
		c.JSON(http.StatusOK, gin.H{"response": query})
	})
}
