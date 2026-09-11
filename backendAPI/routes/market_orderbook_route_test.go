package routes

import (
	"backendAPI/cache"
	"backendAPI/marketdata"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

type stubOrderBookFetcher struct {
	mu       sync.Mutex
	calls    int
	snapshot marketdata.OrderBookSnapshot
	err      error
}

func (s *stubOrderBookFetcher) FetchOrderBook(context.Context) (marketdata.OrderBookSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	return s.snapshot, s.err
}

func (s *stubOrderBookFetcher) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func TestMarketOrderBookRouteReturnsAndCachesSnapshot(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fetchedAt := time.Date(2026, time.August, 9, 12, 0, 0, 0, time.UTC)
	quoteVolume := "75"
	fetcher := &stubOrderBookFetcher{snapshot: marketdata.OrderBookSnapshot{
		Venue:        marketdata.MEXCVenue,
		Symbol:       marketdata.MEXCSymbol,
		FetchedAt:    fetchedAt,
		LastUpdateID: 42,
		Bids:         []marketdata.PriceLevel{{Price: "0.75", Quantity: "10"}},
		Asks:         []marketdata.PriceLevel{{Price: "0.76", Quantity: "12"}},
		RecentTrades: []marketdata.Trade{{
			ID:            "trade-1",
			Price:         "0.755",
			Quantity:      "1",
			QuoteQuantity: "0.755",
			Time:          1786290460978,
			AggressorSide: "buy",
		}},
		Ticker: marketdata.Ticker{
			High:          "0.8",
			Low:           "0.7",
			Last:          "0.755",
			Change:        "0.005",
			ChangePercent: "0.0066",
			BaseVolume:    "100",
			QuoteVolume:   &quoteVolume,
		},
	}}
	router := gin.New()
	router.GET("/market/orderbook", newMarketOrderBookHandler(fetcher, cache.New()))

	for i := 0; i < 2; i++ {
		request := httptest.NewRequest(http.MethodGet, "/market/orderbook?symbol=BTCUSDT&url=http://example.invalid", nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("request %d status = %d, want 200; body = %s", i+1, response.Code, response.Body.String())
		}
		var got marketdata.OrderBookSnapshot
		if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
			t.Fatalf("request %d response JSON error = %v", i+1, err)
		}
		if got.Symbol != marketdata.MEXCSymbol || got.Venue != marketdata.MEXCVenue || got.LastUpdateID != 42 {
			t.Errorf("request %d response = %#v, want fixed MEXC QRLUSDT snapshot", i+1, got)
		}
	}
	if got := fetcher.callCount(); got != 1 {
		t.Fatalf("fetch calls = %d, want 1 within cache TTL", got)
	}
}

func TestMarketOrderBookRouteMapsUpstreamFailureToBadGateway(t *testing.T) {
	gin.SetMode(gin.TestMode)
	fetcher := &stubOrderBookFetcher{err: errors.New("upstream unavailable")}
	router := gin.New()
	router.GET("/market/orderbook", newMarketOrderBookHandler(fetcher, cache.New()))

	for i := 0; i < 2; i++ {
		request := httptest.NewRequest(http.MethodGet, "/market/orderbook", nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if response.Code != http.StatusBadGateway {
			t.Fatalf("request %d status = %d, want 502; body = %s", i+1, response.Code, response.Body.String())
		}
		if got, want := response.Body.String(), `{"error":"market data temporarily unavailable"}`; got != want {
			t.Fatalf("request %d body = %q, want %q", i+1, got, want)
		}
		if got := response.Header().Get("Retry-After"); got != "3" {
			t.Fatalf("request %d Retry-After = %q, want 3", i+1, got)
		}
	}
	if got := fetcher.callCount(); got != 1 {
		t.Fatalf("fetch calls = %d, want 1 during failure backoff", got)
	}
}
