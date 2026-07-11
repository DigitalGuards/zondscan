package routes

// Gas / mempool stats. The sibling /pending-tx-eta/:hash route (in
// tx_routes.go) shares the same block + mempool snapshot helpers from
// db/gas.go.

import (
	"backendAPI/db"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// handleGasSummary serves GET /gas/summary, which feeds the top stat cards
// on the /gas page and the home "Avg Gas Price" tile. Combines the same
// block + mempool snapshots used by the ETA endpoint.
func handleGasSummary(c *gin.Context) {
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
			"avgGasPriceHex":           headlineHex,
			"recentTxMedianHex":        "0x" + recentTxMedian.Text(16),
			"recentTxSampleSize":       len(recentTxPrices),
			"recentMedianGasPriceHex":  "0x" + blockStats.MedianTxGasPrice.Text(16),
			"mempoolMedianGasPriceHex": "0x" + mp.MedianGasPrice.Text(16),
			"recentTxCount":            blockStats.TxCount,
			// QRL/USD spot price (last coingecko sample). Used by the
			// frontend to convert gas prices into USD transfer costs.
			"qrlUsdPrice":        db.GetCurrentPrice(),
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
		log.Printf("error computing gas summary: %v", err)
		respondInternal(c)
		return
	}
	c.JSON(http.StatusOK, v)
}

// handleGasHistory serves GET /gas/history?range=24h|7d, the timeseries
// used by the /gas page charts. Data is sourced from the `gasHistory`
// collection populated by the syncer's periodic task.
func handleGasHistory(c *gin.Context) {
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
		log.Printf("error fetching gas history: %v", err)
		respondInternal(c)
		return
	}
	c.JSON(http.StatusOK, v)
}
