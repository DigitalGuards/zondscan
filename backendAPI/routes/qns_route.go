package routes

import (
	"backendAPI/cache"
	"backendAPI/db"
	"backendAPI/middleware"
	"backendAPI/qns"
	"backendAPI/qrladdress"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	qnsCacheTTL       = 15 * time.Second
	qnsRequestTimeout = 8 * time.Second
	qnsFillTimeout    = 5 * time.Second
)

// RegisterQNSRoutes wires the configurable, read-only QNS resolvers.
// The per-IP limit and short shared cache bound the amount of node work a
// public caller can trigger.
func RegisterQNSRoutes(router *gin.Engine) {
	router.GET(
		"/qns/resolve/:name",
		qnsRateLimit(30),
		newQNSResolveHandler(db.NodeRPC, routeCache),
	)
	router.GET(
		"/qns/reverse/:address",
		qnsRateLimit(60),
		newQNSReverseHandler(db.NodeRPC, routeCache),
	)
}

func qnsRateLimit(limit float64) gin.HandlerFunc {
	limiter := middleware.PerIPRateLimit(limit, limit, time.Minute)
	token := os.Getenv("QNS_INTERNAL_TOKEN")
	expected := sha256.Sum256([]byte(token))
	return func(c *gin.Context) {
		// Equal-length digests keep the secret comparison constant-time.
		if token != "" {
			provided := sha256.Sum256([]byte(c.GetHeader("X-QNS-Internal-Token")))
			if subtle.ConstantTimeCompare(provided[:], expected[:]) == 1 {
				c.Next()
				return
			}
		}
		limiter(c)
	}
}

func newQNSResolveHandler(call qns.RPCFunc, responseCache *cache.TTLCache) gin.HandlerFunc {
	return newQNSHandler(call, responseCache, false)
}

func newQNSReverseHandler(call qns.RPCFunc, responseCache *cache.TTLCache) gin.HandlerFunc {
	return newQNSHandler(call, responseCache, true)
}

func newQNSHandler(call qns.RPCFunc, responseCache *cache.TTLCache, reverse bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		var normalized string
		operation := "resolve"
		if reverse {
			operation = "reverse"
			var ok bool
			normalized, ok = qrladdress.Canonicalize(c.Param("address"))
			if !ok {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid QNS address"})
				return
			}
		} else {
			var err error
			normalized, err = qns.NormalizeName(c.Param("name"))
			if err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"error": "invalid QNS name"})
				return
			}
		}

		config, err := qns.ConfigFromEnv()
		if err != nil {
			respondQNSError(c, err, reverse)
			return
		}
		if responseCache == nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		requestCtx, cancel := context.WithTimeout(c.Request.Context(), qnsRequestTimeout)
		defer cancel()
		client := qns.NewClient(call)

		deploymentValue, err := responseCache.GetOrComputeContext(
			requestCtx,
			"qns:deployment:"+config.CacheKey(),
			qnsCacheTTL,
			func() (interface{}, error) {
				fillCtx, fillCancel := context.WithTimeout(context.Background(), qnsFillTimeout)
				defer fillCancel()
				return client.CheckDeployment(fillCtx, config)
			},
		)
		if err != nil {
			respondQNSError(c, err, reverse)
			return
		}
		deployment, ok := deploymentValue.(qns.Deployment)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		resolveForward := func(ctx context.Context, name string) (qns.Resolution, error) {
			value, err := responseCache.GetOrComputeContext(
				ctx,
				"qns:resolve:"+config.CacheKey()+":"+name,
				qnsCacheTTL,
				func() (interface{}, error) {
					fillCtx, fillCancel := context.WithTimeout(context.Background(), qnsFillTimeout)
					defer fillCancel()
					return client.ResolveName(fillCtx, deployment, name)
				},
			)
			if err != nil {
				return qns.Resolution{}, err
			}
			resolution, ok := value.(qns.Resolution)
			if !ok {
				return qns.Resolution{}, errors.New("invalid cached QNS resolution")
			}
			return resolution, nil
		}

		// Reverse verification shares the forward route's cache key and fill.
		resolutionValue, err := responseCache.GetOrComputeContext(
			requestCtx,
			"qns:"+operation+":"+config.CacheKey()+":"+normalized,
			qnsCacheTTL,
			func() (interface{}, error) {
				fillCtx, fillCancel := context.WithTimeout(context.Background(), qnsFillTimeout)
				defer fillCancel()
				if reverse {
					return client.ReverseNameWithLookup(fillCtx, deployment, normalized, resolveForward)
				}
				return client.ResolveName(fillCtx, deployment, normalized)
			},
		)
		if err != nil {
			respondQNSError(c, err, reverse)
			return
		}
		resolution, ok := resolutionValue.(qns.Resolution)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		if reverse && resolution.Name == "" {
			c.JSON(http.StatusNotFound, gin.H{
				"error":   "QNS address has no verified primary name",
				"address": resolution.Address,
			})
			return
		}
		switch resolution.Missing {
		case qns.MissingResolver:
			c.JSON(http.StatusNotFound, gin.H{
				"error": "QNS name has no resolver",
				"name":  resolution.Name,
			})
			return
		case qns.MissingAddress:
			c.JSON(http.StatusNotFound, gin.H{
				"error": "QNS name has no address record",
				"name":  resolution.Name,
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"name":    resolution.Name,
			"address": resolution.Address,
		})
	}
}

func respondQNSError(c *gin.Context, err error, reverse bool) {
	switch {
	case errors.Is(err, qns.ErrInvalidName):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid QNS name"})
	case errors.Is(err, qns.ErrNotConfigured), errors.Is(err, qns.ErrInvalidConfig):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "QNS resolution is not configured"})
	case errors.Is(err, qns.ErrChainMismatch):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "QNS deployment does not match the connected chain"})
	case errors.Is(err, qns.ErrRegistryUnavailable):
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "QNS registry is unavailable on the connected chain"})
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		if reverse {
			c.JSON(http.StatusBadGateway, gin.H{"error": "QNS resolution failed"})
			return
		}
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "QNS resolution timed out"})
	case errors.Is(err, qns.ErrRPC),
		errors.Is(err, qns.ErrMalformedResponse),
		errors.Is(err, qns.ErrInvalidAddress):
		c.JSON(http.StatusBadGateway, gin.H{"error": "QNS resolution failed"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
