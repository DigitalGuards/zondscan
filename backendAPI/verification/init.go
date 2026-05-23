package verification

import (
	"log"
	"sync"
)

// Process-wide default Verifier. nil when the runner isn't configured ,
// the routes layer must check Default() and return 503 in that case so
// the rest of the backend boots normally.
var (
	defaultMu sync.RWMutex
	def       *Verifier
)

// Init constructs the package-level Verifier from environment variables.
// Called once at backend startup. A nil error here means /contract/verify*
// routes are live; a non-nil error logs a warning and leaves the default
// nil, the rest of the backend remains usable.
func Init() error {
	c, err := NewCompiler()
	if err != nil {
		log.Printf("WARN: contract verification disabled, %v", err)
		return err
	}
	defaultMu.Lock()
	def = &Verifier{Compiler: c}
	defaultMu.Unlock()
	return nil
}

// Default returns the package-level Verifier or nil when verification is
// not configured.
func Default() *Verifier {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return def
}
