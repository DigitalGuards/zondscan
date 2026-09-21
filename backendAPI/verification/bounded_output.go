package verification

import (
	"bytes"
	"sync"
)

// boundedBuffer retains at most limit bytes. The first excess byte triggers
// cancel exactly once while Write still reports the full input as consumed, so
// os/exec can tear down the process without a competing short-write error.
type boundedBuffer struct {
	buffer   bytes.Buffer
	limit    int
	cancel   func()
	overflow bool
	once     sync.Once
}

func newBoundedBuffer(limit int, cancel func()) *boundedBuffer {
	return &boundedBuffer{limit: limit, cancel: cancel}
}

func (b *boundedBuffer) Write(p []byte) (int, error) {
	written := len(p)
	remaining := b.limit - b.buffer.Len()
	if remaining > len(p) {
		remaining = len(p)
	}
	if remaining > 0 {
		_, _ = b.buffer.Write(p[:remaining])
	}
	if remaining < len(p) {
		b.overflow = true
		b.once.Do(b.cancel)
	}
	return written, nil
}

func (b *boundedBuffer) Bytes() []byte { return b.buffer.Bytes() }

func (b *boundedBuffer) Overflowed() bool { return b.overflow }
