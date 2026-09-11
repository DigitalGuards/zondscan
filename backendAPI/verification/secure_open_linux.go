//go:build linux

package verification

import (
	"os"

	"golang.org/x/sys/unix"
)

func openReadOnlyNoFollow(path string) (*os.File, error) {
	how := &unix.OpenHow{
		Flags: uint64(unix.O_RDONLY | unix.O_CLOEXEC | unix.O_NONBLOCK),
		Resolve: unix.RESOLVE_NO_SYMLINKS |
			unix.RESOLVE_NO_MAGICLINKS,
	}
	fd, err := unix.Openat2(unix.AT_FDCWD, path, how)
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
