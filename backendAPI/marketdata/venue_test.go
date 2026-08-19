package marketdata

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type stubVenue struct {
	id         string
	thresholds SizeThresholds
}

func (s stubVenue) ID() string                     { return s.id }
func (s stubVenue) Name() string                   { return s.id }
func (s stubVenue) Symbol() string                 { return "QRLUSDT" }
func (s stubVenue) QuoteAsset() string             { return "USDT" }
func (s stubVenue) SizeThresholds() SizeThresholds { return s.thresholds }
func (s stubVenue) FetchOrderBook(context.Context) (OrderBookSnapshot, error) {
	return OrderBookSnapshot{}, nil
}
func (s stubVenue) FetchTrades(context.Context) ([]Trade, error) { return nil, nil }

func validStub(id string) stubVenue {
	return stubVenue{id: id, thresholds: SizeThresholds{Medium: 10, Large: 100}}
}

func TestSizeThresholdsClassifyIsInclusiveOnEachFloor(t *testing.T) {
	thresholds := SizeThresholds{Medium: 10, Large: 100}
	cases := []struct {
		quote float64
		want  SizeBucket
	}{
		{0, BucketSmall},
		{9.99, BucketSmall},
		{10, BucketMedium},
		{99.99, BucketMedium},
		{100, BucketLarge},
		{100000, BucketLarge},
	}
	for _, tc := range cases {
		if got := thresholds.Classify(tc.quote); got != tc.want {
			t.Errorf("Classify(%v) = %q, want %q", tc.quote, got, tc.want)
		}
	}
}

func TestSizeThresholdsValidRejectsUnusableBands(t *testing.T) {
	cases := []struct {
		name       string
		thresholds SizeThresholds
		want       bool
	}{
		{"ordered", SizeThresholds{Medium: 10, Large: 100}, true},
		{"zero medium", SizeThresholds{Medium: 0, Large: 100}, false},
		{"negative medium", SizeThresholds{Medium: -1, Large: 100}, false},
		{"large below medium", SizeThresholds{Medium: 100, Large: 10}, false},
		{"large equals medium", SizeThresholds{Medium: 10, Large: 10}, false},
	}
	for _, tc := range cases {
		if got := tc.thresholds.Valid(); got != tc.want {
			t.Errorf("%s: Valid() = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestNewRegistryRejectsInvalidVenueSets(t *testing.T) {
	if _, err := NewRegistry(); err == nil {
		t.Error("empty registry should be rejected")
	}
	if _, err := NewRegistry(validStub("mexc"), validStub("mexc")); err == nil {
		t.Error("duplicate venue id should be rejected")
	}
	if _, err := NewRegistry(validStub("")); err == nil {
		t.Error("empty venue id should be rejected")
	}
	bad := stubVenue{id: "broken", thresholds: SizeThresholds{Medium: 100, Large: 10}}
	if _, err := NewRegistry(bad); err == nil {
		t.Error("venue with unordered thresholds should be rejected")
	}
}

func TestRegistryResolvesVenuesAndDefaultsToTheFirst(t *testing.T) {
	registry, err := NewRegistry(validStub("mexc"), validStub("kraken"))
	if err != nil {
		t.Fatalf("NewRegistry: %v", err)
	}
	if got := registry.Default().ID(); got != "mexc" {
		t.Errorf("Default() = %q, want the first registered venue", got)
	}
	if _, ok := registry.ByID("kraken"); !ok {
		t.Error("ByID(kraken) should resolve a registered venue")
	}
	if _, ok := registry.ByID("nope"); ok {
		t.Error("ByID(nope) should not resolve an unregistered venue")
	}
	if got, want := registry.IDs(), []string{"kraken", "mexc"}; len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("IDs() = %v, want %v sorted", got, want)
	}
	// All must hand back a copy: a caller mutating it must not corrupt the
	// registry every request reads.
	all := registry.All()
	all[0] = validStub("tampered")
	if registry.Default().ID() != "mexc" {
		t.Error("mutating the All() slice changed the registry")
	}
}

func TestValidSizeBucket(t *testing.T) {
	for _, bucket := range OrderedSizeBuckets {
		if !ValidSizeBucket(string(bucket)) {
			t.Errorf("ValidSizeBucket(%q) = false, want true", bucket)
		}
	}
	if ValidSizeBucket("enormous") {
		t.Error("ValidSizeBucket(enormous) = true, want false")
	}
}

func TestMEXCFetchTradesNormalizesTheTape(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != mexcTradesPath {
			t.Errorf("path = %q, want %q", r.URL.Path, mexcTradesPath)
		}
		if got := r.URL.Query().Get("symbol"); got != MEXCSymbol {
			t.Errorf("symbol = %q, want %q", got, MEXCSymbol)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[
			{"id":null,"price":"0.80","qty":"10","quoteQty":"8.0","time":1787139418000,"isBuyerMaker":true},
			{"id":null,"price":"0.81","qty":"5","quoteQty":"4.05","time":1787139419000,"isBuyerMaker":false}
		]`))
	}))
	defer server.Close()

	client, err := NewMEXCClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewMEXCClient: %v", err)
	}
	trades, err := client.FetchTrades(context.Background())
	if err != nil {
		t.Fatalf("FetchTrades: %v", err)
	}
	if len(trades) != 2 {
		t.Fatalf("trades = %d, want 2", len(trades))
	}
	// isBuyerMaker true means the resting side was the buyer, so the taker
	// (the aggressor this feed reports) sold into it.
	if trades[0].AggressorSide != "sell" || trades[1].AggressorSide != "buy" {
		t.Errorf("aggressor sides = %q/%q, want sell/buy", trades[0].AggressorSide, trades[1].AggressorSide)
	}
	if trades[0].ID == "" || trades[0].ID == trades[1].ID {
		t.Errorf("synthetic ids must be present and distinct, got %q and %q", trades[0].ID, trades[1].ID)
	}
}

func TestMEXCFetchTradesRejectsNonArrayPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":429,"msg":"rate limited"}`))
	}))
	defer server.Close()

	client, err := NewMEXCClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewMEXCClient: %v", err)
	}
	if _, err := client.FetchTrades(context.Background()); err == nil {
		t.Fatal("an error payload must not decode as an empty tape")
	}
}

func TestMEXCClientSatisfiesVenue(t *testing.T) {
	client, err := NewMEXCClient(MEXCAPIBaseURL, nil)
	if err != nil {
		t.Fatalf("NewMEXCClient: %v", err)
	}
	var venue Venue = client
	if venue.ID() != MEXCVenueID || venue.Symbol() != MEXCSymbol || venue.QuoteAsset() != MEXCQuoteAsset {
		t.Errorf("venue identity = %q/%q/%q", venue.ID(), venue.Symbol(), venue.QuoteAsset())
	}
	if !venue.SizeThresholds().Valid() {
		t.Error("shipped MEXC thresholds must be valid")
	}
	// The registry the process actually serves must build.
	registry, err := DefaultRegistry()
	if err != nil {
		t.Fatalf("DefaultRegistry: %v", err)
	}
	if registry.Default().ID() != MEXCVenueID {
		t.Errorf("default venue = %q, want %q", registry.Default().ID(), MEXCVenueID)
	}
}

func TestOrderedSizeBucketsIsStableForJSONConsumers(t *testing.T) {
	encoded, err := json.Marshal(OrderedSizeBuckets)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if want := `["large","medium","small"]`; string(encoded) != want {
		t.Errorf("OrderedSizeBuckets = %s, want %s", encoded, want)
	}
}
