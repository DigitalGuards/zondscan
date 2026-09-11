package db

import (
	"strings"
	"testing"
)

func TestAppendRollbackNativeAddressValidatesAndCanonicalizes(t *testing.T) {
	firstBody := strings.Repeat("ab", 64)
	secondBody := strings.Repeat("cd", 64)
	zeroAddress := "Q" + strings.Repeat("0", 128)

	addresses := make([]string, 0)
	for _, address := range []string{
		"",
		"Qinvalid",
		zeroAddress,
		"0x" + strings.ToUpper(firstBody),
		"Q" + secondBody,
	} {
		addresses = appendRollbackNativeAddress(addresses, address)
	}

	want := []string{"Q" + firstBody, "Q" + secondBody}
	if len(addresses) != len(want) {
		t.Fatalf("rollback native addresses = %v, want %v", addresses, want)
	}
	for index := range want {
		if addresses[index] != want[index] {
			t.Fatalf("rollback native address %d = %q, want %q", index, addresses[index], want[index])
		}
	}
}
