package validation

import (
	"fmt"
	"strings"
)

const (
	AddressLength = 40 // Length of address without 0x prefix
	HashLength    = 64 // Length of transaction/block hash without 0x prefix
)

// IsValidHexString checks if a string is a valid hex string with 0x prefix
func IsValidHexString(hex string) bool {
	if !strings.HasPrefix(hex, "0x") {
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

// IsValidAddress checks if a string is a valid Zond address
func IsValidAddress(address string) bool {
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
	if !strings.HasPrefix(hex, "0x") {
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

// StripHexPrefix removes "0x" prefix if present
func StripHexPrefix(hex string) string {
	if strings.HasPrefix(hex, "0x") {
		return hex[2:]
	}
	return hex
}
