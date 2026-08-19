package marketdata

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestMEXCClientFetchOrderBook(t *testing.T) {
	var mu sync.Mutex
	requestCounts := make(map[string]int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		requestCounts[r.URL.Path]++
		mu.Unlock()

		if got := r.URL.Query().Get("symbol"); got != MEXCSymbol {
			t.Errorf("symbol query = %q, want %q", got, MEXCSymbol)
		}
		if got := r.Header.Get("Accept"); got != "application/json" {
			t.Errorf("Accept header = %q, want application/json", got)
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v3/depth":
			if got := r.URL.Query().Get("limit"); got != "100" {
				t.Errorf("depth limit = %q, want 100", got)
			}
			fmt.Fprint(w, `{
                    "lastUpdateId":42,
                    "bids":[["0.74000","3.000"],["0.75000","10.500"]],
                    "asks":[["0.77000","4.000"],["0.76000","2.000"]]
                }`)
		case "/api/v3/trades":
			if got := r.URL.Query().Get("limit"); got != "100" {
				t.Errorf("trades limit = %q, want 100", got)
			}
			fmt.Fprint(w, `[
                    {
                      "id":null,"price":"0.75500","qty":"1.2500","quoteQty":"0.9437500",
                      "time":1786290460978,"isBuyerMaker":true
                    },
                    {
                      "id":null,"price":"0.75500","qty":"1.2500","quoteQty":"0.9437500",
                      "time":1786290460978,"isBuyerMaker":true
                    },
                    {
                      "id":"trade-2","price":"0.75600","qty":"2.000","quoteQty":"1.512000",
                      "time":1786290461978,"isBuyerMaker":false
                    }
                ]`)
		case "/api/v3/ticker/24hr":
			if got := r.URL.Query().Get("limit"); got != "" {
				t.Errorf("ticker limit = %q, want empty", got)
			}
			fmt.Fprint(w, `{
                    "symbol":"QRLUSDT","priceChange":"-0.00500","priceChangePercent":"-0.00660",
                    "lastPrice":"0.75500","highPrice":"0.80000","lowPrice":"0.70000",
                    "volume":"12345.6700","quoteVolume":"9320.98000"
                }`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client, err := NewMEXCClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewMEXCClient() error = %v", err)
	}
	started := time.Now().UTC()
	snapshot, err := client.FetchOrderBook(context.Background())
	if err != nil {
		t.Fatalf("FetchOrderBook() error = %v", err)
	}

	if snapshot.Venue != MEXCVenue || snapshot.Symbol != MEXCSymbol {
		t.Fatalf("venue/symbol = %q/%q, want %q/%q", snapshot.Venue, snapshot.Symbol, MEXCVenue, MEXCSymbol)
	}
	if snapshot.FetchedAt.Before(started) || snapshot.FetchedAt.After(time.Now().UTC()) {
		t.Fatalf("fetchedAt = %s, want current UTC time", snapshot.FetchedAt)
	}
	if snapshot.LastUpdateID != 42 {
		t.Errorf("lastUpdateId = %d, want 42", snapshot.LastUpdateID)
	}
	if len(snapshot.Bids) != 2 ||
		snapshot.Bids[0] != (PriceLevel{Price: "0.75", Quantity: "10.5"}) ||
		snapshot.Bids[1] != (PriceLevel{Price: "0.74", Quantity: "3"}) {
		t.Errorf("bids = %#v, want normalized descending levels", snapshot.Bids)
	}
	if len(snapshot.Asks) != 2 ||
		snapshot.Asks[0] != (PriceLevel{Price: "0.76", Quantity: "2"}) ||
		snapshot.Asks[1] != (PriceLevel{Price: "0.77", Quantity: "4"}) {
		t.Errorf("asks = %#v, want normalized ascending levels", snapshot.Asks)
	}
	if len(snapshot.RecentTrades) != 3 {
		t.Fatalf("recentTrades length = %d, want 3", len(snapshot.RecentTrades))
	}
	if got := snapshot.RecentTrades[0]; got.ID == "" ||
		got.AggressorSide != "sell" ||
		got.Price != "0.755" ||
		got.Quantity != "1.25" ||
		got.QuoteQuantity != "0.94375" {
		t.Errorf("recentTrades[0] = %#v, want synthetic id and normalized sell trade", got)
	}
	if snapshot.RecentTrades[0].ID == snapshot.RecentTrades[1].ID {
		t.Errorf("duplicate null-id trades received the same synthetic id %q", snapshot.RecentTrades[0].ID)
	}
	if got := snapshot.RecentTrades[2]; got.ID != "trade-2" || got.AggressorSide != "buy" {
		t.Errorf("recentTrades[2] = %#v, want upstream id and buy aggressor", got)
	}
	if snapshot.Ticker.High != "0.8" || snapshot.Ticker.Low != "0.7" || snapshot.Ticker.Last != "0.755" ||
		snapshot.Ticker.Change != "-0.005" ||
		snapshot.Ticker.ChangePercent != "-0.0066" ||
		snapshot.Ticker.BaseVolume != "12345.67" ||
		snapshot.Ticker.QuoteVolume == nil || *snapshot.Ticker.QuoteVolume != "9320.98" {
		t.Errorf("ticker = %#v, want normalized decimal strings", snapshot.Ticker)
	}

	mu.Lock()
	defer mu.Unlock()
	for _, path := range []string{"/api/v3/depth", "/api/v3/trades", "/api/v3/ticker/24hr"} {
		if requestCounts[path] != 1 {
			t.Errorf("request count for %s = %d, want 1", path, requestCounts[path])
		}
	}
}

func TestMEXCClientRejectsInvalidUpstreamResponses(t *testing.T) {
	tests := []struct {
		name        string
		badPath     string
		badResponse func(http.ResponseWriter)
		wantError   string
	}{
		{
			name:    "non-2xx",
			badPath: "/api/v3/depth",
			badResponse: func(w http.ResponseWriter) {
				w.WriteHeader(http.StatusTooManyRequests)
				fmt.Fprint(w, `{"code":429}`)
			},
			wantError: "upstream returned HTTP 429",
		},
		{
			name:    "malformed JSON",
			badPath: "/api/v3/depth",
			badResponse: func(w http.ResponseWriter) {
				fmt.Fprint(w, `{"lastUpdateId":`)
			},
			wantError: "decode MEXC /api/v3/depth response",
		},
		{
			name:    "missing depth fields",
			badPath: "/api/v3/depth",
			badResponse: func(w http.ResponseWriter) {
				fmt.Fprint(w, `{}`)
			},
			wantError: "depth response is missing",
		},
		{
			name:    "oversized body",
			badPath: "/api/v3/ticker/24hr",
			badResponse: func(w http.ResponseWriter) {
				fmt.Fprint(w, strings.Repeat(" ", maxTickerBodyBytes+1))
			},
			wantError: "response exceeds",
		},
		{
			name:    "invalid decimal",
			badPath: "/api/v3/depth",
			badResponse: func(w http.ResponseWriter) {
				fmt.Fprint(w, `{"lastUpdateId":42,"bids":[["NaN","1"]],"asks":[]}`)
			},
			wantError: "invalid decimal string",
		},
		{
			name:    "too many levels",
			badPath: "/api/v3/depth",
			badResponse: func(w http.ResponseWriter) {
				levels := strings.TrimSuffix(strings.Repeat(`["0.75","1"],`, marketDataLimit+1), ",")
				fmt.Fprintf(w, `{"lastUpdateId":42,"bids":[%s],"asks":[]}`, levels)
			},
			wantError: "exceeds 100 levels",
		},
		{
			name:    "duplicate normalized level",
			badPath: "/api/v3/depth",
			badResponse: func(w http.ResponseWriter) {
				fmt.Fprint(w, `{
                    "lastUpdateId":42,
                    "bids":[["0.7500","1"],["0.75","2"]],
                    "asks":[]
                }`)
			},
			wantError: "contains duplicate price 0.75",
		},
		{
			name:    "missing maker flag",
			badPath: "/api/v3/trades",
			badResponse: func(w http.ResponseWriter) {
				fmt.Fprint(w, `[
                    {"id":null,"price":"0.755","qty":"1","quoteQty":"0.755","time":1786290460978}
                ]`)
			},
			wantError: "is missing isBuyerMaker",
		},
		{
			name:    "wrong ticker symbol",
			badPath: "/api/v3/ticker/24hr",
			badResponse: func(w http.ResponseWriter) {
				fmt.Fprint(w, validTickerJSON("BTCUSDT"))
			},
			wantError: `returned symbol "BTCUSDT"`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.URL.Path == test.badPath {
					test.badResponse(w)
					return
				}
				writeValidMEXCResponse(w, r.URL.Path)
			}))
			defer server.Close()

			client, err := NewMEXCClient(server.URL, server.Client())
			if err != nil {
				t.Fatalf("NewMEXCClient() error = %v", err)
			}
			_, err = client.FetchOrderBook(context.Background())
			if err == nil || !strings.Contains(err.Error(), test.wantError) {
				t.Fatalf("FetchOrderBook() error = %v, want substring %q", err, test.wantError)
			}
		})
	}
}

