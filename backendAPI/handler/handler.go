package handler

import (
	"backendAPI/aiexplain"
	"backendAPI/configs"
	"backendAPI/routes"
	"backendAPI/verification"
	"log"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// Custom recovery middleware with better logging
func recoveryMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("PANIC RECOVERED in API handler: %v\nStack trace:\n%s", err, debug.Stack())
				c.AbortWithStatusJSON(500, gin.H{
					"error": "Internal server error, please try again later",
				})
			}
		}()
		c.Next()
	}
}

// Monitor middleware to log request processing time
func monitorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path

		// Process request
		c.Next()

		// Log request details
		latency := time.Since(start)
		log.Printf("Request: %s %s | Status: %d | Latency: %s",
			c.Request.Method, path, c.Writer.Status(), latency)
	}
}

func RequestHandler() {
	log.Println("Initializing API server...")

	// Always use release mode as specified in README.md
	gin.SetMode(gin.ReleaseMode)

	router := gin.New() // Use New() instead of Default() for custom middleware

	// Trust only the local reverse proxy when reading X-Forwarded-For /
	// X-Real-IP. Without this, gin.New() defaults to trusting all proxies
	// and `c.ClientIP()` returns the first IP in any client-supplied XFF
	// header — letting an attacker pick a fresh per-IP rate-limit bucket
	// per request just by sending `X-Forwarded-For: <random>`. In prod,
	// nginx terminates TLS and reaches us over loopback, so loopback +
	// link-local IPv6 are the only legitimate proxy hops.
	//
	// Operators behind a different proxy topology should override via the
	// TRUSTED_PROXIES env var (comma-separated CIDRs or addresses).
	trustedProxies := []string{"127.0.0.1", "::1"}
	if env := os.Getenv("TRUSTED_PROXIES"); env != "" {
		trustedProxies = trustedProxies[:0]
		for _, p := range strings.Split(env, ",") {
			if p = strings.TrimSpace(p); p != "" {
				trustedProxies = append(trustedProxies, p)
			}
		}
	}
	if err := router.SetTrustedProxies(trustedProxies); err != nil {
		log.Fatalf("Failed to configure trusted proxies (%v): %v", trustedProxies, err)
	}
	log.Printf("Trusted proxies: %v", trustedProxies)

	// Add custom middlewares
	router.Use(gin.Logger())         // Standard logger
	router.Use(recoveryMiddleware()) // Custom recovery middleware
	router.Use(monitorMiddleware())  // Request monitoring middleware

	// CORS — scope to the explorer's own origins. POST endpoints
	// (/contract/verify, /contract/call, /contract/explain) are not safe
	// to expose to arbitrary web origins: a third-party page could fire
	// off Anthropic-billed explain calls under the visitor's IP. The
	// wildcard `Access-Control-Allow-Origin: *` is preserved only when
	// CORS_ALLOW_ORIGINS is unset, so operators who explicitly want a
	// fully open API can opt back in via env.
	allowOrigins := []string{
		"https://zondscan.com",
		"https://www.zondscan.com",
	}
	if env := os.Getenv("CORS_ALLOW_ORIGINS"); env != "" {
		allowOrigins = nil
		for _, o := range strings.Split(env, ",") {
			if o = strings.TrimSpace(o); o != "" {
				allowOrigins = append(allowOrigins, o)
			}
		}
	}
	router.Use(cors.New(cors.Config{
		AllowOrigins:     allowOrigins,
		AllowMethods:     []string{"GET", "POST"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}))
	log.Printf("CORS allow-origins: %v", allowOrigins)

	// Initialize MongoDB connection with additional error handling
	log.Println("Initializing MongoDB connection...")
	dbClient := configs.ConnectDB()
	if dbClient == nil {
		log.Fatal("Failed to get MongoDB client, shutting down")
	}
	log.Println("MongoDB connection successful")

	// Init the contract-verification singleton before routes register —
	// the handlers tolerate a nil verifier (503 response) so a missing
	// HYPC_RUNNER env never blocks the rest of the backend from booting.
	if err := verification.Init(); err != nil {
		log.Printf("Contract verification disabled: %v", err)
	} else {
		log.Println("Contract verification ready")
	}

	// Init the AI explainer singleton. Same nil-tolerance pattern as the
	// verifier — handlers return 503 when Default() is nil, so a missing
	// API key never blocks the rest of the backend.
	if err := aiexplain.Init(); err != nil {
		log.Printf("Contract AI explainer disabled: %v", err)
	} else {
		log.Println("Contract AI explainer ready")
	}

	// Configure routes
	log.Println("Configuring API routes...")
	routes.UserRoute(router)
	log.Println("API routes initialized successfully")

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	// Start the server
	if env == "production" {
		certPath := os.Getenv("CERT_PATH")
		keyPath := os.Getenv("KEY_PATH")
		httpsPort := os.Getenv("HTTPS_PORT")
		if certPath == "" || keyPath == "" {
			log.Fatal("TLS paths are not configured")
		}
		log.Printf("Starting production server on HTTPS port %s\n", httpsPort)
		if err := router.RunTLS(httpsPort, certPath, keyPath); err != nil {
			log.Fatalf("Failed to start HTTPS server: %v", err)
		}
	} else {
		httpPort := os.Getenv("HTTP_PORT")
		if httpPort == "" {
			httpPort = ":8080"
		}
		log.Printf("Starting development server on HTTP port %s\n", httpPort)
		if err := router.Run(httpPort); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}

	log.Println("Server shutdown complete") // This should never execute unless router.Run returns
}
