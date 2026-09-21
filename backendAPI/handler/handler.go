package handler

import (
	"backendAPI/aiexplain"
	"backendAPI/configs"
	"backendAPI/db"
	"backendAPI/explainauth"
	"backendAPI/models"
	"backendAPI/routes"
	"backendAPI/verification"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"runtime/debug"
	"strings"
	"time"

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

// corsMiddleware implements the method-split CORS policy: GET (and HEAD)
// requests are served with a wildcard allow-origin, POST requests only
// echo origins on the allowlist. Browser extensions send
// Origin: chrome-extension://<install-id>; the id is per-install so it
// cannot be allowlisted by value, so extension schemes are checked here.
// Disallowed browser origins are rejected before route middleware runs.
// Requests without Origin remain available to non-browser API clients.
func corsMiddleware(postAllowOrigins []string) gin.HandlerFunc {
	postOriginAllowed := func(origin string) bool {
		lower := strings.ToLower(origin)
		for _, scheme := range []string{
			"chrome-extension://",
			"moz-extension://",
			"safari-web-extension://",
		} {
			if strings.HasPrefix(lower, scheme) {
				return true
			}
		}
		for _, allowed := range postAllowOrigins {
			if strings.EqualFold(allowed, origin) {
				return true
			}
		}
		return false
	}
	return func(c *gin.Context) {
		// Responses differ per Origin (POST allowlist echoes it back), so
		// caches must key on it unconditionally: a shared cache that stored
		// a no-Origin response without Vary would serve it, ACAO-less, to
		// later browser CORS requests.
		c.Header("Vary", "Origin")
		origin := c.Request.Header.Get("Origin")
		if origin == "" {
			c.Next()
			return
		}

		// For preflights the method being authorized is in the
		// Access-Control-Request-Method header, the request itself is OPTIONS.
		effectiveMethod := c.Request.Method
		if effectiveMethod == http.MethodOptions {
			effectiveMethod = c.Request.Header.Get("Access-Control-Request-Method")
		}
		switch effectiveMethod {
		case http.MethodGet, http.MethodHead:
			c.Header("Access-Control-Allow-Origin", "*")
		default:
			if !postOriginAllowed(origin) {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
					"error": "origin is not allowed for this request",
				})
				return
			}
			c.Header("Access-Control-Allow-Origin", origin)
		}
		c.Header("Access-Control-Expose-Headers", "Content-Length")

		if c.Request.Method == http.MethodOptions {
			c.Header("Access-Control-Allow-Methods", "GET, POST")
			c.Header("Access-Control-Allow-Headers", "Origin, Content-Length, Content-Type, Authorization")
			c.Header("Access-Control-Max-Age", "43200")
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
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
	// header, letting an attacker pick a fresh per-IP rate-limit bucket
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

	// CORS is split by method. GET endpoints are the public read API and
	// are open to every origin (`Access-Control-Allow-Origin: *`), which
	// is what makes the "free public API, no key" promise real for
	// browser-based dApps. POST endpoints stay scoped to the explorer's
	// own origins: /contract/explain is Anthropic-billed and
	// /contract/verify + /contract/call are abuse-sensitive, so a
	// third-party page must never be able to fire them under each
	// visitor's IP. Operators can extend the POST allowlist via the
	// CORS_ALLOW_ORIGINS env var (comma-separated origins). Credentials
	// are never allowed in either mode.
	postAllowOrigins := []string{
		"https://zondscan.com",
		"https://www.zondscan.com",
	}
	if env := os.Getenv("CORS_ALLOW_ORIGINS"); env != "" {
		postAllowOrigins = nil
		for _, o := range strings.Split(env, ",") {
			if o = strings.TrimSpace(o); o != "" {
				postAllowOrigins = append(postAllowOrigins, o)
			}
		}
	}
	router.Use(corsMiddleware(postAllowOrigins))
	log.Printf("CORS: GET open to all origins; POST allowlist: %v", postAllowOrigins)

	// Initialize MongoDB connection with additional error handling
	log.Println("Initializing MongoDB connection...")
	dbClient := configs.ConnectDB()
	if dbClient == nil {
		log.Fatal("Failed to get MongoDB client, shutting down")
	}
	preflightCtx, preflightCancel := context.WithTimeout(context.Background(), 10*time.Second)
	if err := db.ValidateBlockIngestionReadiness(preflightCtx); err != nil {
		preflightCancel()
		log.Fatalf("Block ingestion migration preflight failed: %v", err)
	}
	preflightCancel()
	log.Println("MongoDB connection successful")

	// Init the contract-verification singleton before routes register;
	// the handlers tolerate a nil verifier (503 response) so a missing
	// compiler configuration never blocks the rest of the backend from booting.
	if err := verification.Init(); err != nil {
		log.Printf("Contract verification disabled: %v", err)
	} else {
		log.Println("Contract verification ready")
		defer func() {
			if err := verification.Close(); err != nil {
				log.Printf("Contract verification cleanup failed: %v", err)
			}
		}()
	}

	// Init the AI explainer singleton. Same nil-tolerance pattern as the
	// verifier, handlers return 503 when Default() is nil, so a missing
	// API key never blocks the rest of the backend.
	if err := aiexplain.Init(); err != nil {
		log.Printf("Contract AI explainer disabled: %v", err)
	} else {
		log.Println("Contract AI explainer ready")
	}

	// Creator authorization is independently optional at startup. A missing or
	// invalid configuration disables only regeneration; initial explanation
	// generation remains available through the public verified-contract gate.
	if err := initExplainAuthorization(); err != nil {
		log.Printf("Contract AI regeneration authorization disabled: %v", err)
	} else {
		log.Println("Contract AI regeneration authorization ready")
	}

	// Start the market trade collector. The fund-flow rollups read stored
	// trades, and venues expose no historical tape, so this must run
	// continuously rather than on request: nothing else writes the history
	// those endpoints read. A failure here leaves the rollups empty and
	// every other route unaffected, so it never blocks startup.
	startMarketCollector()

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
		server := newHTTPServer(httpsPort, router)
		if err := server.ListenAndServeTLS(certPath, keyPath); err != nil {
			log.Fatalf("Failed to start HTTPS server: %v", err)
		}
	} else {
		httpPort := os.Getenv("HTTP_PORT")
		if httpPort == "" {
			httpPort = ":8080"
		}
		log.Printf("Starting development server on HTTP port %s\n", httpPort)
		server := newHTTPServer(httpPort, router)
		if err := server.ListenAndServe(); err != nil {
			log.Fatalf("Failed to start HTTP server: %v", err)
		}
	}

	log.Println("Server shutdown complete") // This should never execute unless router.Run returns
}

