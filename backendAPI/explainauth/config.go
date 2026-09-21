package explainauth

import (
	"fmt"
	"math/big"
	"net"
	"net/url"
	"strings"
)

// NewConfig validates the trusted site origin and expected execution chain.
// Production origins require HTTPS. Plain HTTP is limited to loopback hosts.
func NewConfig(origin, expectedChainID string) (Config, error) {
	canonicalOrigin, err := normalizeOrigin(origin)
	if err != nil {
		return Config{}, fmt.Errorf("%w: origin: %v", ErrInvalidConfig, err)
	}
	chainID, err := NormalizeChainID(expectedChainID)
	if err != nil {
		return Config{}, fmt.Errorf("%w: chain ID: %v", ErrInvalidConfig, err)
	}
	return Config{origin: canonicalOrigin, expectedChainID: chainID}, nil
}

// NormalizeChainID returns a minimal lowercase 0x-prefixed chain ID.
func NormalizeChainID(value string) (string, error) {
	if value == "" || value != strings.TrimSpace(value) || strings.HasPrefix(value, "+") || strings.HasPrefix(value, "-") {
		return "", errorsNewInvalidChainID()
	}

	base := 10
	digits := value
	if strings.HasPrefix(value, "0x") || strings.HasPrefix(value, "0X") {
		base = 16
		digits = value[2:]
	}
	if digits == "" {
		return "", errorsNewInvalidChainID()
	}
	for _, r := range digits {
		if (r < '0' || r > '9') && (base != 16 || ((r < 'a' || r > 'f') && (r < 'A' || r > 'F'))) {
			return "", errorsNewInvalidChainID()
		}
	}

	n := new(big.Int)
	if _, ok := n.SetString(digits, base); !ok || n.Sign() <= 0 || n.BitLen() > 256 {
		return "", errorsNewInvalidChainID()
	}
	return "0x" + n.Text(16), nil
}

func errorsNewInvalidChainID() error {
	return fmt.Errorf("expected a positive decimal or 0x-prefixed value of at most 256 bits")
}

func normalizeOrigin(value string) (string, error) {
	if value == "" || value != strings.TrimSpace(value) {
		return "", fmt.Errorf("origin is empty or contains surrounding whitespace")
	}
	for _, r := range value {
		if r < 0x21 || r > 0x7e {
			return "", fmt.Errorf("origin must contain printable ASCII only")
		}
	}

	u, err := url.Parse(value)
	if err != nil || u.Scheme == "" || u.Host == "" || u.Opaque != "" {
		return "", fmt.Errorf("origin must be an absolute URL")
	}
	if u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/") {
		return "", fmt.Errorf("origin must not contain credentials, path, query, or fragment")
	}

	scheme := strings.ToLower(u.Scheme)
	hostname := strings.ToLower(u.Hostname())
	if hostname == "" {
		return "", fmt.Errorf("origin hostname is empty")
	}
	isLoopback := hostname == "localhost" || hostname == "127.0.0.1" || hostname == "::1"
	if scheme != "https" && !(scheme == "http" && isLoopback) {
		return "", fmt.Errorf("origin requires HTTPS except on explicit loopback")
	}

	port := u.Port()
	if (scheme == "https" && port == "443") || (scheme == "http" && port == "80") {
		port = ""
	}
	host := hostname
	if port != "" {
		host = net.JoinHostPort(hostname, port)
	} else if strings.Contains(hostname, ":") {
		host = "[" + hostname + "]"
	}
	return scheme + "://" + host, nil
}
