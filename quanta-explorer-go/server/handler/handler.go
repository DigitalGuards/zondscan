package handler

import (
	"quanta-explorer-go/configs"
	"quanta-explorer-go/routes"
	"time"
	"os"
	"log"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func RequestHandler() {
	router := gin.Default()
	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST"},
		AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	configs.ConnectDB()

	routes.UserRoute(router)

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
		router.RunTLS(httpsPort, certPath, keyPath)
	} else {
		httpPort := os.Getenv("HTTP_PORT")
		if httpPort == "" {
			httpPort = ":8080"
		}
		router.Run(httpPort)
	}
}
