package handler

import (
	"backendAPI/configs"
	"backendAPI/routes"
	"log"
	"os"
	"runtime/debug"
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

	// Add custom middlewares
	router.Use(gin.Logger())         // Standard logger
	router.Use(recoveryMiddleware()) // Custom recovery middleware
	router.Use(monitorMiddleware())  // Request monitoring middleware

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	log.Println("CORS configuration initialized successfully")

	// Initialize MongoDB connection with additional error handling
	log.Println("Initializing MongoDB connection...")
	dbClient := configs.ConnectDB()
	if dbClient == nil {
		log.Fatal("Failed to get MongoDB client, shutting down")
	}
	log.Println("MongoDB connection successful")

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
