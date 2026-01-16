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
	path, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}

	err = godotenv.Load(path + "/.env" + os.Getenv("APP_ENV"))
	if err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	return os.Getenv("MONGOURI")
}
