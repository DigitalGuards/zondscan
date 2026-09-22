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
	"reflect"
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
	failAt      string
	queries     []fundFlowStoreQuery
}

type fundFlowStoreQuery struct {
	method   string
	venue    string
	from     time.Time
	to       time.Time
	step     time.Duration
	context  context.Context
	deadline time.Time
}

func (s *stubFundFlowStore) recordQuery(ctx context.Context, method, venue string, from, to time.Time, step time.Duration) error {
	deadline, _ := ctx.Deadline()
	s.queries = append(s.queries, fundFlowStoreQuery{method, venue, from, to, step, ctx, deadline})
	if s.failAt == "" || s.failAt == method {
		return s.err
	}
	return nil
}

func (s *stubFundFlowStore) Buckets(ctx context.Context, venue string, from, to time.Time) ([]models.MarketFlowBucket, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.venuesAsked = append(s.venuesAsked, venue)
	s.windowSpans = append(s.windowSpans, to.Sub(from))
	return s.buckets, s.recordQuery(ctx, "buckets", venue, from, to, 0)
}

func (s *stubFundFlowStore) Series(ctx context.Context, venue string, from, to time.Time, step time.Duration) ([]models.MarketFlowPoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.steps = append(s.steps, step)
	return s.series, s.recordQuery(ctx, "series", venue, from, to, step)
}

func (s *stubFundFlowStore) Daily(ctx context.Context, venue string, from, to time.Time) ([]models.MarketFlowPoint, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.daily, s.recordQuery(ctx, "daily", venue, from, to, 0)
}

