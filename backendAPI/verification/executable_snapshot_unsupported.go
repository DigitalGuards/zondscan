//go:build !linux

package verification

import (
	"errors"
	"os"
)

func snapshotExecutable(_, _ string) (*os.File, error) {
	return nil, errors.New("sealed compiler snapshots require Linux")
}