func TestMEXCClientRejectsRedirects(t *testing.T) {
	var targetCalls int
	var mu sync.Mutex
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		mu.Lock()
		targetCalls++
		mu.Unlock()
		writeValidMEXCResponse(w, "/api/v3/depth")
	}))
	defer target.Close()

	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+r.URL.Path, http.StatusFound)
	}))
	defer redirector.Close()

	client, err := NewMEXCClient(redirector.URL, redirector.Client())
	if err != nil {
		t.Fatalf("NewMEXCClient() error = %v", err)
	}
	_, err = client.FetchOrderBook(context.Background())
	if err == nil || !strings.Contains(err.Error(), "upstream returned HTTP 302") {
		t.Fatalf("FetchOrderBook() error = %v, want rejected redirect", err)
	}
	mu.Lock()
	defer mu.Unlock()
	if targetCalls != 0 {
		t.Fatalf("redirect target calls = %d, want 0", targetCalls)
	}
}

func TestMEXCClientHonorsContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	client, err := NewMEXCClient(server.URL, server.Client())
	if err != nil {
		t.Fatalf("NewMEXCClient() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err = client.FetchOrderBook(ctx)
	if err == nil || (!errors.Is(err, context.DeadlineExceeded) && !errors.Is(err, context.Canceled)) {
		t.Fatalf("FetchOrderBook() error = %v, want context cancellation", err)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("FetchOrderBook() took %s after cancellation, want under 1s", elapsed)
	}
}

func writeValidMEXCResponse(w http.ResponseWriter, path string) {
	switch path {
	case "/api/v3/depth":
		fmt.Fprint(w, `{"lastUpdateId":42,"bids":[["0.75","1"]],"asks":[["0.76","2"]]}`)
	case "/api/v3/trades":
		fmt.Fprint(w, `[{"id":null,"price":"0.755","qty":"1","quoteQty":"0.755","time":1786290460978,"isBuyerMaker":false}]`)
	case "/api/v3/ticker/24hr":
		fmt.Fprint(w, validTickerJSON(MEXCSymbol))
	default:
		http.Error(w, "unexpected path", http.StatusNotFound)
	}
}

func validTickerJSON(symbol string) string {
	return fmt.Sprintf(`{
        "symbol":%q,"priceChange":"0.005","priceChangePercent":"0.0066",
        "lastPrice":"0.755","highPrice":"0.8","lowPrice":"0.7",
        "volume":"100","quoteVolume":"75"
    }`, symbol)
}
