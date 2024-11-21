package configs

import (
	"log"
	"os"
	"github.com/joho/godotenv"
)

func EnvMongoURI() string {
	path, err := os.Getwd()
	if err != nil {
		log.Println(err)
	}

	err = godotenv.Load(path + "/.env." + os.Getenv("APP_ENV"))
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	return os.Getenv("MONGOURI")
}
