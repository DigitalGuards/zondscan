package routes

import (
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"backendAPI/aiexplain"
	"backendAPI/explainauth"
	"backendAPI/middleware"

	"github.com/gin-gonic/gin"
)

const maxExplainAuthorizationBodyBytes int64 = 24 << 10

type contractExplainer interface {
	Explain(context.Context, string, bool) (*aiexplain.ExplainResponse, error)
}

type regenerationAuthorizer interface {
	IssueChallenge(context.Context, string) (*explainauth.ChallengeResponse, error)
	AuthorizeRegeneration(context.Context, string, explainauth.AuthorizationRequest) error
}

// RegisterContractExplainRoute wires the challenge and explanation POST
// routes onto the supplied router. Routes stay registered when
// EXPERT_ANALIST_CLAUDE_HAIKU is absent; the handler returns 503 so the rest
// of the backend keeps working.
//
// Each call to Anthropic costs real money, so the route is conservatively
// rate-limited per IP. The verified-only gate inside the explainer is the
// hard floor: an attacker who somehow exhausts the rate limit STILL can't
// burn the API budget on unverified contracts (no source means we never
// reach the LLM call).
func RegisterContractExplainRoute(router *gin.Engine) {
	// 10 calls/min/IP is a deliberately tight cap. The expected usage
	// pattern is one click per contract page, and the cap limits both challenge
	// issuance and explanation requests from one client address.
	// Bursts of 4 absorb a curious user clicking around between contracts.
	limiter := middleware.PerIPRateLimit(10, 4, time.Minute)

	registerContractExplainRoutes(router, aiexplain.Default(), explainauth.Default(), limiter)
}

func registerContractExplainRoutes(
	router *gin.Engine,
	explainer contractExplainer,
	authorizer regenerationAuthorizer,
	middleware ...gin.HandlerFunc,
) {
	challengeHandlers := append([]gin.HandlerFunc{}, middleware...)
	challengeHandlers = append(challengeHandlers, contractExplainChallengeHandler(explainer, authorizer))
	router.POST("/contract/explain/:address/challenge", challengeHandlers...)

	explainHandlers := append([]gin.HandlerFunc{}, middleware...)
	explainHandlers = append(explainHandlers, contractExplainHandler(explainer, authorizer))
	router.POST("/contract/explain/:address", explainHandlers...)
}

func contractExplainChallengeHandler(
	explainer contractExplainer,
	authorizer regenerationAuthorizer,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		if explainer == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI contract explainer not configured"})
			return
		}
		if authorizer == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI regeneration authorization not configured"})
			return
		}

		addr := strings.TrimSpace(c.Param("address"))
		if !isValidAddress(addr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Q-prefixed address"})
			return
		}
		if c.Request.URL.RawQuery != "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "challenge route does not accept query parameters"})
			return
		}
		body, err := readBoundedBody(c, 1<<10)
		if err != nil || len(body) != 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "challenge request body must be empty"})
			return
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
		defer cancel()
		challenge, err := authorizer.IssueChallenge(ctx, addr)
		if err != nil {
			switch {
			case errors.Is(err, explainauth.ErrContractNotFound):
				c.JSON(http.StatusNotFound, gin.H{"error": "address is not a known contract"})
			case errors.Is(err, explainauth.ErrCreatorUnavailable):
				c.JSON(http.StatusConflict, gin.H{"error": "contract creator is unavailable"})
			default:
				log.Printf("explain auth: challenge issuance failed for %s: %v", addr, err)
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI regeneration authorization unavailable"})
			}
			return
		}
		c.Header("Cache-Control", "no-store")
		c.JSON(http.StatusOK, challenge)
	}
}

