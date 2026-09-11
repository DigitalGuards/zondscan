//go:build !unix && !windows

package verification

import "os"

// openSecureRegularFile performs the portable pre-open and post-open identity
// checks around this fallback. Platforms without Unix open flags cannot add
// O_NONBLOCK or O_NOFOLLOW at the descriptor-open boundary.
func openReadOnlyNoFollow(path string) (*os.File, error) {
	return os.Open(path)
}
