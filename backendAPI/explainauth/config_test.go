package explainauth

import (
	"strings"
	"testing"
)

func TestNewConfigCanonicalizesTrustedBindings(t *testing.T) {
	config, err := NewConfig("HTTPS://ZONDSCAN.COM:443/", "1337")
	if err != nil {
		t.Fatalf("NewConfig: %v", err)
	}
	if config.origin != "https://zondscan.com" || config.expectedChainID != "0x539" {
		t.Fatalf("config = %#v", config)
	}
}

func TestNewConfigRejectsUntrustedOriginAndInvalidChain(t *testing.T) {
	tests := []struct {
		name    string
		origin  string
		chainID string
	}{
		{name: "plain HTTP remote", origin: "http://zondscan.com", chainID: "0x539"},
		{name: "credentials", origin: "https://user@example.com", chainID: "0x539"},
		{name: "path", origin: "https://zondscan.com/api", chainID: "0x539"},
		{name: "query", origin: "https://zondscan.com?chain=1", chainID: "0x539"},
		{name: "zero chain", origin: "https://zondscan.com", chainID: "0"},
		{name: "negative chain", origin: "https://zondscan.com", chainID: "-1"},
		{name: "oversized chain", origin: "https://zondscan.com", chainID: "0x1" + strings.Repeat("0", 64)},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := NewConfig(test.origin, test.chainID); err == nil {
				t.Fatal("NewConfig succeeded")
			}
		})
	}
}

func TestNewConfigAllowsExplicitLoopbackHTTP(t *testing.T) {
	for _, origin := range []string{
		"http://localhost:3000",
		"http://127.0.0.1:3000",
		"http://[::1]:3000",
	} {
		t.Run(origin, func(t *testing.T) {
			if _, err := NewConfig(origin, "0x539"); err != nil {
				t.Fatalf("NewConfig: %v", err)
			}
		})
	}
}

func TestNewConfigRejectsNonCanonicalLoopbackAliases(t *testing.T) {
	for _, origin := range []string{
		"http://dev.localhost:3000",
		"http://127.0.0.2:3000",
	} {
		t.Run(origin, func(t *testing.T) {
			if _, err := NewConfig(origin, "0x539"); err == nil {
				t.Fatal("NewConfig succeeded")
			}
		})
	}
}

func TestNormalizeChainID(t *testing.T) {
	tests := map[string]string{
		"1":      "0x1",
		"001337": "0x539",
		"0X0539": "0x539",
		"0x539":  "0x539",
	}
	for input, want := range tests {
		got, err := NormalizeChainID(input)
		if err != nil || got != want {
			t.Fatalf("NormalizeChainID(%q) = %q, %v; want %q", input, got, err, want)
		}
	}
}
