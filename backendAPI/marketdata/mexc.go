// Package marketdata fetches and normalizes public exchange market data.
package marketdata

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
)

const (
	MEXCVenue      = "MEXC"
	MEXCVenueID    = "mexc"
	MEXCSymbol     = "QRLUSDT"
	MEXCQuoteAsset = "USDT"
	MEXCAPIBaseURL = "https://api.mexc.com"

	// MEXC size bands in USDT notional. Sized from the observed QRLUSDT
	// tape (median ~8, p90 ~49, max ~333 USDT per print), which puts
	// roughly half of prints in small, a third in medium, and a tenth in
	// large. Retune these if the pair's liquidity profile changes;
	// historical rows keep the band they were classified into at write
	// time, so a retune is not retroactive.
	mexcMediumThresholdUSDT = 10
	mexcLargeThresholdUSDT  = 100

	mexcDepthPath  = "/api/v3/depth"
	mexcTradesPath = "/api/v3/trades"
	mexcTickerPath = "/api/v3/ticker/24hr"

	marketDataLimit      = 100
	maxDepthBodyBytes    = 128 << 10
	maxTradesBodyBytes   = 256 << 10
	maxTickerBodyBytes   = 32 << 10
	maxDecimalCharacters = 128
	maxTradeIDCharacters = 128

	// The collector asks for a deeper tape than the order-book view needs.
	// MEXC caps its public trade buffer well below this (about 200 prints
	// for QRLUSDT, roughly 8 hours at current activity), so the request is
	// a ceiling rather than an expectation, and the extra depth is what
	// lets the collector miss polls without losing trades.
	tradeTapeLimit    = 1000
	maxTapeBodyBytes  = 1 << 20
	tradeTapeMaxItems = 2000
)

var decimalPattern = regexp.MustCompile(`^-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?$`)

// PriceLevel is one aggregated order-book level.
type PriceLevel struct {
	Price    string `json:"price"`
	Quantity string `json:"quantity"`
}

// Trade is one recent public trade. Time is a Unix timestamp in milliseconds.
// AggressorSide describes the taker side inferred from MEXC's isBuyerMaker flag.
type Trade struct {
	ID            string `json:"id"`
	Price         string `json:"price"`
	Quantity      string `json:"quantity"`
	QuoteQuantity string `json:"quoteQuantity"`
	Time          int64  `json:"time"`
	AggressorSide string `json:"aggressorSide"`
}

// Ticker contains the 24-hour fields needed by the order-book visualization.
// QuoteVolume is nullable because MEXC documents and occasionally returns null.
type Ticker struct {
	High          string  `json:"high"`
	Low           string  `json:"low"`
	Last          string  `json:"last"`
	Change        string  `json:"change"`
	ChangePercent string  `json:"changePercent"`
	BaseVolume    string  `json:"baseVolume"`
	QuoteVolume   *string `json:"quoteVolume"`
}

// OrderBookSnapshot is the fixed QRLUSDT market snapshot exposed by the API.
type OrderBookSnapshot struct {
	Venue        string       `json:"venue"`
	Symbol       string       `json:"symbol"`
	FetchedAt    time.Time    `json:"fetchedAt"`
	LastUpdateID uint64       `json:"lastUpdateId"`
	Bids         []PriceLevel `json:"bids"`
	Asks         []PriceLevel `json:"asks"`
	RecentTrades []Trade      `json:"recentTrades"`
	Ticker       Ticker       `json:"ticker"`
}

// MEXCClient fetches the three fixed public MEXC endpoints used by the route.
// The base URL is constructor-injected for local httptest servers. Production
// always constructs it with MEXCAPIBaseURL, and callers cannot select a URL or
// symbol through the public HTTP endpoint.
type MEXCClient struct {
	baseURL    *url.URL
	httpClient *http.Client
}

