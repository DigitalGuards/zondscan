package routes

import (
	"fmt"
	"net/http"
	"quanta-explorer-go/db"
	"strconv"

	"github.com/gin-gonic/gin"
)

// remove ReturnHashToBlockNumber in db.go

func UserRoute(router *gin.Engine) {
	router.GET("/overview", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"marketcap":    db.GetMarketCap(),
			"countwallets": db.GetWalletCount(),
			"circulating":  db.ReturnTotalCirculatingSupply(),
			"volume":       db.ReturnDailyTransactionsVolume(),
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

		// page := c.Request.URL.Query().Get("page")
		// pageStr, err := strconv.Atoi(page)
		// if err != nil {
		// 	fmt.Println(err)
		// }

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

		// // Transactions for the address
		// transactions, err := db.ReturnTransactions(param, pageStr, 15)
		// if err != nil {
		// 	fmt.Println(err)
		// }

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
			// "transactions":            transactions,
		})
	})
	// "latest_transactions": db.ReturnLastSixTransactions(),
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
	// "latest_transactions": db.ReturnLastSixTransactions(),
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

		page, err := strconv.Atoi(pageStr)
		if err != nil {
			fmt.Println(err)
		}

		blocks, err := db.ReturnLatestBlocks(page)
		if err != nil {
			fmt.Println(err)
		}

		countBlocks, err := db.CountBlocksNetwork()
		if err != nil {
			fmt.Println(err)
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
