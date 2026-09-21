package qns

import (
	"encoding/hex"
	"errors"
	"strings"
	"testing"
)

func TestNormalizeNameMatchesSDKProfile(t *testing.T) {
	cases := map[string]string{
		"alice.qrl":    "alice.qrl",
		"ALICE.qrl":    "alice.qrl",
		"Alice.QRL":    "alice.qrl",
		"a-1.b2.qrl":   "a-1.b2.qrl",
		"a--b.qrl":     "a--b.qrl",
		"-edge-.qrl":   "-edge-.qrl",
		"nested.x.qrl": "nested.x.qrl",
	}
	for input, want := range cases {
		got, err := NormalizeName(input)
		if err != nil {
			t.Fatalf("NormalizeName(%q): %v", input, err)
		}
		if got != want {
			t.Errorf("NormalizeName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNormalizeNameRejectsUnsupportedNames(t *testing.T) {
	for _, input := range []string{
		"",
		"qrl",
		".qrl",
		"alice.qrl.",
		"alice..qrl",
		"alice.eth",
		"al ice.qrl",
		"al_ice.qrl",
		"café.qrl",
		"аlice.qrl",
		"ab--cd.qrl",
		"xn--name.qrl",
		strings.Repeat("a", MaxNameBytes) + ".qrl",
	} {
		if _, err := NormalizeName(input); !errors.Is(err, ErrInvalidName) {
			t.Errorf("NormalizeName(%q) error = %v, want ErrInvalidName", input, err)
		}
	}
}

func TestNamehashKnownSDKVectors(t *testing.T) {
	vectors := map[string]string{
		"":             strings.Repeat("0", 64),
		"addr.reverse": "91d1777781884d03a6757a803996e38de2a42967fb37eeaca72729271025a9e2",
		"alice.qrl":    "efe3586aa9a851831a32d38044822af21cc5380e38f05cdc0dd562b4cfada103",
	}
	for name, want := range vectors {
		node := Namehash(name)
		if got := hex.EncodeToString(node[:]); got != want {
			t.Errorf("Namehash(%q) = %s, want %s", name, got, want)
		}
	}
}

func TestQRVM64CallDataMatchesSDKFraming(t *testing.T) {
	node := Namehash("alice.qrl")
	paddedNode := "efe3586aa9a851831a32d38044822af21cc5380e38f05cdc0dd562b4cfada103" +
		strings.Repeat("0", 64)
	cases := map[string]string{
		resolverCallData(node): "0x0178b8bf" + paddedNode,
		addrCallData(node):     "0x3b3b57de" + paddedNode,
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("call data = %s, want %s", got, want)
		}
		if len(got) != 2+8+abiWordHex {
			t.Errorf("call data length = %d, want %d", len(got), 2+8+abiWordHex)
		}
	}
}