// NewBoundedHTTPClient returns the production HTTP client. Every phase has a
// finite budget, response bodies have separate byte caps in fetchJSON, and
// redirects are rejected so the fixed upstream cannot redirect requests to a
// different host.
func NewBoundedHTTPClient() *http.Client {
	transport := &http.Transport{
		Proxy:                  http.ProxyFromEnvironment,
		DialContext:            (&net.Dialer{Timeout: 2 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:      true,
		MaxIdleConns:           10,
		MaxIdleConnsPerHost:    5,
		MaxConnsPerHost:        6,
		IdleConnTimeout:        60 * time.Second,
		TLSHandshakeTimeout:    2 * time.Second,
		ResponseHeaderTimeout:  3 * time.Second,
		MaxResponseHeaderBytes: 64 << 10,
		ExpectContinueTimeout:  time.Second,
	}
	return &http.Client{
		Transport: transport,
		Timeout:   4 * time.Second,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// NewMEXCClient constructs a client for a trusted base URL. A supplied HTTP
// client is shallow-copied so redirect rejection can be enforced without
// mutating the caller's client. Pass nil to use the bounded production client.
func NewMEXCClient(baseURL string, httpClient *http.Client) (*MEXCClient, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return nil, fmt.Errorf("parse MEXC base URL: %w", err)
	}
	if (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil ||
		(u.Path != "" && u.Path != "/") || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("MEXC base URL must be an absolute HTTP URL without credentials, path, query, or fragment")
	}
	u.Path = ""

	if httpClient == nil {
		httpClient = NewBoundedHTTPClient()
	} else {
		clientCopy := *httpClient
		clientCopy.CheckRedirect = func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		}
		httpClient = &clientCopy
	}

	return &MEXCClient{baseURL: u, httpClient: httpClient}, nil
}

// ID, Name, Symbol, QuoteAsset, and SizeThresholds implement Venue. They are
// constants rather than configuration: the venue a client reads is selected
// from the registry, and nothing about a venue is settable per request.
func (c *MEXCClient) ID() string         { return MEXCVenueID }
func (c *MEXCClient) Name() string       { return MEXCVenue }
func (c *MEXCClient) Symbol() string     { return MEXCSymbol }
func (c *MEXCClient) QuoteAsset() string { return MEXCQuoteAsset }

func (c *MEXCClient) SizeThresholds() SizeThresholds {
	return SizeThresholds{Medium: mexcMediumThresholdUSDT, Large: mexcLargeThresholdUSDT}
}

// FetchTrades returns the venue's public trade tape, deepest first request
// the upstream will serve. This is the collector's entry point; the
// order-book view uses the shallower tape embedded in FetchOrderBook.
func (c *MEXCClient) FetchTrades(ctx context.Context) ([]Trade, error) {
	var raw []mexcTradeResponse
	if err := c.fetchJSON(ctx, mexcTradesPath, tradeTapeLimit, maxTapeBodyBytes, &raw); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, errors.New("MEXC trades response must be an array")
	}
	return normalizeTrades(raw, tradeTapeMaxItems)
}

type mexcDepthResponse struct {
	LastUpdateID *uint64    `json:"lastUpdateId"`
	Bids         [][]string `json:"bids"`
	Asks         [][]string `json:"asks"`
}

type mexcTradeResponse struct {
	ID           json.RawMessage `json:"id"`
	Price        string          `json:"price"`
	Quantity     string          `json:"qty"`
	QuoteQty     string          `json:"quoteQty"`
	Time         int64           `json:"time"`
	IsBuyerMaker *bool           `json:"isBuyerMaker"`
}

type mexcTickerResponse struct {
	Symbol             string  `json:"symbol"`
	PriceChange        string  `json:"priceChange"`
	PriceChangePercent string  `json:"priceChangePercent"`
	LastPrice          string  `json:"lastPrice"`
	HighPrice          string  `json:"highPrice"`
	LowPrice           string  `json:"lowPrice"`
	Volume             string  `json:"volume"`
	QuoteVolume        *string `json:"quoteVolume"`
}

// FetchOrderBook fetches depth, recent trades, and the 24-hour ticker in
// parallel. The request context controls all three upstream requests.
func (c *MEXCClient) FetchOrderBook(ctx context.Context) (OrderBookSnapshot, error) {
	var depth mexcDepthResponse
	var trades []mexcTradeResponse
	var ticker mexcTickerResponse

	g, fetchCtx := errgroup.WithContext(ctx)
	g.Go(func() error {
		return c.fetchJSON(fetchCtx, mexcDepthPath, marketDataLimit, maxDepthBodyBytes, &depth)
	})
	g.Go(func() error {
		return c.fetchJSON(fetchCtx, mexcTradesPath, marketDataLimit, maxTradesBodyBytes, &trades)
	})
	g.Go(func() error {
		return c.fetchJSON(fetchCtx, mexcTickerPath, 0, maxTickerBodyBytes, &ticker)
	})
	if err := g.Wait(); err != nil {
		return OrderBookSnapshot{}, err
	}
	if depth.LastUpdateID == nil || depth.Bids == nil || depth.Asks == nil {
		return OrderBookSnapshot{}, errors.New("MEXC depth response is missing lastUpdateId, bids, or asks")
	}
	if trades == nil {
		return OrderBookSnapshot{}, errors.New("MEXC trades response must be an array")
	}

	bids, err := normalizeLevels("bids", depth.Bids, true)
	if err != nil {
		return OrderBookSnapshot{}, err
	}
	asks, err := normalizeLevels("asks", depth.Asks, false)
	if err != nil {
		return OrderBookSnapshot{}, err
	}
	normalizedTrades, err := normalizeTrades(trades, marketDataLimit)
	if err != nil {
		return OrderBookSnapshot{}, err
	}
	normalizedTicker, err := normalizeTicker(ticker)
	if err != nil {
		return OrderBookSnapshot{}, err
	}

	return OrderBookSnapshot{
		Venue:        MEXCVenue,
		Symbol:       MEXCSymbol,
		FetchedAt:    time.Now().UTC(),
		LastUpdateID: *depth.LastUpdateID,
		Bids:         bids,
		Asks:         asks,
		RecentTrades: normalizedTrades,
		Ticker:       normalizedTicker,
	}, nil
}

func (c *MEXCClient) fetchJSON(
	ctx context.Context,
	endpoint string,
	limit int,
	maxBodyBytes int64,
	target interface{},
) error {
	u := *c.baseURL
	u.Path = endpoint
	query := u.Query()
	query.Set("symbol", MEXCSymbol)
	if limit > 0 {
		query.Set("limit", fmt.Sprintf("%d", limit))
	}
	u.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("build MEXC %s request: %w", endpoint, err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "ZondScan-MarketData/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("fetch MEXC %s: %w", endpoint, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("fetch MEXC %s: upstream returned HTTP %d", endpoint, resp.StatusCode)
	}
	if resp.ContentLength > maxBodyBytes {
		return fmt.Errorf("fetch MEXC %s: response exceeds %d bytes", endpoint, maxBodyBytes)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes+1))
	if err != nil {
		return fmt.Errorf("read MEXC %s response: %w", endpoint, err)
	}
	if int64(len(body)) > maxBodyBytes {
		return fmt.Errorf("fetch MEXC %s: response exceeds %d bytes", endpoint, maxBodyBytes)
	}
	if err := json.Unmarshal(body, target); err != nil {
		return fmt.Errorf("decode MEXC %s response: %w", endpoint, err)
	}
	return nil
}

