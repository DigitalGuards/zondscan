package aiexplain

import (
	"log"
	"os"
	"strconv"
	"sync"
)

// Env-driven config. The verifier package uses the same idiom so handler
// init reads naturally.
//
//	EXPERT_ANALIST_CLAUDE_HAIKU  Anthropic API key. Required.
//	AI_EXPLAIN_MODEL             override model id (default claude-haiku-4-5).
//	AI_EXPLAIN_MAX_TOKENS        cap on response tokens (default 1500).
//	AI_EXPLAIN_SOURCE_MAX_BYTES  truncate source before sending (default 32 KiB).
//
// Init returns a non-nil error iff the API key is missing or empty. The
// rest of the backend boots regardless, routes/explain returns 503 when
// Default() is nil.
const (
	envAPIKey         = "EXPERT_ANALIST_CLAUDE_HAIKU"
	envModel          = "AI_EXPLAIN_MODEL"
	envMaxTokens      = "AI_EXPLAIN_MAX_TOKENS"
	envSourceMaxBytes = "AI_EXPLAIN_SOURCE_MAX_BYTES"
)

var (
	defaultMu sync.RWMutex
	def       *Explainer
)

func Init() error {
	apiKey := os.Getenv(envAPIKey)
	if apiKey == "" {
		log.Printf("WARN: contract AI explainer disabled, %s not set", envAPIKey)
		return errMissingKey
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
	def = &Explainer{Client: client, SourceMaxBytes: srcMax}
	defaultMu.Unlock()
	log.Printf("aiexplain ready: model=%s maxTokens=%d sourceMaxBytes=%d", client.model, maxTokens, srcMax)
	return nil
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