func newHTTPServer(address string, handler http.Handler) *http.Server {
	return &http.Server{
		Addr:              address,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    1 << 20,
	}
}

func initExplainAuthorization() error {
	explainauth.SetDefault(nil)
	settings, err := configs.EnvExplainAuthSettings()
	if err != nil {
		return err
	}
	config, err := explainauth.NewConfig(settings.Origin, settings.ExpectedChainID)
	if err != nil {
		return err
	}
	indexContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := configs.ValidateExplainChallengeTTLIndex(indexContext); err != nil {
		return fmt.Errorf("validate challenge TTL index: %w", err)
	}
	service, err := explainauth.NewService(config, explainauth.Dependencies{
		Store: db.NewContractExplainChallengeStore(configs.ContractExplainChallengesCollection),
		Contracts: explainauth.ContractReaderFunc(func(
			ctx context.Context,
			address string,
		) (models.ContractInfo, error) {
			if err := ctx.Err(); err != nil {
				return models.ContractInfo{}, err
			}
			contract, err := db.ReturnContractCode(address)
			if err != nil {
				return models.ContractInfo{}, err
			}
			if err := ctx.Err(); err != nil {
				return models.ContractInfo{}, err
			}
			return contract, nil
		}),
		Chain: explainauth.ChainIDReaderFunc(readNodeChainID),
	})
	if err != nil {
		return err
	}
	explainauth.SetDefault(service)
	return nil
}

func readNodeChainID(ctx context.Context) (string, error) {
	result, rpcErr, err := db.NodeRPC(ctx, "qrl_chainId", []interface{}{})
	if err != nil {
		return "", err
	}
	if rpcErr != nil {
		return "", rpcErr
	}
	var chainID string
	if err := json.Unmarshal(result, &chainID); err != nil || chainID == "" {
		return "", fmt.Errorf("qrl_chainId returned an invalid result")
	}
	return chainID, nil
}
