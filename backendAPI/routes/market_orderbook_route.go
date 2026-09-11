package routes

import (
	"backendAPI/cache"
	"backendAPI/marketdata"
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	marketOrderBookCacheKey = "market:orderbook"
	marketOrderBookCacheTTL = 3 * time.Second
	marketRequestTimeout    = 5 * time.Second
)

type orderBookFetcher interface {
	FetchOrderBook(context.Context) (marketdata.OrderBookSnapshot, error)
}

type marketOrderBookCacheResult struct {
	snapshot marketdata.OrderBookSnapshot
	fetchErr error
}

var defaultOrderBookFetcher = newDefaultOrderBookFetcher()

func newDefaultOrderBookFetcher() orderBookFetcher {
	client, err := marketdata.NewMEXCClient(marketdata.MEXCAPIBaseURL, nil)
	if err != nil {
		panic("construct fixed MEXC market-data client: " + err.Error())
	}
	return client
}

// handleMarketOrderBook serves GET /market/orderbook for the fixed QRLUSDT
// spot market. No request parameter can change the venue, symbol, or upstream.
func handleMarketOrderBook(c *gin.Context) {
	newMarketOrderBookHandler(defaultOrderBookFetcher, routeCache)(c)
}

func newMarketOrderBookHandler(fetcher orderBookFetcher, responseCache *cache.TTLCache) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, err := responseCache.GetOrComputeContext(
			c.Request.Context(),
			marketOrderBookCacheKey,
			marketOrderBookCacheTTL,
			func() (interface{}, error) {
				fillCtx, cancel := context.WithTimeout(context.Background(), marketRequestTimeout)
				defer cancel()
				snapshot, fetchErr := fetcher.FetchOrderBook(fillCtx)
				if fetchErr != nil {
					log.Printf("MEXC QRLUSDT market-data fetch failed: %v", fetchErr)
				}
				// Store failures as values for the same short TTL. This prevents a
				// fast MEXC 429 or 5xx response from triggering a new three-request
				// upstream burst for every sequential visitor.
				return marketOrderBookCacheResult{snapshot: snapshot, fetchErr: fetchErr}, nil
			},
		)
		if err != nil {
			if c.Request.Context().Err() != nil {
				return
			}
			log.Printf("market order-book cache failed: %v", err)
			respondInternal(c)
			return
		}
		result, ok := value.(marketOrderBookCacheResult)
		if !ok {
			log.Printf("market order-book cache returned unexpected type %T", value)
			respondInternal(c)
			return
		}
		if result.fetchErr != nil {
			c.Header("Retry-After", "3")
			c.JSON(http.StatusBadGateway, gin.H{"error": "market data temporarily unavailable"})
			return
		}
		c.JSON(http.StatusOK, result.snapshot)
	}
}
