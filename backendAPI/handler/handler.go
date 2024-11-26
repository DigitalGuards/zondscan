package handler

import (
	"backendAPI/configs"
	"backendAPI/routes"
	"log"
	"os"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func RequestHandler() {
	log.Println("Initializing API server...")

	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))
	log.Println("CORS configuration initialized successfully")

	configs.ConnectDB()

	routes.UserRoute(router)
	log.Println("API routes initialized successfully")

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "development"
	}

	if env == "production" {
		certPath := os.Getenv("CERT_PATH")
		keyPath := os.Getenv("KEY_PATH")
		httpsPort := os.Getenv("HTTPS_PORT")
		if certPath == "" || keyPath == "" {
			log.Fatal("TLS paths are not configured")
		}
		log.Printf("Starting production server on HTTPS port %s\n", httpsPort)
		router.RunTLS(httpsPort, certPath, keyPath)
	} else {
		httpPort := os.Getenv("HTTP_PORT")
		if httpPort == "" {
			httpPort = ":8080"
		}
		log.Printf("Starting development server on HTTP port %s\n", httpPort)
		router.Run(httpPort)
	}
}
