package configs

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

func EnvMongoURI() string {
	// If MONGOURI is already set (e.g., via Docker), use it directly
	if uri := os.Getenv("MONGOURI"); uri != "" {
		return uri
	}

	// Otherwise, try to load from .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	return os.Getenv("MONGOURI")
}
