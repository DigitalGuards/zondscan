package routes

import (
	"backendAPI/cache"
	"backendAPI/marketdata"
	"backendAPI/models"
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

type stubFundFlowVenue struct{ id string }

func (s stubFundFlowVenue) ID() string         { return s.id }
func (s stubFundFlowVenue) Name() string       { return "Stub " + s.id }
func (s stubFundFlowVenue) Symbol() string     { return "QRLUSDT" }
func (s stubFundFlowVenue) QuoteAsset() string { return "USDT" }
func (s stubFundFlowVenue) SizeThresholds() marketdata.SizeThresholds {
	return marketdata.SizeThresholds{Medium: 10, Large: 100}
}
func (s stubFundFlowVenue) FetchOrderBook(context.Context) (marketdata.OrderBookSnapshot, error) {
	return marketdata.OrderBookSnapshot{}, nil
}
func (s stubFundFlowVenue) FetchTrades(context.Context) ([]marketdata.Trade, error) {
	return nil, nil
}

type stubFundFlowStore struct {
	mu          sync.Mutex
	calls       int
	venuesAsked []string
	windowSpans []time.Duration
	steps       []time.Duration
	buckets     []models.MarketFlowBucket
	series      []models.MarketFlowPoint
	daily       []models.MarketFlowPoint
	coverage    models.MarketFlowCoverage
	err         error
}

func (s *stubFundFlowStore) Buckets(_ context.Context, venue string, from, to time.Time) ([]models.MarketFlowBucket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.venuesAsked = append(s.venuesAsked, venue)
	s.windowSpans = append(s.windowSpans, to.Sub(from))
	return s.buckets, s.err
}

func (s *stubFundFlowStore) Series(_ context.Context, _ string, _, _ time.Time, step time.Duration) ([]models.MarketFlowPoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.steps = append(s.steps, step)
	return s.series, s.err
}

func (s *stubFundFlowStore) Daily(_ context.Context, _ string, _, _ time.Time) ([]models.MarketFlowPoint, error) {
	return s.daily, s.err
}

func (s *stubFundFlowStore) Coverage(_ context.Context, _ string, _ time.Time) (models.MarketFlowCoverage, error) {
	return s.coverage, s.err
}

func (s *stubFundFlowStore) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

func testRegistry(t *testing.T) *marketdata.Registry {
	t.Helper()
	registry, err := marketdata.NewRegistry(stubFundFlowVenue{id: "mexc"}, stubFundFlowVenue{id: "kraken"})
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	return registry
}

func populatedStore() *stubFundFlowStore {
	return &stubFundFlowStore{
		buckets: []models.MarketFlowBucket{
			{Bucket: "large", BuyQuantity: 100, SellQuantity: 40, BuyQuote: 80, SellQuote: 32, BuyTradeCount: 2, SellTradeCount: 1},
			{Bucket: "medium", BuyQuantity: 30, SellQuantity: 50, BuyQuote: 24, SellQuote: 40, BuyTradeCount: 3, SellTradeCount: 4},
			{Bucket: "small", BuyQuantity: 5, SellQuantity: 20, BuyQuote: 4, SellQuote: 16, BuyTradeCount: 6, SellTradeCount: 9},
		},
		series: []models.MarketFlowPoint{{Time: 1, BuyQuantity: 2, SellQuantity: 1, NetQuantity: 1}},
		daily:  []models.MarketFlowPoint{{Time: 1, NetQuantity: 3}},
	}
}

func doFundFlowRequest(t *testing.T, registry *marketdata.Registry, store fundFlowStore, target string) *httptest.ResponseRecorder {
	t.Helper()
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/market/fundflow", newMarketFundFlowHandler(registry, store, cache.New()))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
	return response
}

func TestMarketFundFlowTotalsMatchTheSummedSides(t *testing.T) {
	store := populatedStore()
	response := doFundFlowRequest(t, testRegistry(t), store, "/market/fundflow")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}

	var got marketFundFlowResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Totals.BuyQuantity != 135 || got.Totals.SellQuantity != 110 {
		t.Errorf("totals buy/sell = %v/%v, want 135/110", got.Totals.BuyQuantity, got.Totals.SellQuantity)
	}
	if got.Totals.NetQuantity != 25 {
		t.Errorf("total net = %v, want 25", got.Totals.NetQuantity)
	}
	if got.Totals.NetQuote != 20 {
		t.Errorf("total net quote = %v, want 20", got.Totals.NetQuote)
	}
	if got.Totals.BuyTradeCount != 11 || got.Totals.SellTradeCount != 14 {
		t.Errorf("total counts = %d/%d, want 11/14", got.Totals.BuyTradeCount, got.Totals.SellTradeCount)
	}
	if got.Totals.Bucket != "total" {
		t.Errorf("totals bucket = %q, want total", got.Totals.Bucket)
	}
}

