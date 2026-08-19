// Package collector runs the background pollers that turn third-party feeds
// into stored history. It exists because the read API only fetches when a
// visitor arrives: an endpoint that computes a 24-hour rollup on demand
// would have nothing to compute from on a quiet site.
package collector

import (
	"backendAPI/db"
	"backendAPI/marketdata"
	"backendAPI/models"
	"context"
	"fmt"
	"log"
	"strconv"
	"time"
)

const (
	// DefaultInterval is deliberately far shorter than the upstream tape's
	// span (MEXC holds roughly 8 hours of QRLUSDT prints). Every poll
	// re-reads a heavily overlapping window, so a missed poll, a restart,
	// or a multi-hour outage costs no trades at all.
	DefaultInterval = time.Minute
	// DefaultTimeout bounds one venue's fetch-and-store cycle.
	DefaultTimeout = 20 * time.Second
)

// TradeStore is the persistence seam. The production implementation is
// Mongo; tests supply their own.
type TradeStore interface {
	UpsertMarketTrades(ctx context.Context, trades []models.MarketTrade) (int, error)
}

type mongoTradeStore struct{}

func (mongoTradeStore) UpsertMarketTrades(ctx context.Context, trades []models.MarketTrade) (int, error) {
	return db.UpsertMarketTrades(ctx, trades)
}

// MarketTradeCollector polls every configured venue and stores its tape.
type MarketTradeCollector struct {
	registry *marketdata.Registry
	store    TradeStore
	interval time.Duration
	timeout  time.Duration
}

// NewMarketTradeCollector wires a collector against the default Mongo store.
func NewMarketTradeCollector(registry *marketdata.Registry, interval, timeout time.Duration) *MarketTradeCollector {
	return NewMarketTradeCollectorWithStore(registry, mongoTradeStore{}, interval, timeout)
}

// NewMarketTradeCollectorWithStore is the injectable form used by tests.
func NewMarketTradeCollectorWithStore(
	registry *marketdata.Registry,
	store TradeStore,
	interval, timeout time.Duration,
) *MarketTradeCollector {
	if interval <= 0 {
		interval = DefaultInterval
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &MarketTradeCollector{registry: registry, store: store, interval: interval, timeout: timeout}
}

// Run blocks until ctx is cancelled, polling every venue on each tick. It
// collects once immediately so a fresh deploy captures the venue's whole
// retained tape (hours of history) instead of starting from the next print.
//
// A venue failure is logged and skipped: one venue rate-limiting must not
// stall collection for the others, and the next tick retries anyway.
func (c *MarketTradeCollector) Run(ctx context.Context) {
	venues := c.registry.All()
	log.Printf("[market-collector] starting for %d venue(s) every %s", len(venues), c.interval)

	c.collectAll(ctx, venues)

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			log.Printf("[market-collector] stopping: %v", ctx.Err())
			return
		case <-ticker.C:
			c.collectAll(ctx, venues)
		}
	}
}

func (c *MarketTradeCollector) collectAll(ctx context.Context, venues []marketdata.Venue) {
	for _, venue := range venues {
		if ctx.Err() != nil {
			return
		}
		stored, total, err := c.CollectOnce(ctx, venue)
		if err != nil {
			log.Printf("[market-collector] %s: %v", venue.ID(), err)
			continue
		}
		if stored > 0 {
			log.Printf("[market-collector] %s: stored %d new of %d tape prints", venue.ID(), stored, total)
		}
	}
}

// CollectOnce fetches one venue's tape and stores it, returning how many
// prints were new and how many the tape carried.
func (c *MarketTradeCollector) CollectOnce(ctx context.Context, venue marketdata.Venue) (int, int, error) {
	fetchCtx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()

	trades, err := venue.FetchTrades(fetchCtx)
	if err != nil {
		return 0, 0, fmt.Errorf("fetch tape: %w", err)
	}
	rows, err := TradesToModels(venue, trades)
	if err != nil {
		return 0, len(trades), err
	}
	stored, err := c.store.UpsertMarketTrades(fetchCtx, rows)
	if err != nil {
		return stored, len(trades), fmt.Errorf("store tape: %w", err)
	}
	return stored, len(trades), nil
}

// TradesToModels converts a venue tape into storable rows, classifying each
// print into a size band at write time.
//
// The row id is namespaced by venue so two venues can carry structurally
// identical prints without colliding, and so a venue's history stays
// self-contained.
func TradesToModels(venue marketdata.Venue, trades []marketdata.Trade) ([]models.MarketTrade, error) {
	thresholds := venue.SizeThresholds()
	rows := make([]models.MarketTrade, 0, len(trades))
	for _, trade := range trades {
		price, err := strconv.ParseFloat(trade.Price, 64)
		if err != nil {
			return nil, fmt.Errorf("trade %s price %q: %w", trade.ID, trade.Price, err)
		}
		quantity, err := strconv.ParseFloat(trade.Quantity, 64)
		if err != nil {
			return nil, fmt.Errorf("trade %s quantity %q: %w", trade.ID, trade.Quantity, err)
		}
		quoteQuantity, err := strconv.ParseFloat(trade.QuoteQuantity, 64)
		if err != nil {
			return nil, fmt.Errorf("trade %s quote quantity %q: %w", trade.ID, trade.QuoteQuantity, err)
		}
		if trade.Time <= 0 {
			return nil, fmt.Errorf("trade %s has a non-positive timestamp", trade.ID)
		}
		rows = append(rows, models.MarketTrade{
			ID:            venue.ID() + ":" + trade.ID,
			Venue:         venue.ID(),
			Symbol:        venue.Symbol(),
			Price:         price,
			Quantity:      quantity,
			QuoteQuantity: quoteQuantity,
			Time:          trade.Time,
			At:            time.UnixMilli(trade.Time).UTC(),
			Side:          trade.AggressorSide,
			Bucket:        string(thresholds.Classify(quoteQuantity)),
		})
	}
	return rows, nil
}
