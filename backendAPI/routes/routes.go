package routes

import (
	"backendAPI/cache"
	"backendAPI/db"
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// routeCache absorbs concurrent traffic on read endpoints with a small TTL
// (3-30 s depending on freshness needs). Singleflight inside the cache
// guarantees only one goroutine recomputes a key when it expires, so
// MongoDB never sees a "thundering herd" on cache miss.
//
// ── Cache inventory (audit, dev-loop iter 15) ────────────────────────────
// QRL Zond block time is ~15 s; mempool turnover ~5 s. TTLs below were
// picked against those clocks. Anything tagged (*) carries a known
// staleness trade-off worth being aware of when reasoning about a bug.
//
//	key                       TTL  endpoint                  notes
//	────────────────────────  ───  ────────────────────────  ─────────────
//	contracts:counts          30s  /contracts/counts         one $group aggregation; counts shift block-paced
//	pending-tx:<page>:<lim>    5s  /pending-transactions     mempool turnover ~5s, matches
//	overview                  10s  /overview                 8 mongo round trips fused; values change slowly
//	txs:<page>:<lim>          10s  /transactions             (*) embeds latestBlock; lagged confirmation counts up to 10s
//	addr:<addr>:<page>:<lim>  10s  /address/aggregate/:addr  (*) embeds latestBlock; same trade-off as /transactions
//	latestblock                5s  /latestblock              hot poller endpoint; 5s = 3x ratio under block time
//	richlist                  30s  /richlist                 wallet ranking shifts on timescale of minutes
//	blocks:<page>:<lim>       10s  /blocks                   new block ~15s, list refresh tolerates lag
//	blocksizes                30s  /blocksizes               precomputed time series; updated by syncer hourly
//	latest-txs                 5s  /transactions (legacy)    home-page feed, hot path
//	eta:<hash>                 5s  /pending-tx-eta/:hash     per-tx pending ETA, valid for one block window
//	gas:summary                5s  /gas/summary              live gas snapshot
//	gas:history:<range>       30s  /gas/history              precomputed time series; 30s is fine
//	market:orderbook           3s  /market/orderbook         MEXC snapshot or short failure backoff
//
// Staleness contract:
//   - Endpoints embedding `latestBlock` carry the cache window as their
//     worst-case confirmation lag. The /tx/:hash detail endpoint deliberately
//     skips this cache (see backendAPI README + iter9 history) because a
//     newly-mined tx must show its true confirmation count immediately.
//   - Mempool endpoints (pending + eta + gas) hover at 5s, the smallest
//     value that meaningfully amortises concurrent visitors.
//   - Anything > 30s should be re-justified here, longer TTLs hide real
//     changes that users notice (block height, gas price spikes).
//
// To change a TTL: edit the call site and update the row above so the
// inventory stays accurate. Two minutes of doc-keeping costs less than
// chasing a stale-cache bug.
var routeCache = cache.New()

// stopCacheJanitor terminates the routeCache janitor goroutine. Set by
// UserRoute; kept package-level so a future shutdown hook can call it.
var stopCacheJanitor func()

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

// handleHealth serves GET /health, the health check endpoint for
// Kubernetes probes. Readiness probe: pings MongoDB with a short budget so
// an orchestrator pulls the pod from rotation when the DB is unreachable.
// Returns 503 on ping failure.
func handleHealth(c *gin.Context) {
	pingCtx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	if err := db.PingDatabase(pingCtx); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "error", "error": "database unreachable"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// UserRoute registers every public endpoint. Handler bodies live in the
// per-domain files (tx_routes.go, block_routes.go, address_routes.go,
// token_routes.go, validator_routes.go, gas_routes.go, market_routes.go);
// this function is intentionally just the registration list.
func UserRoute(router *gin.Engine) {
	stopCacheJanitor = routeCache.StartJanitor(time.Minute)

	router.GET("/health", handleHealth)

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

	router.GET("/pending-transactions", handlePendingTransactions)
	router.GET("/pending-transaction/:hash", handlePendingTransaction)
	router.GET("/overview", handleOverview)
	router.GET("/price-history", handlePriceHistory)
	router.GET("/market/orderbook", handleMarketOrderBook)
	router.POST("/getBalance", handleGetBalance)
	router.GET("/txs", handleTxs)
	router.GET("/walletdistribution/:query", handleWalletDistribution)
	router.GET("/address/aggregate/:query", handleAddressAggregate)
	router.GET("/tx/:query", handleTx)
	router.GET("/latestblock", handleLatestBlock)
	router.GET("/coinbase/:query", handleCoinbase)
	router.GET("/richlist", handleRichlist)
	router.GET("/blocks", handleBlocks)
	router.GET("/blocksizes", handleBlockSizes)
	router.GET("/epochs", handleEpochs)
	router.GET("/validators", handleValidators)
	router.GET("/epoch/:id", handleEpochByID)
	router.GET("/epoch", handleEpoch)
	router.GET("/validators/history", handleValidatorsHistory)
	router.GET("/validators/stats", handleValidatorsStats)
	router.GET("/validator/:id", handleValidator)
	router.GET("/transactions", handleTransactions)
	router.GET("/contracts/counts", handleContractsCounts)
	router.GET("/contracts", handleContracts)
	router.GET("/block/:query", handleBlock)
	router.GET("/address/:address/transactions", handleAddressTransactions)
	router.GET("/address/:address/token-transfers", handleAddressTokenTransfers)
	router.GET("/address/:address/tokens", handleAddressTokens)
	router.GET("/address/:address/nfts", handleAddressNFTs)
	router.GET("/token/:address/info", handleTokenInfo)
	router.GET("/token/:address/holders", handleTokenHolders)
	router.GET("/token/:address/tokens", handleTokenIDs)
	router.GET("/token/:address/:id", handleTokenMetadata)
	router.GET("/token/:address/transfers", handleTokenTransfers)
	router.GET("/pending-tx-eta/:hash", handlePendingTxETA)
	router.GET("/gas/summary", handleGasSummary)
	router.GET("/gas/history", handleGasHistory)
}