func TestMarketFundFlowDefaultsAndAdvertisesItsOptions(t *testing.T) {
	store := populatedStore()
	response := doFundFlowRequest(t, testRegistry(t), store, "/market/fundflow")

	var got marketFundFlowResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Window != defaultFundFlowWindow {
		t.Errorf("window = %q, want the default %q", got.Window, defaultFundFlowWindow)
	}
	if got.Venue.ID != "mexc" {
		t.Errorf("venue = %q, want the registry default", got.Venue.ID)
	}
	// The UI decides whether to render a venue switcher from this list, so
	// every configured venue has to appear even when one is selected.
	if len(got.Venues) != 2 {
		t.Errorf("venues = %d, want 2", len(got.Venues))
	}
	if len(got.Windows) != len(fundFlowWindows) {
		t.Errorf("windows = %d, want %d", len(got.Windows), len(fundFlowWindows))
	}
	// Bands must be reported so the UI can label them with real numbers
	// instead of implying the venue publishes this split.
	if got.Bands.MediumFrom != 10 || got.Bands.LargeFrom != 100 || got.Bands.QuoteAsset != "USDT" {
		t.Errorf("bands = %#v", got.Bands)
	}
	if got.WindowEnd <= got.WindowStart {
		t.Errorf("window %d..%d is not ordered", got.WindowStart, got.WindowEnd)
	}
	if got.SeriesStepMs != time.Hour.Milliseconds() {
		t.Errorf("series step = %d ms, want the 1d window's hourly step", got.SeriesStepMs)
	}
}

func TestMarketFundFlowAppliesTheRequestedWindowAndVenue(t *testing.T) {
	store := populatedStore()
	response := doFundFlowRequest(t, testRegistry(t), store, "/market/fundflow?venue=kraken&window=15m")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", response.Code, response.Body.String())
	}

	var got marketFundFlowResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Venue.ID != "kraken" || got.Window != "15m" {
		t.Errorf("venue/window = %q/%q, want kraken/15m", got.Venue.ID, got.Window)
	}
	if len(store.venuesAsked) == 0 || store.venuesAsked[0] != "kraken" {
		t.Errorf("store queried %v, want the requested venue", store.venuesAsked)
	}
	if len(store.windowSpans) == 0 || store.windowSpans[0] != 15*time.Minute {
		t.Errorf("queried span = %v, want 15m", store.windowSpans)
	}
	if len(store.steps) == 0 || store.steps[0] != time.Minute {
		t.Errorf("series step = %v, want 1m for the 15m window", store.steps)
	}
}

func TestMarketFundFlowRejectsUnknownVenueAndWindow(t *testing.T) {
	registry := testRegistry(t)

	response := doFundFlowRequest(t, registry, populatedStore(), "/market/fundflow?venue=binance")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown venue status = %d, want 400", response.Code)
	}

	response = doFundFlowRequest(t, registry, populatedStore(), "/market/fundflow?window=7d")
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown window status = %d, want 400", response.Code)
	}
	// A rejected window must not leak into an unbounded query.
	var body map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["error"] == "" {
		t.Error("a 400 must explain the accepted values")
	}
}

func TestMarketFundFlowCachesPerVenueAndWindow(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := populatedStore()
	registry := testRegistry(t)
	router := gin.New()
	router.GET("/market/fundflow", newMarketFundFlowHandler(registry, store, cache.New()))

	get := func(target string) {
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, target, nil))
		if response.Code != http.StatusOK {
			t.Fatalf("%s status = %d", target, response.Code)
		}
	}

	get("/market/fundflow?window=1h")
	get("/market/fundflow?window=1h")
	if got := store.callCount(); got != 1 {
		t.Fatalf("same venue+window recomputed %d times, want 1 within the TTL", got)
	}

	// A different window is a different key: it must not be served the
	// cached rollup of another window.
	get("/market/fundflow?window=4h")
	if got := store.callCount(); got != 2 {
		t.Fatalf("calls = %d, want a separate computation per window", got)
	}
	get("/market/fundflow?venue=kraken&window=1h")
	if got := store.callCount(); got != 3 {
		t.Fatalf("calls = %d, want a separate computation per venue", got)
	}
}

func TestMarketFundFlowMapsStoreFailureToInternalError(t *testing.T) {
	store := &stubFundFlowStore{err: errors.New("mongo unavailable")}
	response := doFundFlowRequest(t, testRegistry(t), store, "/market/fundflow")
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", response.Code)
	}
	if got, want := response.Body.String(), `{"error":"internal server error"}`; got != want {
		t.Fatalf("body = %q, want %q", got, want)
	}
}

func TestMarketFundFlowReportsCoverageForAnEmptyStore(t *testing.T) {
	// Nothing collected yet is the state every fresh deploy starts in. The
	// response must be a valid zeroed rollup that says so, rather than an
	// error, so the UI can distinguish "no flow" from "not collecting yet".
	store := &stubFundFlowStore{
		buckets: []models.MarketFlowBucket{
			{Bucket: "large"}, {Bucket: "medium"}, {Bucket: "small"},
		},
	}
	response := doFundFlowRequest(t, testRegistry(t), store, "/market/fundflow")
	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", response.Code)
	}

	var got marketFundFlowResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Coverage.Complete {
		t.Error("an empty store must not report complete coverage")
	}
	if got.Coverage.FirstTradeAt != nil || got.Coverage.TradeCount != 0 {
		t.Errorf("coverage = %#v, want empty", got.Coverage)
	}
	if got.Totals.NetQuantity != 0 {
		t.Errorf("net = %v, want 0", got.Totals.NetQuantity)
	}
}

func TestFundFlowWindowStepsStayLegible(t *testing.T) {
	// Every window must divide into a point count a narrow card can render.
	for _, window := range fundFlowWindows {
		points := int(window.Duration / window.Step)
		if points < 10 || points > 30 {
			t.Errorf("window %s yields %d points, want 10-30", window.ID, points)
		}
		if window.Duration%window.Step != 0 {
			t.Errorf("window %s does not divide evenly into %s steps", window.ID, window.Step)
		}
	}
	if _, ok := lookupFundFlowWindow(defaultFundFlowWindow); !ok {
		t.Errorf("default window %q is not in the table", defaultFundFlowWindow)
	}
}
