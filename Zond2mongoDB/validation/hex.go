package validation

import (
	"fmt"
	"strings"
)

const (
	AddressLength = 40 // Length of address without prefix (0x or Z)
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
// Supports both legacy 0x format and new Z format (case-insensitive Z prefix)
func IsValidAddress(address string) bool {
	// Check for new Z-prefix format (uppercase or lowercase)
	if strings.HasPrefix(address, "Z") || strings.HasPrefix(address, "z") {
		// Validate the rest of the address is hex
		for _, c := range address[1:] {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return false
			}
		}
		return len(address) == AddressLength+1 // +1 for "Z" or "z" prefix
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

// ValidateAddress validates an address string and returns an error if invalid
// Supports both legacy 0x format and new Z format
func ValidateAddress(address string) error {
	if !IsValidAddress(address) {
		return fmt.Errorf("invalid address format: %s", address)
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

// StripAddressPrefix removes "0x", "Z", or "z" prefix if present
func StripAddressPrefix(address string) string {
	if strings.HasPrefix(address, "0x") {
		return address[2:]
	}
	if strings.HasPrefix(address, "Z") || strings.HasPrefix(address, "z") {
		return address[1:]
	}
	return address
}

// ConvertToZAddress converts a 0x address to Z format if needed
func ConvertToZAddress(address string) string {
	// If already in Z format (uppercase or lowercase), return as is
	if strings.HasPrefix(address, "Z") || strings.HasPrefix(address, "z") {
		return address
	}

	// If in 0x format, convert to Z format
	if strings.HasPrefix(address, "0x") {
		return "Z" + address[2:]
	}

	// If no prefix, add Z prefix
	return "Z" + address
}
