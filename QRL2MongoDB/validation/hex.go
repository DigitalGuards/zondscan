package validation

import (
	"fmt"
	"strings"
)

// hasQPrefix reports whether s begins with 'Q' or 'q'.
func hasQPrefix(s string) bool {
	return len(s) > 0 && (s[0] == 'Q' || s[0] == 'q')
}

// hasHexPrefix reports whether s begins with "0x" or "0X".
func hasHexPrefix(s string) bool {
	return len(s) > 1 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X')
}

const (
	AddressLength = 40 // Length of address without prefix (0x or Q)
	HashLength    = 64 // Length of transaction/block hash without 0x prefix
)

// IsValidHexString checks if a string is a valid hex string with 0x prefix
func IsValidHexString(hex string) bool {
	if !hasHexPrefix(hex) {
		return false
	}

	// Check remaining characters are valid hex
	for _, c := range hex[2:] {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

// IsValidAddress checks if a string is a valid QRL address
// Supports both legacy 0x format and new Q format (case-insensitive Q prefix)
func IsValidAddress(address string) bool {
	// Check for new Q-prefix format (uppercase or lowercase)
	if hasQPrefix(address) {
		// Validate the rest of the address is hex
		for _, c := range address[1:] {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
		return len(address) == AddressLength+1 // +1 for "Q" or "q" prefix
	}

	// Check for legacy 0x format
	if !IsValidHexString(address) {
		return false
	}
	return len(address) == AddressLength+2 // +2 for "0x" prefix
}

// IsValidHash checks if a string is a valid transaction or block hash
func IsValidHash(hash string) bool {
	if !IsValidHexString(hash) {
		return false
	}
	return len(hash) == HashLength+2 // +2 for "0x" prefix
}

// EnsureHexPrefix adds "0x" prefix if missing
func EnsureHexPrefix(hex string) string {
	if !hasHexPrefix(hex) {
		return "0x" + hex
	}
	return hex
}

// ValidateHexString validates a hex string and returns an error if invalid
func ValidateHexString(hex string, expectedLength int) error {
	if !IsValidHexString(hex) {
		return fmt.Errorf("invalid hex string format: %s", hex)
	}

	actualLength := len(hex) - 2 // subtract "0x" prefix
	if expectedLength > 0 && actualLength != expectedLength {
		return fmt.Errorf("invalid hex string length: got %d, want %d", actualLength, expectedLength)
	}

	return nil
}

// ValidateAddress validates an address string and returns an error if invalid
// Supports both legacy 0x format and new Q format
func ValidateAddress(address string) error {
	if !IsValidAddress(address) {
		return fmt.Errorf("invalid address format: %s", address)
	}
	return nil
}

// StripHexPrefix removes "0x" prefix if present
func StripHexPrefix(hex string) string {
	if hasHexPrefix(hex) {
		return hex[2:]
	}
	return hex
}

// StripAddressPrefix removes "0x", "Q", or "q" prefix if present
func StripAddressPrefix(address string) string {
	if hasHexPrefix(address) {
		return address[2:]
	}
	if hasQPrefix(address) {
		return address[1:]
	}
	return address
}

// ConvertToQAddress converts any address format to canonical Q-prefix form.
// The canonical storage format is "Q" + lowercase hex.
// Returns empty string for empty input to avoid producing bare "Q".
func ConvertToQAddress(address string) string {
	if address == "" {
		return ""
	}

	if hasQPrefix(address) {
		return "Q" + strings.ToLower(address[1:])
	}

	if hasHexPrefix(address) {
		return "Q" + strings.ToLower(address[2:])
	}

	return "Q" + strings.ToLower(address)
}

// IsZeroAddress reports whether addr is any accepted spelling of the zero
// address (mint/burn endpoint): short or full-width, Q or 0x prefixed.
func IsZeroAddress(addr string) bool {
	switch addr {
	case "Q0", "0x0",
		"Q0000000000000000000000000000000000000000",
		"0x0000000000000000000000000000000000000000":
		return true
	}
	return false
}
