package collector

import (
	"backendAPI/marketdata"
	"backendAPI/models"
	"context"
	"errors"
	"testing"
	"time"
)

type fakeVenue struct {
	id     string
	trades []marketdata.Trade
	err    error
	calls  int
}

func (f *fakeVenue) ID() string         { return f.id }
func (f *fakeVenue) Name() string       { return f.id }
func (f *fakeVenue) Symbol() string     { return "QRLUSDT" }
func (f *fakeVenue) QuoteAsset() string { return "USDT" }
func (f *fakeVenue) SizeThresholds() marketdata.SizeThresholds {
	return marketdata.SizeThresholds{Medium: 10, Large: 100}
}
func (f *fakeVenue) FetchOrderBook(context.Context) (marketdata.OrderBookSnapshot, error) {
	return marketdata.OrderBookSnapshot{}, nil
}
func (f *fakeVenue) FetchTrades(context.Context) ([]marketdata.Trade, error) {
	f.calls++
	return f.trades, f.err
}

type fakeStore struct {
	received [][]models.MarketTrade
	upserted int
	err      error
}

func (f *fakeStore) UpsertMarketTrades(_ context.Context, trades []models.MarketTrade) (int, error) {
	f.received = append(f.received, trades)
	return f.upserted, f.err
}

func tape() []marketdata.Trade {
	return []marketdata.Trade{
		{ID: "a", Price: "0.8", Quantity: "5", QuoteQuantity: "4", Time: 1787139418000, AggressorSide: "sell"},
		{ID: "b", Price: "0.8", Quantity: "25", QuoteQuantity: "20", Time: 1787139419000, AggressorSide: "buy"},
		{ID: "c", Price: "0.8", Quantity: "250", QuoteQuantity: "200", Time: 1787139420000, AggressorSide: "buy"},
	}
}

func TestTradesToModelsClassifiesAndNamespacesRows(t *testing.T) {
	venue := &fakeVenue{id: "mexc"}
	rows, err := TradesToModels(venue, tape())
	if err != nil {
		t.Fatalf("TradesToModels: %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}

	wantBuckets := []string{"small", "medium", "large"}
	for i, row := range rows {
		if row.Bucket != wantBuckets[i] {
			t.Errorf("row %d bucket = %q, want %q", i, row.Bucket, wantBuckets[i])
		}
		if row.Venue != "mexc" || row.Symbol != "QRLUSDT" {
			t.Errorf("row %d venue/symbol = %q/%q", i, row.Venue, row.Symbol)
		}
		if !row.At.Equal(time.UnixMilli(row.Time).UTC()) {
			t.Errorf("row %d At = %v, want the UTC form of Time", i, row.At)
		}
	}
	// The id must carry the venue so two venues cannot collide on a
	// structurally identical print.
	if rows[0].ID != "mexc:a" {
		t.Errorf("row id = %q, want %q", rows[0].ID, "mexc:a")
	}
}

func TestTradesToModelsIsStableAcrossPollsForTheSamePrint(t *testing.T) {
	venue := &fakeVenue{id: "mexc"}
	first, err := TradesToModels(venue, tape())
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	// A later poll returns the same prints plus a newer one, which is what
	// makes storage idempotent: the overlapping rows must keep their ids.
	second, err := TradesToModels(venue, append(tape(), marketdata.Trade{
		ID: "d", Price: "0.81", Quantity: "1", QuoteQuantity: "0.81", Time: 1787139421000, AggressorSide: "buy",
	}))
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	for i := range first {
		if first[i].ID != second[i].ID {
			t.Errorf("row %d id changed between polls: %q then %q", i, first[i].ID, second[i].ID)
		}
	}
}

func TestTradesToModelsRejectsUnusableRows(t *testing.T) {
	venue := &fakeVenue{id: "mexc"}
	cases := []struct {
		name  string
		trade marketdata.Trade
	}{
		{"bad price", marketdata.Trade{ID: "x", Price: "abc", Quantity: "1", QuoteQuantity: "1", Time: 1}},
		{"bad quantity", marketdata.Trade{ID: "x", Price: "1", Quantity: "", QuoteQuantity: "1", Time: 1}},
		{"bad quote", marketdata.Trade{ID: "x", Price: "1", Quantity: "1", QuoteQuantity: "n/a", Time: 1}},
		{"zero time", marketdata.Trade{ID: "x", Price: "1", Quantity: "1", QuoteQuantity: "1", Time: 0}},
		{"negative time", marketdata.Trade{ID: "x", Price: "1", Quantity: "1", QuoteQuantity: "1", Time: -5}},
	}
	for _, tc := range cases {
		if _, err := TradesToModels(venue, []marketdata.Trade{tc.trade}); err == nil {
			t.Errorf("%s: expected an error", tc.name)
		}
	}
}

func TestCollectOnceStoresTheTape(t *testing.T) {
	venue := &fakeVenue{id: "mexc", trades: tape()}
	store := &fakeStore{upserted: 2}
	c := NewMarketTradeCollectorWithStore(nil, store, time.Minute, time.Second)

	stored, total, err := c.CollectOnce(context.Background(), venue)
	if err != nil {
		t.Fatalf("CollectOnce: %v", err)
	}
	if stored != 2 || total != 3 {
		t.Fatalf("stored/total = %d/%d, want 2/3", stored, total)
	}
	if len(store.received) != 1 || len(store.received[0]) != 3 {
		t.Fatalf("store received %d batches", len(store.received))
	}
}

func TestCollectOnceSurfacesFetchAndStoreFailures(t *testing.T) {
	fetchFail := &fakeVenue{id: "mexc", err: errors.New("upstream down")}
	c := NewMarketTradeCollectorWithStore(nil, &fakeStore{}, time.Minute, time.Second)
	if _, _, err := c.CollectOnce(context.Background(), fetchFail); err == nil {
		t.Error("a fetch failure must be reported")
	}

	storeFail := NewMarketTradeCollectorWithStore(nil, &fakeStore{err: errors.New("mongo down")}, time.Minute, time.Second)
	if _, _, err := storeFail.CollectOnce(context.Background(), &fakeVenue{id: "mexc", trades: tape()}); err == nil {
		t.Error("a store failure must be reported")
	}
}

func TestRunCollectsImmediatelyThenStops(t *testing.T) {
	venue := &fakeVenue{id: "mexc", trades: tape()}
	registry, err := marketdata.NewRegistry(venue)
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	store := &fakeStore{}
	// A long interval proves the first collection happens up front rather
	// than only on the first tick; a fresh deploy must capture the venue's
	// retained tape without waiting.
	c := NewMarketTradeCollectorWithStore(registry, store, time.Hour, time.Second)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		c.Run(ctx)
		close(done)
	}()

	deadline := time.After(2 * time.Second)
	for venue.calls == 0 {
		select {
		case <-deadline:
			t.Fatal("collector did not collect before its first tick")
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("collector did not stop on context cancellation")
	}
}

func TestNewMarketTradeCollectorAppliesDefaults(t *testing.T) {
	c := NewMarketTradeCollectorWithStore(nil, &fakeStore{}, 0, 0)
	if c.interval != DefaultInterval || c.timeout != DefaultTimeout {
		t.Errorf("defaults = %s/%s, want %s/%s", c.interval, c.timeout, DefaultInterval, DefaultTimeout)
	}
}
