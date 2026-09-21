//go:build !linux

package verification

import (
	"strings"
	"testing"
)

func TestExecutableSnapshotFailsClosedOutsideLinux(t *testing.T) {
	_, err := snapshotExecutable("/absolute/path/to/hypc", strings.Repeat("0", 64))
	if err == nil || !strings.Contains(err.Error(), "require Linux") {
		t.Fatalf("snapshotExecutable error = %v, want Linux availability failure", err)
	}
}