func normalizeLevels(name string, raw [][]string, descending bool) ([]PriceLevel, error) {
	if len(raw) > marketDataLimit {
		return nil, fmt.Errorf("MEXC %s response exceeds %d levels", name, marketDataLimit)
	}
	levels := make([]PriceLevel, 0, len(raw))
	seenPrices := make(map[string]struct{}, len(raw))
	for i, level := range raw {
		if len(level) != 2 {
			return nil, fmt.Errorf("MEXC %s level %d must contain price and quantity", name, i)
		}
		price, err := normalizeDecimal(level[0], false)
		if err != nil {
			return nil, fmt.Errorf("MEXC %s level %d price: %w", name, i, err)
		}
		quantity, err := normalizeDecimal(level[1], false)
		if err != nil {
			return nil, fmt.Errorf("MEXC %s level %d quantity: %w", name, i, err)
		}
		if _, exists := seenPrices[price]; exists {
			return nil, fmt.Errorf("MEXC %s contains duplicate price %s", name, price)
		}
		seenPrices[price] = struct{}{}
		levels = append(levels, PriceLevel{Price: price, Quantity: quantity})
	}
	sort.Slice(levels, func(i, j int) bool {
		comparison := compareUnsignedDecimals(levels[i].Price, levels[j].Price)
		if descending {
			return comparison > 0
		}
		return comparison < 0
	})
	return levels, nil
}

func normalizeTrades(raw []mexcTradeResponse, maxEntries int) ([]Trade, error) {
	if len(raw) > maxEntries {
		return nil, fmt.Errorf("MEXC trades response exceeds %d entries", maxEntries)
	}
	trades := make([]Trade, 0, len(raw))
	syntheticOccurrences := make(map[string]int)
	for i, source := range raw {
		id, err := normalizeTradeID(source.ID)
		if err != nil {
			return nil, fmt.Errorf("MEXC trade %d id: %w", i, err)
		}
		price, err := normalizeDecimal(source.Price, false)
		if err != nil {
			return nil, fmt.Errorf("MEXC trade %d price: %w", i, err)
		}
		quantity, err := normalizeDecimal(source.Quantity, false)
		if err != nil {
			return nil, fmt.Errorf("MEXC trade %d quantity: %w", i, err)
		}
		quoteQuantity, err := normalizeDecimal(source.QuoteQty, false)
		if err != nil {
			return nil, fmt.Errorf("MEXC trade %d quote quantity: %w", i, err)
		}
		if source.Time <= 0 {
			return nil, fmt.Errorf("MEXC trade %d has an invalid timestamp", i)
		}
		if source.IsBuyerMaker == nil {
			return nil, fmt.Errorf("MEXC trade %d is missing isBuyerMaker", i)
		}
		side := "buy"
		if *source.IsBuyerMaker {
			side = "sell"
		}
		if id == "" {
			baseID := syntheticTradeID(source.Time, price, quantity, quoteQuantity, side)
			occurrence := syntheticOccurrences[baseID]
			syntheticOccurrences[baseID] = occurrence + 1
			id = fmt.Sprintf("%s-%d", baseID, occurrence)
		}
		trades = append(trades, Trade{
			ID:            id,
			Price:         price,
			Quantity:      quantity,
			QuoteQuantity: quoteQuantity,
			Time:          source.Time,
			AggressorSide: side,
		})
	}
	return trades, nil
}

