package routes

import (
	"context"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"backendAPI/aiexplain"
	"backendAPI/middleware"

	"github.com/gin-gonic/gin"
)

// RegisterContractExplainRoute wires POST /contract/explain/:address onto
// the supplied router. Always registered — when the explainer isn't
// configured (missing EXPERT_ANALIST_CLAUDE_HAIKU), the handler returns
// 503 so the rest of the backend keeps working.
//
// Each call to Anthropic costs real money, so the route is conservatively
// rate-limited per IP. The verified-only gate inside the explainer is the
// hard floor: an attacker who somehow exhausts the rate limit STILL can't
// burn the API budget on unverified contracts (no source means we never
// reach the LLM call).
func RegisterContractExplainRoute(router *gin.Engine) {
	// 10 calls/min/IP is a deliberately tight cap. The expected usage
	// pattern is one click per contract page; cached responses don't even
	// reach this limiter because they short-circuit on the first read.
	// Bursts of 4 absorb a curious user clicking around between contracts.
	limiter := middleware.PerIPRateLimit(10, 4, time.Minute)

	router.POST("/contract/explain/:address", limiter, func(c *gin.Context) {
		e := aiexplain.Default()
		if e == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI contract explainer not configured"})
			return
		}

		addr := strings.TrimSpace(c.Param("address"))
		if !isValidAddress(addr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Q-prefixed address"})
			return
		}

		regenerate := c.Query("regenerate") == "1" || c.Query("regenerate") == "true"

		// Anthropic Haiku typically replies in ~3-8s. Give the whole
		// explain pipeline (mongo lookup + LLM + cache write) a comfortable
		// 45s deadline so the gin worker isn't tied up indefinitely.
		ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
		defer cancel()

		resp, err := e.Explain(ctx, addr, regenerate)
		if err != nil {
			// resp can be non-nil here when the LLM succeeded but the
			// cache write failed; we want to log that case but still hand
			// the result to the client.
			if resp != nil {
				log.Printf("explain: %s — cache write failed but result delivered: %v", addr, err)
				c.JSON(http.StatusOK, resp)
				return
			}
			switch {
			case errors.Is(err, aiexplain.ErrNotFound):
				c.JSON(http.StatusNotFound, gin.H{"error": "address is not a known contract"})
			case errors.Is(err, aiexplain.ErrNotVerified):
				c.JSON(http.StatusForbidden, gin.H{"error": "contract is not verified — only verified contracts can be analysed"})
			case errors.Is(err, aiexplain.ErrRegenCap):
				c.JSON(http.StatusTooManyRequests, gin.H{
					"error": "regeneration cap reached: 5 regenerations per contract per 7-day window. Try again later.",
				})
			default:
				// Log the wrapped error server-side (may include Anthropic
				// status / response details) but return a generic public
				// message — internal error strings can leak provider config
				// hints (quota messages, model id) that aren't useful to
				// API consumers.
				log.Printf("explain: %s failed: %v", addr, err)
				c.JSON(http.StatusBadGateway, gin.H{"error": "AI explanation failed; try again shortly"})
			}
			return
		}
		c.JSON(http.StatusOK, resp)
	})
}
