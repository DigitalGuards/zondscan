package marketdata

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
)

// SizeBucket is a trade-size class used by the fund-flow rollups.
//
// These bands are ZondScan's own classification. Exchanges publish similar
// "large/medium/small" splits but do not document their thresholds, so the
// numbers here are deliberately ours and must be presented as ours. Do not
// label a rollup built from these as a venue's own classification.
type SizeBucket string

const (
	BucketSmall  SizeBucket = "small"
	BucketMedium SizeBucket = "medium"
	BucketLarge  SizeBucket = "large"
)

// OrderedSizeBuckets lists the buckets largest first, which is the order the
// API emits and the UI renders. Iterate this instead of ranging over a map so
// the response order is stable.
var OrderedSizeBuckets = []SizeBucket{BucketLarge, BucketMedium, BucketSmall}

// ValidSizeBucket reports whether a stored bucket string is one this build
// knows. Rows written by a future build with more bands must not silently
// vanish from a total, so callers validate rather than assume.
func ValidSizeBucket(value string) bool {
	switch SizeBucket(value) {
	case BucketSmall, BucketMedium, BucketLarge:
		return true
	default:
		return false
	}
}

// SizeThresholds splits executions by quote-currency notional (price times
// quantity) rather than base quantity, so a band keeps its meaning as the
// base asset's price moves. Thresholds are per venue because a second venue
// may quote a different asset, where the same numbers would mean something
// else entirely.
type SizeThresholds struct {
	// Medium is the inclusive floor of the medium band.
	Medium float64
	// Large is the inclusive floor of the large band.
	Large float64
}

// Classify places one execution into a band by its quote notional.
func (t SizeThresholds) Classify(quoteQuantity float64) SizeBucket {
	switch {
	case quoteQuantity >= t.Large:
		return BucketLarge
	case quoteQuantity >= t.Medium:
		return BucketMedium
	default:
		return BucketSmall
	}
}

// Valid reports whether the bands are ordered and positive. An invalid set
// would silently collapse every trade into one bucket, so the registry
// rejects a venue that supplies one.
func (t SizeThresholds) Valid() bool {
	return t.Medium > 0 && t.Large > t.Medium
}

// Venue is one public spot market ZondScan reads. Implementations are
// read-only and fully fixed at construction: no request parameter selects a
// venue's upstream URL or symbol, only which already-configured venue to
// read. ID is the stable lowercase key used in URLs and in stored rows, so
// changing one orphans that venue's history.
type Venue interface {
	ID() string
	Name() string
	Symbol() string
	QuoteAsset() string
	SizeThresholds() SizeThresholds
	FetchOrderBook(context.Context) (OrderBookSnapshot, error)
	FetchTrades(context.Context) ([]Trade, error)
}

// Registry is the fixed set of configured venues. It is built once at
// startup and never mutated, so it is safe for concurrent readers.
type Registry struct {
	order []Venue
	byID  map[string]Venue
}

// NewRegistry validates and freezes a venue set. The first venue is the
// default the API serves when a request names none, which keeps existing
// single-venue clients working unchanged as venues are added.
func NewRegistry(venues ...Venue) (*Registry, error) {
	if len(venues) == 0 {
		return nil, errors.New("market data registry needs at least one venue")
	}
	byID := make(map[string]Venue, len(venues))
	for _, venue := range venues {
		id := venue.ID()
		if id == "" {
			return nil, errors.New("market data venue has an empty id")
		}
		if _, exists := byID[id]; exists {
			return nil, fmt.Errorf("market data venue %q is registered twice", id)
		}
		if !venue.SizeThresholds().Valid() {
			return nil, fmt.Errorf("market data venue %q has invalid size thresholds", id)
		}
		byID[id] = venue
	}
	order := make([]Venue, len(venues))
	copy(order, venues)
	return &Registry{order: order, byID: byID}, nil
}

// Default is the venue served when a request does not name one.
func (r *Registry) Default() Venue { return r.order[0] }

// ByID resolves a venue by its stable key.
func (r *Registry) ByID(id string) (Venue, bool) {
	venue, ok := r.byID[id]
	return venue, ok
}

// All returns the venues in registration order.
func (r *Registry) All() []Venue {
	out := make([]Venue, len(r.order))
	copy(out, r.order)
	return out
}

// IDs returns the registered keys, sorted, for error messages and for the
// venue list the UI uses to decide whether to render a venue switcher.
func (r *Registry) IDs() []string {
	ids := make([]string, 0, len(r.order))
	for _, venue := range r.order {
		ids = append(ids, venue.ID())
	}
	sort.Strings(ids)
	return ids
}

var (
	defaultRegistryOnce sync.Once
	defaultRegistry     *Registry
	defaultRegistryErr  error
)

// DefaultRegistry is the venue set this build ships with, constructed once
// and shared by the read routes and the background collector so both agree
// on which venues exist and how each classifies trade size.
//
// To add a venue: implement Venue, construct it here, and append it. The
// stored rows, the API's venue parameter, and the UI's venue list all key
// off Venue.ID with no further wiring.
func DefaultRegistry() (*Registry, error) {
	defaultRegistryOnce.Do(func() {
		mexc, err := NewMEXCClient(MEXCAPIBaseURL, nil)
		if err != nil {
			defaultRegistryErr = fmt.Errorf("construct MEXC market-data client: %w", err)
			return
		}
		defaultRegistry, defaultRegistryErr = NewRegistry(mexc)
	})
	return defaultRegistry, defaultRegistryErr
}
