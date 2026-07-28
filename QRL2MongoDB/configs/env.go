package configs

import (
	"log"
	"os"
	"sync"

	"github.com/joho/godotenv"
)

var loadEnvOnce sync.Once

// LoadEnv loads .env into the process environment (idempotent). Entrypoints
// must call it before reading any config from os.Getenv: since ConnectDB
// stopped running at package init, nothing loads .env implicitly anymore.
// Missing .env is fine: real environment variables take precedence anyway.
func LoadEnv() {
	loadEnvOnce.Do(func() {
		if err := godotenv.Load(); err != nil {
			log.Println("Warning: .env file not found, using environment variables")
		}
	})
}

func EnvMongoURI() string {
	// If MONGOURI is already set (e.g., via Docker), use it directly
	if uri := os.Getenv("MONGOURI"); uri != "" {
		return uri
	}

	// Otherwise, try to load from .env file
	LoadEnv()

	return os.Getenv("MONGOURI")
}
