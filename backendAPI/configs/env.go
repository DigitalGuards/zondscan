package configs

import (
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

const (
	explainAuthOriginEnv  = "AI_EXPLAIN_AUTH_ORIGIN"
	explainAuthChainIDEnv = "AI_EXPLAIN_AUTH_CHAIN_ID"
)

var ErrExplainAuthNotConfigured = errors.New("AI explanation authorization is not configured")

type ExplainAuthSettings struct {
	Origin          string
	ExpectedChainID string
}

// LoadExplainAuthSettings reads the paired trusted values used by the
// regeneration authorization service. Both settings are required together.
// Their protocol-level validation occurs in explainauth.NewConfig.
func LoadExplainAuthSettings(getenv func(string) string) (ExplainAuthSettings, error) {
	origin := strings.TrimSpace(getenv(explainAuthOriginEnv))
	chainID := strings.TrimSpace(getenv(explainAuthChainIDEnv))
	if origin == "" || chainID == "" {
		return ExplainAuthSettings{}, fmt.Errorf(
			"%w: set %s and %s",
			ErrExplainAuthNotConfigured,
			explainAuthOriginEnv,
			explainAuthChainIDEnv,
		)
	}
	return ExplainAuthSettings{Origin: origin, ExpectedChainID: chainID}, nil
}

func EnvExplainAuthSettings() (ExplainAuthSettings, error) {
	return LoadExplainAuthSettings(os.Getenv)
}

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

// ValidateEnv fail-fasts on missing required configuration before the server
// starts serving. Without this, a missing MONGOURI surfaces only as a late
// log.Fatal inside ConnectDB, and a missing NODE_URL silently defaults to
// localhost in db.NodeRPC, masking a misconfiguration in production. Call
// this once early in main. EnvMongoURI() loads the .env file as a side
// effect, so calling it first ensures NODE_URL is populated from .env too.
func ValidateEnv() {
	if EnvMongoURI() == "" {
		log.Fatal("MONGOURI is not set; set it in the environment or .env before starting")
	}
	if os.Getenv("NODE_URL") == "" {
		log.Fatal("NODE_URL is not set; set it in the environment or .env before starting")
	}
}
