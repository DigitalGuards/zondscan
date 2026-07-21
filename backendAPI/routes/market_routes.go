package routes

import (
	"backendAPI/db"
	"backendAPI/models"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// handleOverview serves GET /overview.
func handleOverview(c *gin.Context) {
	// /overview is the homepage hero data, 8 separate Mongo round-trips
	// per request. Cache the assembled payload for 10 s so N concurrent
	// pageviews fan into 1 backend computation.
	v, err := routeCache.GetOrCompute("overview", 10*time.Second, func() (interface{}, error) {
		// GetMarketCap / GetCurrentPrice / GetCurrentVolume each issue their
		// own FindOne against the single coingecko document. Fetch the
		// snapshot once and reuse it so /overview makes one read instead of
		// three for the same row.
		market := db.GetMarketSnapshot()
		marketCap := market.MarketCapUSD
		currentPrice := market.PriceUSD
		walletCount := db.GetWalletCount()

		// "0" is the bootstrap seed value written before the syncer's first
		// supply pass, treat it like a missing doc so the UI never shows 0
		// circulating during initial sync. The raw value still feeds
		// dataInitialized below, which must not count the fallback.
		rawCirculating := db.ReturnTotalCirculatingSupply()
		circulating := rawCirculating
		if circulating == "" || circulating == "0" {
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

		tradingVolume := market.VolumeUSD

		priceChange24h := db.GetPriceChange24h(currentPrice)

		return gin.H{
			"marketcap":      marketCap,
			"currentPrice":   currentPrice,
			"priceChange24h": priceChange24h,
			"countwallets":   walletCount,
			"circulating":    circulating,
			"volume":         volume,
			"tradingVolume":  tradingVolume,
			"validatorCount": validatorCount,
			"contractCount":  contractCount,
			"status": gin.H{
				"syncing":         true,
				"dataInitialized": marketCap > 0 || currentPrice > 0 || walletCount > 0 || (rawCirculating != "" && rawCirculating != "0") || volume > 0,
			},
		}, nil
	})
	if err != nil {
		log.Printf("error building overview: %v", err)
		respondInternal(c)
		return
	}
	c.JSON(http.StatusOK, v)
}

// handlePriceHistory serves GET /price-history, the price history for
// wallet apps and charts. Supports intervals: 4h, 12h, 24h, 7d, 30d, all.
func handlePriceHistory(c *gin.Context) {
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
		respondInternal(c)
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
}
