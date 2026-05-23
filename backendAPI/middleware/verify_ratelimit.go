// Package middleware contains lightweight Gin middlewares that don't
// warrant a dedicated package per concern. Currently: a per-IP rate
// limiter used by the contract-verification endpoints to keep the
// hypc runner from being DOSed by repeated submissions.
package middleware

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// realClientIP returns the best available identifier for the originating
// client. Production sits behind Cloudflare → nginx → backend, so each
// inbound connection arrives from a Cloudflare edge IP rather than the
// real client. Cloudflare strips any client-supplied `CF-Connecting-IP`
// at its edge before forwarding, so when that header is present we can
// trust it as the true client. If the header is missing we fall back to
// `c.ClientIP()`, which (with SetTrustedProxies) returns either an XFF-
// walked IP or the immediate RemoteAddr.
//
// Worst-case fallback path (nginx hit directly, no Cloudflare hop): the
// limiter buckets all requests under one key (loopback). That's a
// strictly tighter limit than the public-facing rate, not looser, so it
// degrades safely.
func realClientIP(c *gin.Context) string {
	if v := c.GetHeader("CF-Connecting-IP"); v != "" {
		// Header is a single IP, never a list. Trim to be defensive.
		if ip := strings.TrimSpace(v); ip != "" {
			return ip
		}
	}
	return c.ClientIP()
}

// PerIPRateLimit returns a Gin middleware that enforces a token-bucket-
// style limit per remote IP. The bucket holds `burst` tokens and refills
// at `refill` tokens per `window`. Tokens are floats so partial refill
// granularity matches the request cadence.
//
// Implementation is sync.Map + a per-entry mutex; cheap enough for the
// small request volume we expect on /contract/verify (one submission per
// human action). A janitor goroutine fully evicts idle buckets every
// `window` so the map doesn't grow unbounded as unique IPs hit the endpoint.
func PerIPRateLimit(burst float64, refill float64, window time.Duration) gin.HandlerFunc {
	var buckets sync.Map // key: string IP, val: *bucket
	const idleMultiplier = 10
	evictAfter := time.Duration(idleMultiplier) * window

	// Start the janitor once per middleware instance. It walks the map
	// each `window` and `buckets.Delete`s any entry untouched for longer
	// than `evictAfter`. Costs O(active IPs) per tick, which is fine, we
	// never expect more than thousands of concurrent verifiers.
	go func() {
		ticker := time.NewTicker(window)
		defer ticker.Stop()
		for now := range ticker.C {
			buckets.Range(func(k, v interface{}) bool {
				b := v.(*bucket)
				b.mu.Lock()
				idle := now.Sub(b.last)
				b.mu.Unlock()
				if idle > evictAfter {
					buckets.Delete(k)
				}
				return true
			})
		}
	}()

	return func(c *gin.Context) {
		ip := realClientIP(c)
		now := time.Now()

		val, _ := buckets.LoadOrStore(ip, &bucket{tokens: burst, last: now})
		b := val.(*bucket)

		b.mu.Lock()
		// Belt-and-braces in case a request races the janitor: if the
		// bucket was effectively dropped mid-tick, reset it.
		if now.Sub(b.last) > evictAfter {
			b.tokens = burst
		} else {
			elapsed := now.Sub(b.last).Seconds()
			b.tokens += elapsed * (refill / window.Seconds())
			if b.tokens > burst {
				b.tokens = burst
			}
		}
		b.last = now

		if b.tokens < 1 {
			b.mu.Unlock()
			c.JSON(http.StatusTooManyRequests, gin.H{"error": "rate limit exceeded"})
			c.Abort()
			return
		}
		b.tokens -= 1
		b.mu.Unlock()
		c.Next()
	}
}

type bucket struct {
	mu     sync.Mutex
	tokens float64
	last   time.Time
}
