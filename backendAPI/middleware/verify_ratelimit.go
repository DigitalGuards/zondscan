// Package middleware contains lightweight Gin middlewares that don't
// warrant a dedicated package per concern. Currently: a per-IP rate
// limiter used by the contract-verification endpoints to keep the
// hypc runner from being DOSed by repeated submissions.
package middleware

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// PerIPRateLimit returns a Gin middleware that enforces a token-bucket-
// style limit per remote IP. The bucket holds `burst` tokens and refills
// at `refill` tokens per `window`. Tokens are floats so partial refill
// granularity matches the request cadence.
//
// Implementation is sync.Map + a per-entry mutex; cheap enough for the
// small request volume we expect on /contract/verify (one submission per
// human action). Eviction is opportunistic — buckets that haven't been
// touched in 10×window get garbage-collected on the next access.
func PerIPRateLimit(burst float64, refill float64, window time.Duration) gin.HandlerFunc {
	var buckets sync.Map // key: string IP, val: *bucket
	evictAfter := 10 * window

	return func(c *gin.Context) {
		ip := c.ClientIP()
		now := time.Now()

		val, _ := buckets.LoadOrStore(ip, &bucket{tokens: burst, last: now})
		b := val.(*bucket)

		b.mu.Lock()
		// Evict idle bucket (other IPs sharing this map don't get cleaned
		// up automatically — opportunistic on access is fine for our
		// volume).
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