func compareUnsignedDecimals(a, b string) int {
	aParts := strings.SplitN(a, ".", 2)
	bParts := strings.SplitN(b, ".", 2)
	if len(aParts[0]) != len(bParts[0]) {
		if len(aParts[0]) < len(bParts[0]) {
			return -1
		}
		return 1
	}
	if aParts[0] != bParts[0] {
		return strings.Compare(aParts[0], bParts[0])
	}
	aFraction := ""
	bFraction := ""
	if len(aParts) == 2 {
		aFraction = aParts[1]
	}
	if len(bParts) == 2 {
		bFraction = bParts[1]
	}
	width := len(aFraction)
	if len(bFraction) > width {
		width = len(bFraction)
	}
	aFraction += strings.Repeat("0", width-len(aFraction))
	bFraction += strings.Repeat("0", width-len(bFraction))
	return strings.Compare(aFraction, bFraction)
}

func normalizeTicker(source mexcTickerResponse) (Ticker, error) {
	if source.Symbol != MEXCSymbol {
		return Ticker{}, fmt.Errorf("MEXC ticker returned symbol %q", source.Symbol)
	}
	values := []struct {
		name   string
		raw    string
		signed bool
	}{
		{name: "high price", raw: source.HighPrice},
		{name: "low price", raw: source.LowPrice},
		{name: "last price", raw: source.LastPrice},
		{name: "price change", raw: source.PriceChange, signed: true},
		{name: "price change percent", raw: source.PriceChangePercent, signed: true},
		{name: "base volume", raw: source.Volume},
	}
	normalized := make([]string, len(values))
	for i, value := range values {
		v, err := normalizeDecimal(value.raw, value.signed)
		if err != nil {
			return Ticker{}, fmt.Errorf("MEXC ticker %s: %w", value.name, err)
		}
		normalized[i] = v
	}

	var quoteVolume *string
	if source.QuoteVolume != nil {
		v, err := normalizeDecimal(*source.QuoteVolume, false)
		if err != nil {
			return Ticker{}, fmt.Errorf("MEXC ticker quote volume: %w", err)
		}
		quoteVolume = &v
	}

	return Ticker{
		High:          normalized[0],
		Low:           normalized[1],
		Last:          normalized[2],
		Change:        normalized[3],
		ChangePercent: normalized[4],
		BaseVolume:    normalized[5],
		QuoteVolume:   quoteVolume,
	}, nil
}

func normalizeTradeID(raw json.RawMessage) (string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", nil
	}
	var id string
	if trimmed[0] == '"' {
		if err := json.Unmarshal(raw, &id); err != nil {
			return "", errors.New("invalid string")
		}
	} else {
		if strings.HasPrefix(trimmed, "-") || !allASCIIDigits(trimmed) {
			return "", errors.New("must be a non-negative integer, string, or null")
		}
		id = trimmed
	}
	if id == "" || len(id) > maxTradeIDCharacters {
		return "", fmt.Errorf("must contain between 1 and %d characters", maxTradeIDCharacters)
	}
	return id, nil
}

func syntheticTradeID(timestamp int64, price, quantity, quoteQuantity, side string) string {
	payload := fmt.Sprintf("%d\x00%s\x00%s\x00%s\x00%s", timestamp, price, quantity, quoteQuantity, side)
	sum := sha256.Sum256([]byte(payload))
	return fmt.Sprintf("synthetic-%x", sum[:12])
}

func normalizeDecimal(raw string, signed bool) (string, error) {
	if raw == "" || len(raw) > maxDecimalCharacters || !decimalPattern.MatchString(raw) {
		return "", errors.New("invalid decimal string")
	}
	negative := strings.HasPrefix(raw, "-")
	if negative && !signed {
		return "", errors.New("value must be non-negative")
	}
	if negative {
		raw = raw[1:]
	}
	parts := strings.SplitN(raw, ".", 2)
	integerPart := strings.TrimLeft(parts[0], "0")
	if integerPart == "" {
		integerPart = "0"
	}
	fractionalPart := ""
	if len(parts) == 2 {
		fractionalPart = strings.TrimRight(parts[1], "0")
	}
	normalized := integerPart
	if fractionalPart != "" {
		normalized += "." + fractionalPart
	}
	if negative && normalized != "0" {
		normalized = "-" + normalized
	}
	return normalized, nil
}

func allASCIIDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
