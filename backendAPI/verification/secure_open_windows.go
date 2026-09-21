//go:build windows

package verification

import "os"

func openReadOnlyNoFollow(path string) (*os.File, error) {
	return os.Open(path)
}
