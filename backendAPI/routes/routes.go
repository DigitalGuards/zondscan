package routes

import (
	"backendAPI/db"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

func UserRoute(router *gin.Engine) {
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
