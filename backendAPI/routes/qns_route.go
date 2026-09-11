package routes

import (
	"backendAPI/cache"
	"backendAPI/db"
	"backendAPI/middleware"
	"backendAPI/qns"
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	qnsCacheTTL       = 15 * time.Second
	qnsRequestTimeout = 8 * time.Second
	qnsFillTimeout    = 5 * time.Second
)

// RegisterQNSRoutes wires the configurable, read-only QNS forward resolver.
// The per-IP limit and short shared cache bound the amount of node work a
// public caller can trigger.
func RegisterQNSRoutes(router *gin.Engine) {
	limiter := middleware.PerIPRateLimit(30, 30, time.Minute)
	router.GET(
		"/qns/resolve/:name",
		limiter,
		newQNSResolveHandler(db.NodeRPC, routeCache),
	)
}

func newQNSResolveHandler(call qns.RPCFunc, responseCache *cache.TTLCache) gin.HandlerFunc {
	return func(c *gin.Context) {
		normalized, err := qns.NormalizeName(c.Param("name"))
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid QNS name"})
			return
		}

		config, err := qns.ConfigFromEnv()
		if err != nil {
			respondQNSError(c, err)
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
			respondQNSError(c, err)
			return
		}
		deployment, ok := deploymentValue.(qns.Deployment)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			return
		}

		resolutionValue, err := responseCache.GetOrComputeContext(
			requestCtx,
			"qns:resolve:"+config.CacheKey()+":"+normalized,
			qnsCacheTTL,
			func() (interface{}, error) {
				fillCtx, fillCancel := context.WithTimeout(context.Background(), qnsFillTimeout)
				defer fillCancel()
				return client.ResolveName(fillCtx, deployment, normalized)
			},
		)
		if err != nil {
			respondQNSError(c, err)
			return
		}
		resolution, ok := resolutionValue.(qns.Resolution)
		if !ok {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
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

func respondQNSError(c *gin.Context, err error) {
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
		c.JSON(http.StatusGatewayTimeout, gin.H{"error": "QNS resolution timed out"})
	case errors.Is(err, qns.ErrRPC),
		errors.Is(err, qns.ErrMalformedResponse),
		errors.Is(err, qns.ErrInvalidAddress):
		c.JSON(http.StatusBadGateway, gin.H{"error": "QNS resolution failed"})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
	}
}
