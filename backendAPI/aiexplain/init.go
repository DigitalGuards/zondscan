package aiexplain

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"sync"
)

// Env-driven config. The verifier package uses the same idiom so handler
// init reads naturally.
//
//	EXPERT_ANALIST_CLAUDE_HAIKU  Anthropic API key. Required.
//	AI_EXPLAIN_MODEL             override model id (default claude-haiku-4-5).
//	AI_EXPLAIN_MAX_TOKENS        cap on response tokens (default 1500).
//	AI_EXPLAIN_SOURCE_MAX_BYTES  truncate source before sending (default 32 KiB).
//	AI_EXPLAIN_DAILY_PROVIDER_CALL_LIMIT  shared UTC-day provider-call cap (default 100).
//
// Init returns a non-nil error when the API key is absent or the daily limit
// is invalid. The rest of the backend boots regardless, routes/explain returns
// 503 when Default() is nil.
const (
	envAPIKey                     = "EXPERT_ANALIST_CLAUDE_HAIKU"
	envModel                      = "AI_EXPLAIN_MODEL"
	envMaxTokens                  = "AI_EXPLAIN_MAX_TOKENS"
	envSourceMaxBytes             = "AI_EXPLAIN_SOURCE_MAX_BYTES"
	envDailyProviderCallLimit     = "AI_EXPLAIN_DAILY_PROVIDER_CALL_LIMIT"
	DefaultDailyProviderCallLimit = 100
)

var (
	defaultMu sync.RWMutex
	def       *Explainer
)

func Init() error {
	apiKey := os.Getenv(envAPIKey)
	if apiKey == "" {
		defaultMu.Lock()
		def = nil
		defaultMu.Unlock()
		log.Printf("WARN: contract AI explainer disabled, %s not set", envAPIKey)
		return errMissingKey
	}
	dailyLimit, err := loadDailyProviderCallLimit(os.Getenv)
	if err != nil {
		defaultMu.Lock()
		def = nil
		defaultMu.Unlock()
		return err
	}

	model := os.Getenv(envModel)
	maxTokens, _ := strconv.Atoi(os.Getenv(envMaxTokens))
	if maxTokens <= 0 {
		maxTokens = 1500
	}
	srcMax, _ := strconv.Atoi(os.Getenv(envSourceMaxBytes))
	if srcMax <= 0 {
		srcMax = 32 * 1024
	}

	client := NewClient(apiKey, model, maxTokens)
	defaultMu.Lock()
	def = &Explainer{
		Client:                 client,
		SourceMaxBytes:         srcMax,
		DailyProviderCallLimit: dailyLimit,
	}
	defaultMu.Unlock()
	log.Printf(
		"aiexplain ready: model=%s maxTokens=%d sourceMaxBytes=%d dailyProviderCallLimit=%d",
		client.model,
		maxTokens,
		srcMax,
		dailyLimit,
	)
	return nil
}

func loadDailyProviderCallLimit(getenv func(string) string) (int, error) {
	raw := strings.TrimSpace(getenv(envDailyProviderCallLimit))
	if raw == "" {
		return DefaultDailyProviderCallLimit, nil
	}
	limit, err := strconv.Atoi(raw)
	if err != nil || limit <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", envDailyProviderCallLimit)
	}
	return limit, nil
}

// Default returns the package-level Explainer or nil when not configured.
func Default() *Explainer {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return def
}

// errMissingKey is a sentinel, callers don't need to compare against it,
// but the typed error keeps the Init() signature honest.
var errMissingKey = errMissingKeyVal{}

type errMissingKeyVal struct{}

func (errMissingKeyVal) Error() string {
	return envAPIKey + " env var is required"
}
