package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// newLimitedRouter wires PerIPRateLimit in front of a trivial 200 handler
// so the limiter is exercised through its only exported surface: the
// middleware behaviour observable over HTTP.
func newLimitedRouter(burst, refill float64, window time.Duration) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/limited", PerIPRateLimit(burst, refill, window), func(c *gin.Context) {
		c.Status(http.StatusOK)
	})
	return router
}

// doLimited issues one request attributed to ip (via CF-Connecting-IP,
// the header production traffic carries) and returns the recorder.
func doLimited(router *gin.Engine, ip string) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/limited", nil)
	req.Header.Set("CF-Connecting-IP", ip)
	router.ServeHTTP(w, req)
	return w
}

func TestPerIPRateLimitBurstThenReject(t *testing.T) {
	router := newLimitedRouter(2, 2, time.Minute)

	for i := 0; i < 2; i++ {
		if w := doLimited(router, "203.0.113.1"); w.Code != http.StatusOK {
			t.Fatalf("request %d within burst: status = %d, want 200", i+1, w.Code)
		}
	}
	w := doLimited(router, "203.0.113.1")
	if w.Code != http.StatusTooManyRequests {
		t.Fatalf("request past burst: status = %d, want 429", w.Code)
	}
	if want := `{"error":"rate limit exceeded"}`; w.Body.String() != want {
		t.Errorf("429 body = %s, want %s", w.Body.String(), want)
	}
}

func TestPerIPRateLimitRefill(t *testing.T) {
	// burst 1, refilling 1 token per 100ms window.
	router := newLimitedRouter(1, 1, 100*time.Millisecond)

	if w := doLimited(router, "203.0.113.2"); w.Code != http.StatusOK {
		t.Fatalf("first request: status = %d, want 200", w.Code)
	}
	if w := doLimited(router, "203.0.113.2"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("immediate second request: status = %d, want 429", w.Code)
	}

	time.Sleep(150 * time.Millisecond) // > one window, bucket refills to full

	if w := doLimited(router, "203.0.113.2"); w.Code != http.StatusOK {
		t.Fatalf("post-refill request: status = %d, want 200", w.Code)
	}
}

func TestPerIPRateLimitBucketsAreIsolatedPerIP(t *testing.T) {
	router := newLimitedRouter(1, 1, time.Minute)

	if w := doLimited(router, "203.0.113.3"); w.Code != http.StatusOK {
		t.Fatalf("ip A first request: status = %d, want 200", w.Code)
	}
	if w := doLimited(router, "203.0.113.3"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("ip A second request: status = %d, want 429", w.Code)
	}
	// A different IP must get a fresh bucket.
	if w := doLimited(router, "203.0.113.4"); w.Code != http.StatusOK {
		t.Fatalf("ip B first request: status = %d, want 200", w.Code)
	}
}

func TestPerIPRateLimitIdleEviction(t *testing.T) {
	// refill 0: tokens never come back except through idle eviction
	// (janitor delete, or the in-handler reset for entries idle past
	// evictAfter = 10 * window). window 20ms puts evictAfter at 200ms.
	router := newLimitedRouter(1, 0, 20*time.Millisecond)

	if w := doLimited(router, "203.0.113.5"); w.Code != http.StatusOK {
		t.Fatalf("first request: status = %d, want 200", w.Code)
	}
	if w := doLimited(router, "203.0.113.5"); w.Code != http.StatusTooManyRequests {
		t.Fatalf("second request: status = %d, want 429", w.Code)
	}

	time.Sleep(300 * time.Millisecond) // idle > evictAfter

	if w := doLimited(router, "203.0.113.5"); w.Code != http.StatusOK {
		t.Fatalf("post-eviction request: status = %d, want 200 (bucket should be reset)", w.Code)
	}
}
