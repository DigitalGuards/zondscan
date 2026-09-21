//go:build unix && !linux

package verification

import (
	"os"

	"golang.org/x/sys/unix"
)

func openReadOnlyNoFollow(path string) (*os.File, error) {
	fd, err := unix.Open(path, unix.O_RDONLY|unix.O_CLOEXEC|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), path)
	if f == nil {
		_ = unix.Close(fd)
		return nil, os.ErrInvalid
	}
	return f, nil
}
