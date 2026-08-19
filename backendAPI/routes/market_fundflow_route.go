package routes

import (
	"backendAPI/cache"
	"backendAPI/db"
	"backendAPI/marketdata"
	"backendAPI/models"
	"context"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	marketFundFlowCacheTTL = 30 * time.Second
	// dailyBarDays is the span of the daily net-inflow bars. Venues expose
	// no historical tape, so this fills in one bar per day from the first
	// deploy rather than being backfillable.
	dailyBarDays = 5
)

// fundFlowWindow is one selectable rollup window. Step sets the resolution
// of the net-inflow series, chosen so every window returns 12-24 points:
// enough shape to read, few enough to stay legible on a narrow card.
type fundFlowWindow struct {
	ID       string
	Duration time.Duration
	Step     time.Duration
}

// fundFlowWindows is ordered shortest first, which is the order the UI
// renders its selector.
var fundFlowWindows = []fundFlowWindow{
	{ID: "15m", Duration: 15 * time.Minute, Step: time.Minute},
	{ID: "30m", Duration: 30 * time.Minute, Step: 2 * time.Minute},
	{ID: "1h", Duration: time.Hour, Step: 5 * time.Minute},
	{ID: "2h", Duration: 2 * time.Hour, Step: 10 * time.Minute},
	{ID: "4h", Duration: 4 * time.Hour, Step: 15 * time.Minute},
	{ID: "1d", Duration: 24 * time.Hour, Step: time.Hour},
}

const defaultFundFlowWindow = "1d"

func lookupFundFlowWindow(id string) (fundFlowWindow, bool) {
	for _, window := range fundFlowWindows {
		if window.ID == id {
			return window, true
		}
	}
	return fundFlowWindow{}, false
}

func fundFlowWindowIDs() []string {
	ids := make([]string, 0, len(fundFlowWindows))
	for _, window := range fundFlowWindows {
		ids = append(ids, window.ID)
	}
	return ids
}

// fundFlowStore is the read seam over stored trades, so the handler can be
// exercised without Mongo.
type fundFlowStore interface {
	Buckets(ctx context.Context, venue string, from, to time.Time) ([]models.MarketFlowBucket, error)
	Series(ctx context.Context, venue string, from, to time.Time, step time.Duration) ([]models.MarketFlowPoint, error)
	Daily(ctx context.Context, venue string, from, to time.Time) ([]models.MarketFlowPoint, error)
	Coverage(ctx context.Context, venue string, windowStart time.Time) (models.MarketFlowCoverage, error)
}

type mongoFundFlowStore struct{}

func (mongoFundFlowStore) Buckets(ctx context.Context, venue string, from, to time.Time) ([]models.MarketFlowBucket, error) {
	return db.MarketFlowBuckets(ctx, venue, from, to)
}

func (mongoFundFlowStore) Series(ctx context.Context, venue string, from, to time.Time, step time.Duration) ([]models.MarketFlowPoint, error) {
	return db.MarketFlowSeries(ctx, venue, from, to, step)
}

func (mongoFundFlowStore) Daily(ctx context.Context, venue string, from, to time.Time) ([]models.MarketFlowPoint, error) {
	return db.MarketFlowDaily(ctx, venue, from, to)
}

func (mongoFundFlowStore) Coverage(ctx context.Context, venue string, windowStart time.Time) (models.MarketFlowCoverage, error) {
	return db.MarketFlowCoverage(ctx, venue, windowStart)
}

type marketVenueInfo struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Symbol     string `json:"symbol"`
	QuoteAsset string `json:"quoteAsset"`
}

// marketSizeBands reports the thresholds a response was classified with, so
// the UI can label the bands with real numbers instead of implying the
// venue publishes this split.
type marketSizeBands struct {
	MediumFrom float64 `json:"mediumFrom"`
	LargeFrom  float64 `json:"largeFrom"`
	QuoteAsset string  `json:"quoteAsset"`
}

type marketFundFlowResponse struct {
	Venue        marketVenueInfo           `json:"venue"`
	Venues       []marketVenueInfo         `json:"venues"`
	Window       string                    `json:"window"`
	Windows      []string                  `json:"windows"`
	WindowStart  int64                     `json:"windowStart"`
	WindowEnd    int64                     `json:"windowEnd"`
	SeriesStepMs int64                     `json:"seriesStepMs"`
	Bands        marketSizeBands           `json:"bands"`
	Buckets      []models.MarketFlowBucket `json:"buckets"`
	Totals       models.MarketFlowBucket   `json:"totals"`
	Series       []models.MarketFlowPoint  `json:"series"`
	Daily        []models.MarketFlowPoint  `json:"daily"`
	Coverage     models.MarketFlowCoverage `json:"coverage"`
}

func venueInfo(venue marketdata.Venue) marketVenueInfo {
	return marketVenueInfo{
		ID:         venue.ID(),
		Name:       venue.Name(),
		Symbol:     venue.Symbol(),
		QuoteAsset: venue.QuoteAsset(),
	}
}

