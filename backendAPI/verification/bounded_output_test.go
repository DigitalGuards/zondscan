package verification

import "testing"

func TestBoundedBufferRetainsExactLimitAndCancelsOnce(t *testing.T) {
	cancels := 0
	buffer := newBoundedBuffer(5, func() { cancels++ })
	for _, chunk := range [][]byte{[]byte("abc"), []byte("def"), []byte("ghi")} {
		written, err := buffer.Write(chunk)
		if err != nil || written != len(chunk) {
			t.Fatalf("Write(%q) = %d, %v", chunk, written, err)
		}
	}
	if got := string(buffer.Bytes()); got != "abcde" {
		t.Fatalf("bounded content = %q, want %q", got, "abcde")
	}
	if !buffer.Overflowed() {
		t.Fatal("bounded buffer did not record overflow")
	}
	if cancels != 1 {
		t.Fatalf("cancel count = %d, want 1", cancels)
	}
}

func TestNormalizedPositiveLimitNeverDisablesABound(t *testing.T) {
	for _, configured := range []int{0, -1, -4096} {
		if got := normalizedPositiveLimit(configured, 1234); got != 1234 {
			t.Errorf("normalizedPositiveLimit(%d) = %d, want fallback 1234", configured, got)
		}
	}
	if got := normalizedPositiveLimit(4096, 1234); got != 4096 {
		t.Fatalf("normalizedPositiveLimit(4096) = %d, want configured value", got)
	}
}