func (s *stubFundFlowStore) Coverage(ctx context.Context, venue string, from time.Time) (models.MarketFlowCoverage, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.coverage, s.recordQuery(ctx, "coverage", venue, from, time.Time{}, 0)
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

	store := populatedStore()
	response = doFundFlowRequest(t, registry, store, "/market/fundflow?window=90d")
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
	if store.callCount() != 0 {
		t.Error("a rejected window must not query storage")
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
	get("/market/fundflow?window=7d")
	get("/market/fundflow?window=30d")
	get("/market/fundflow?window=7d")
	get("/market/fundflow?window=30d")
	if got := store.callCount(); got != 5 {
		t.Fatalf("calls = %d, want separate cached computations for 7d and 30d", got)
	}
	get("/market/fundflow?venue=kraken&window=30d")
	if got := store.callCount(); got != 6 {
		t.Fatalf("calls = %d, want a separate long-window computation per venue", got)
	}
}

func TestMarketFundFlowMapsStoreFailureToInternalError(t *testing.T) {
	for index, stage := range []string{"buckets", "series", "daily", "coverage"} {
		t.Run(stage, func(t *testing.T) {
			store := populatedStore()
			store.err = errors.New("mongo unavailable")
			store.failAt = stage
			router := gin.New()
			router.GET("/market/fundflow", newMarketFundFlowHandler(testRegistry(t), store, cache.New()))
			get := func() *httptest.ResponseRecorder {
				response := httptest.NewRecorder()
				router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/market/fundflow?window=30d", nil))
				return response
			}
			response := get()
			if response.Code != http.StatusInternalServerError {
				t.Fatalf("status = %d, want 500", response.Code)
			}
			if got, want := response.Body.String(), `{"error":"internal server error"}`; got != want {
				t.Fatalf("body = %q, want %q", got, want)
			}
			if len(store.queries) != index+1 {
				t.Fatalf("queries = %d, want %d before stopping on failure", len(store.queries), index+1)
			}
			store.err = nil
			if response := get(); response.Code != http.StatusOK {
				t.Fatalf("retry status = %d, want 200 after storage recovers", response.Code)
			}
			if store.callCount() != 2 {
				t.Fatalf("bucket calls = %d, want retry instead of cached failure", store.callCount())
			}
		})
	}
}

func TestMarketFundFlowReportsCoverageForAnEmptyStore(t *testing.T) {
	// Nothing collected yet is the state every fresh deploy starts in. The
	// response must be a valid zeroed rollup that says so, rather than an
	// error, so the UI can distinguish "no flow" from "not collecting yet".
	for _, window := range []string{"1d", "7d", "30d"} {
		t.Run(window, func(t *testing.T) {
			store := &stubFundFlowStore{
				buckets: []models.MarketFlowBucket{
					{Bucket: "large"}, {Bucket: "medium"}, {Bucket: "small"},
				},
			}
			response := doFundFlowRequest(t, testRegistry(t), store, "/market/fundflow?window="+window)
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
			if got.Coverage.FirstTradeAt != nil || got.Coverage.LastTradeAt != nil || got.Coverage.TradeCount != 0 {
				t.Errorf("coverage = %#v, want empty", got.Coverage)
			}
			if got.Totals.NetQuantity != 0 || got.Totals.BuyTradeCount+got.Totals.SellTradeCount != 0 {
				t.Errorf("totals = %#v, want zero volume and trade counts", got.Totals)
			}
		})
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

func TestMarketFundFlowWindowRangesAndQueryBudget(t *testing.T) {
	tests := []struct {
		id        string
		duration  time.Duration
		step      time.Duration
		points    int64
		dailyDays int
	}{
		{"15m", 15 * time.Minute, time.Minute, 15, 5},
		{"30m", 30 * time.Minute, 2 * time.Minute, 15, 5},
		{"1h", time.Hour, 5 * time.Minute, 12, 5},
		{"2h", 2 * time.Hour, 10 * time.Minute, 12, 5},
		{"4h", 4 * time.Hour, 15 * time.Minute, 16, 5},
		{"1d", 24 * time.Hour, time.Hour, 24, 5},
		{"7d", 7 * 24 * time.Hour, 6 * time.Hour, 28, 7},
		{"30d", 30 * 24 * time.Hour, 24 * time.Hour, 30, 30},
	}
	for _, test := range tests {
		t.Run(test.id, func(t *testing.T) {
			store := populatedStore()
			before := time.Now()
			response := doFundFlowRequest(t, testRegistry(t), store, "/market/fundflow?venue=kraken&window="+test.id)
			if response.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200: %s", response.Code, response.Body.String())
			}
			var got marketFundFlowResponse
			if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if got.Window != test.id || got.WindowEnd-got.WindowStart != test.duration.Milliseconds() {
				t.Errorf("window %q spans %d ms, want %s", got.Window, got.WindowEnd-got.WindowStart, test.duration)
			}
			if got.SeriesStepMs != test.step.Milliseconds() {
				t.Fatalf("step = %d ms, want %s", got.SeriesStepMs, test.step)
			}
			if points := (got.WindowEnd - got.WindowStart) / got.SeriesStepMs; points != test.points {
				t.Errorf("series buckets = %d, want %d", points, test.points)
			}
			if got.DailyDays != test.dailyDays || got.DailyEnd != got.WindowEnd {
				t.Errorf("daily metadata = %d days ending %d, want %d days ending %d", got.DailyDays, got.DailyEnd, test.dailyDays, got.WindowEnd)
			}
			wantDailyStart := time.UnixMilli(got.WindowEnd).UTC().Truncate(24*time.Hour).AddDate(0, 0, -(test.dailyDays - 1))
			if got.DailyStart != wantDailyStart.UnixMilli() {
				t.Errorf("daily start = %d, want UTC day boundary %d", got.DailyStart, wantDailyStart.UnixMilli())
			}
			if len(store.queries) != 4 {
				t.Fatalf("queries = %d, want the same four reads for every window", len(store.queries))
			}
			for index, query := range store.queries {
				if query.venue != "kraken" {
					t.Errorf("query %d venue = %q, want kraken", index, query.venue)
				}
				if query.context != store.queries[0].context || !query.deadline.Equal(store.queries[0].deadline) {
					t.Errorf("query %d must share the single computation context and deadline", index)
				}
				if query.deadline.Before(before) || query.deadline.After(time.Now().Add(10*time.Second)) {
					t.Errorf("query %d deadline = %s, want a bounded 10-second budget", index, query.deadline)
				}
				wantStart := got.WindowStart
				if query.method == "daily" {
					wantStart = got.DailyStart
				}
				if query.from.UnixMilli() != wantStart {
					t.Errorf("%s query starts at %d, want %d", query.method, query.from.UnixMilli(), wantStart)
				}
				if query.method != "coverage" && query.to.UnixMilli() != got.WindowEnd {
					t.Errorf("%s query ends at %d, want %d", query.method, query.to.UnixMilli(), got.WindowEnd)
				}
			}
			if store.queries[1].step != test.step {
				t.Errorf("store series step = %s, want %s", store.queries[1].step, test.step)
			}
			if err := store.queries[0].context.Err(); err != context.Canceled {
				t.Errorf("completed query context error = %v, want cancellation", err)
			}
		})
	}
}

func TestMarketFundFlowPreservesObservedCoverageForLongWindows(t *testing.T) {
	now := time.Now()
	first := now.Add(-36 * time.Hour).UnixMilli()
	last := now.Add(-time.Hour).UnixMilli()
	for _, window := range []string{"7d", "30d"} {
		t.Run(window, func(t *testing.T) {
			store := populatedStore()
			store.coverage = models.MarketFlowCoverage{FirstTradeAt: &first, LastTradeAt: &last, TradeCount: 25}
			response := doFundFlowRequest(t, testRegistry(t), store, "/market/fundflow?window="+window)
			var got marketFundFlowResponse
			if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if !reflect.DeepEqual(got.Coverage, store.coverage) {
				t.Errorf("coverage = %#v, want stored extent %#v", got.Coverage, store.coverage)
			}
			if got.WindowStart >= first || got.DailyStart >= first {
				t.Error("the requested chart ranges must remain explicit when observed history starts later")
			}
			if got.Totals.BuyQuantity != 135 || got.Totals.SellQuantity != 110 {
				t.Error("partial history must preserve the observed volume")
			}
		})
	}
}

func TestMarketFundFlowRetainsLegacyCoverageFlagWithoutInferringContinuity(t *testing.T) {
	first := time.Now().Add(-40 * 24 * time.Hour).UnixMilli()
	last := time.Now().Add(-35 * 24 * time.Hour).UnixMilli()
	store := &stubFundFlowStore{
		buckets: []models.MarketFlowBucket{{Bucket: "large"}, {Bucket: "medium"}, {Bucket: "small"}},
		coverage: models.MarketFlowCoverage{
			FirstTradeAt: &first, LastTradeAt: &last, TradeCount: 3, Complete: true,
		},
	}
	response := doFundFlowRequest(t, testRegistry(t), store, "/market/fundflow?window=30d")
	var got marketFundFlowResponse
	if err := json.Unmarshal(response.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(got.Coverage, store.coverage) {
		t.Errorf("legacy coverage fields changed: %#v", got.Coverage)
	}
	if got.Totals.BuyTradeCount+got.Totals.SellTradeCount != 0 {
		t.Error("old stored history must not imply trades in the selected window")
	}
}