// handleMarketFundFlow serves GET /market/fundflow.
func handleMarketFundFlow(c *gin.Context) {
	newMarketFundFlowHandler(venueRegistry, mongoFundFlowStore{}, routeCache)(c)
}

func newMarketFundFlowHandler(
	registry *marketdata.Registry,
	store fundFlowStore,
	responseCache *cache.TTLCache,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		venue := registry.Default()
		if requested := c.Query("venue"); requested != "" {
			resolved, ok := registry.ByID(requested)
			if !ok {
				c.JSON(http.StatusBadRequest, gin.H{
					"error": "unknown venue; expected one of " + strings.Join(registry.IDs(), ", "),
				})
				return
			}
			venue = resolved
		}

		windowID := c.Query("window")
		if windowID == "" {
			windowID = defaultFundFlowWindow
		}
		window, ok := lookupFundFlowWindow(windowID)
		if !ok {
			c.JSON(http.StatusBadRequest, gin.H{
				"error": "invalid window; expected one of " + strings.Join(fundFlowWindowIDs(), ", "),
			})
			return
		}

		key := "market:fundflow:" + venue.ID() + ":" + window.ID
		value, err := responseCache.GetOrComputeContext(
			c.Request.Context(),
			key,
			marketFundFlowCacheTTL,
			func() (interface{}, error) {
				return buildFundFlowResponse(registry, store, venue, window)
			},
		)
		if err != nil {
			if c.Request.Context().Err() != nil {
				return
			}
			log.Printf("market fund flow (%s/%s): %v", venue.ID(), window.ID, err)
			respondInternal(c)
			return
		}
		response, ok := value.(marketFundFlowResponse)
		if !ok {
			log.Printf("market fund flow cache returned unexpected type %T", value)
			respondInternal(c)
			return
		}
		c.JSON(http.StatusOK, response)
	}
}

func buildFundFlowResponse(
	registry *marketdata.Registry,
	store fundFlowStore,
	venue marketdata.Venue,
	window fundFlowWindow,
) (marketFundFlowResponse, error) {
	// The rollup reads stored rows only, so it uses its own budget rather
	// than a caller's: one client disconnecting must not abandon a result
	// other waiters on the same cache key are blocked on.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	end := time.Now().UTC()
	start := end.Add(-window.Duration)
	dailyStart := end.Truncate(24*time.Hour).AddDate(0, 0, -(dailyBarDays - 1))

	buckets, err := store.Buckets(ctx, venue.ID(), start, end)
	if err != nil {
		return marketFundFlowResponse{}, err
	}
	series, err := store.Series(ctx, venue.ID(), start, end, window.Step)
	if err != nil {
		return marketFundFlowResponse{}, err
	}
	daily, err := store.Daily(ctx, venue.ID(), dailyStart, end)
	if err != nil {
		return marketFundFlowResponse{}, err
	}
	coverage, err := store.Coverage(ctx, venue.ID(), start)
	if err != nil {
		return marketFundFlowResponse{}, err
	}

	venues := make([]marketVenueInfo, 0, len(registry.All()))
	for _, known := range registry.All() {
		venues = append(venues, venueInfo(known))
	}

	thresholds := venue.SizeThresholds()
	return marketFundFlowResponse{
		Venue:        venueInfo(venue),
		Venues:       venues,
		Window:       window.ID,
		Windows:      fundFlowWindowIDs(),
		WindowStart:  start.UnixMilli(),
		WindowEnd:    end.UnixMilli(),
		SeriesStepMs: window.Step.Milliseconds(),
		Bands: marketSizeBands{
			MediumFrom: thresholds.Medium,
			LargeFrom:  thresholds.Large,
			QuoteAsset: venue.QuoteAsset(),
		},
		Buckets:  buckets,
		Totals:   sumFlowBuckets(buckets),
		Series:   series,
		Daily:    daily,
		Coverage: coverage,
	}, nil
}

// sumFlowBuckets folds the bands into a total row. Net is recomputed from
// the summed sides rather than by adding the per-band nets, so the total
// stays consistent with its own buy and sell columns.
func sumFlowBuckets(buckets []models.MarketFlowBucket) models.MarketFlowBucket {
	total := models.MarketFlowBucket{Bucket: "total"}
	for _, bucket := range buckets {
		total.BuyQuantity += bucket.BuyQuantity
		total.SellQuantity += bucket.SellQuantity
		total.BuyQuote += bucket.BuyQuote
		total.SellQuote += bucket.SellQuote
		total.BuyTradeCount += bucket.BuyTradeCount
		total.SellTradeCount += bucket.SellTradeCount
	}
	total.NetQuantity = total.BuyQuantity - total.SellQuantity
	total.NetQuote = total.BuyQuote - total.SellQuote
	return total
}