func contractExplainHandler(
	explainer contractExplainer,
	authorizer regenerationAuthorizer,
) gin.HandlerFunc {
	return func(c *gin.Context) {
		e := explainer
		if e == nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI contract explainer not configured"})
			return
		}

		addr := strings.TrimSpace(c.Param("address"))
		if !isValidAddress(addr) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid Q-prefixed address"})
			return
		}

		regenerate := c.Request.URL.RawQuery == "regenerate=1"
		if c.Request.URL.RawQuery != "" && !regenerate {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid explanation query"})
			return
		}

		// Anthropic Haiku typically replies in ~3-8s. Give the whole
		// explain pipeline (mongo lookup + LLM + cache write) a comfortable
		// 45s deadline so the gin worker isn't tied up indefinitely.
		ctx, cancel := context.WithTimeout(c.Request.Context(), 45*time.Second)
		defer cancel()
		if regenerate {
			if authorizer == nil {
				c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI regeneration authorization not configured"})
				return
			}
			body, err := readBoundedBody(c, maxExplainAuthorizationBodyBytes)
			if err != nil {
				var maxBytesError *http.MaxBytesError
				if errors.As(err, &maxBytesError) {
					c.JSON(http.StatusRequestEntityTooLarge, gin.H{"error": "authorization body is too large"})
				} else {
					c.JSON(http.StatusBadRequest, gin.H{"error": "invalid authorization body"})
				}
				return
			}
			request, err := explainauth.DecodeAuthorizationRequest(body)
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid authorization body"})
				return
			}
			if err := authorizer.AuthorizeRegeneration(ctx, addr, request); err != nil {
				if errors.Is(err, explainauth.ErrUnavailable) {
					log.Printf("explain auth: authorization unavailable for %s: %v", addr, err)
					c.JSON(http.StatusServiceUnavailable, gin.H{"error": "AI regeneration authorization unavailable"})
				} else {
					c.JSON(http.StatusUnauthorized, gin.H{
						"error": "invalid, expired, or already used authorization proof",
					})
				}
				return
			}
		} else {
			body, err := readBoundedBody(c, 1<<10)
			if err != nil || len(body) != 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "initial explanation request body must be empty"})
				return
			}
		}

		resp, err := e.Explain(ctx, addr, regenerate)
		if err != nil {
			if retryAfter, ok := aiexplain.RetryAfter(err); ok {
				seconds := int64((retryAfter + time.Second - 1) / time.Second)
				c.Header("Retry-After", strconv.FormatInt(seconds, 10))
			}
			if status, body, ok := knownExplainErrorResponse(err); ok {
				c.JSON(status, body)
			} else {
				// Log the wrapped error server-side (may include Anthropic
				// status / response details) but return a generic public
				// message, internal error strings can leak provider config
				// hints (quota messages, model id) that aren't useful to
				// API consumers.
				log.Printf("explain: %s failed: %v", addr, err)
				c.JSON(http.StatusBadGateway, gin.H{"error": "AI explanation failed; try again shortly"})
			}
			return
		}
		c.JSON(http.StatusOK, resp)
	}
}

func readBoundedBody(c *gin.Context, limit int64) ([]byte, error) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, limit)
	return io.ReadAll(c.Request.Body)
}

func knownExplainErrorResponse(err error) (int, gin.H, bool) {
	switch {
	case errors.Is(err, aiexplain.ErrNotFound):
		return http.StatusNotFound, gin.H{"error": "address is not a known contract"}, true
	case errors.Is(err, aiexplain.ErrNotVerified):
		return http.StatusForbidden, gin.H{
			"error": "contract is not verified, only verified contracts can be analysed",
		}, true
	case errors.Is(err, aiexplain.ErrRegenCap):
		return http.StatusTooManyRequests, gin.H{
			"error": "regeneration cap reached: 5 regenerations per contract per 7-day window. Try again later.",
		}, true
	case errors.Is(err, aiexplain.ErrGenerationInProgress):
		return http.StatusConflict, gin.H{
			"error": "AI explanation generation is already in progress; retry after the indicated delay",
		}, true
	case errors.Is(err, aiexplain.ErrDailyBudget):
		return http.StatusTooManyRequests, gin.H{
			"error": "AI explanation daily provider budget reached; retry after the UTC reset",
		}, true
	case errors.Is(err, aiexplain.ErrBudgetCooldown):
		return http.StatusServiceUnavailable, gin.H{
			"error": "AI explanation provider budget is unavailable; retry after the indicated delay",
		}, true
	case errors.Is(err, aiexplain.ErrProviderCooldown):
		return http.StatusBadGateway, gin.H{
			"error": "AI explanation provider call failed; retry after the indicated delay",
		}, true
	case errors.Is(err, aiexplain.ErrCacheCooldown):
		return http.StatusServiceUnavailable, gin.H{
			"error": "AI explanation could not be cached; retry after the indicated delay",
		}, true
	case errors.Is(err, aiexplain.ErrStorageUnavailable):
		return http.StatusServiceUnavailable, gin.H{
			"error": "AI explanation storage is unavailable; retry after the indicated delay",
		}, true
	case errors.Is(err, aiexplain.ErrSourceChanged):
		return http.StatusConflict, gin.H{
			"error": "contract source changed while the explanation was generated; retry",
		}, true
	case errors.Is(err, aiexplain.ErrProvenance):
		return http.StatusConflict, gin.H{
			"error": "AI explanation requires digest-backed compiler and source-bundle provenance",
		}, true
	default:
		return 0, nil, false
	}
}
