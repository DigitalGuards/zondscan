package verification

import (
	"log"
	"sync"
)

// Process-wide default Verifier. nil when the compiler registry is unavailable;
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
	reg, err := NewRegistry()
	if err != nil {
		log.Printf("WARN: contract verification disabled, %v", err)
		return err
	}
	next := &Verifier{Registry: reg}
	defaultMu.Lock()
	previous := def
	def = next
	defaultMu.Unlock()
	if previous != nil {
		_ = previous.Registry.Close()
	}
	return nil
}

// Default returns the package-level Verifier or nil when verification is
// not configured.
func Default() *Verifier {
	defaultMu.RLock()
	defer defaultMu.RUnlock()
	return def
}

// Close releases the sealed compiler snapshots held by the process-wide
// verifier. It is safe to call when verification was never initialized.
func Close() error {
	defaultMu.Lock()
	previous := def
	def = nil
	defaultMu.Unlock()
	if previous == nil {
		return nil
	}
	return previous.Registry.Close()
}
