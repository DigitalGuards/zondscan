package verification

import (
	"fmt"
	"os"
	"path/filepath"
)

// openSecureRegularFile opens one trusted artifact without following a final
// symlink. Platform-specific open flags provide non-blocking, no-follow
// behavior where available. The pre-open and post-open identity checks keep
// FIFO, device, socket, symlink, and path-replacement inputs out of callers.
func openSecureRegularFile(path, kind string) (*os.File, os.FileInfo, error) {
	if !filepath.IsAbs(path) {
		return nil, nil, fmt.Errorf("%s path must be absolute", kind)
	}
	cleanPath := filepath.Clean(path)
	resolvedPath, err := filepath.EvalSymlinks(cleanPath)
	if err != nil {
		return nil, nil, fmt.Errorf("resolve %s path %q: %w", kind, path, err)
	}
	if resolvedPath != cleanPath {
		return nil, nil, fmt.Errorf("%s path must not contain symlinks: %q", kind, path)
	}

	pathInfo, err := os.Lstat(cleanPath)
	if err != nil {
		return nil, nil, fmt.Errorf("stat %s path %q: %w", kind, path, err)
	}
	if !pathInfo.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("%s must be a regular file: %q", kind, path)
	}

	f, err := openReadOnlyNoFollow(cleanPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open %s %q without symlink following: %w", kind, path, err)
	}
	keepOpen := false
	defer func() {
		if !keepOpen {
			_ = f.Close()
		}
	}()

	openInfo, err := f.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("stat open %s %q: %w", kind, path, err)
	}
	if !openInfo.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("%s must be a regular file: %q", kind, path)
	}
	if !os.SameFile(pathInfo, openInfo) {
		return nil, nil, fmt.Errorf("%s changed while opening", kind)
	}

	keepOpen = true
	return f, openInfo, nil
}
