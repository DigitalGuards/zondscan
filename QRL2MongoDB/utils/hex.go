package utils

import (
	"math/big"
	"strconv"
	"strings"
)

// HexToInt converts a hex string to a *big.Int
func HexToInt(hex string) *big.Int {
	if len(hex) <= 2 {
		return big.NewInt(0)
	}
	hex = strings.TrimPrefix(hex, "0x")
	n := new(big.Int)
	n.SetString(hex, 16)
	return n
}

// CompareHexNumbers compares two hex numbers, returns:
// -1 if a < b
//
//	0 if a == b
//	1 if a > b
func CompareHexNumbers(a, b string) int {
	aInt := HexToInt(a)
	bInt := HexToInt(b)
	return aInt.Cmp(bInt)
}

// AddHexNumbers adds two hex numbers and returns the result as a hex string
func AddHexNumbers(a, b string) string {
	aInt := HexToInt(a)
	bInt := HexToInt(b)
	result := new(big.Int).Add(aInt, bInt)
	if result.Sign() == 0 {
		return "0x0"
	}
	return "0x" + result.Text(16)
}

// SubtractHexNumbers subtracts two hex numbers and returns the result as a hex string
func SubtractHexNumbers(a, b string) string {
	aInt := HexToInt(a)
	bInt := HexToInt(b)
	result := new(big.Int).Sub(aInt, bInt)
	if result.Sign() == 0 {
		return "0x0"
	}
	return "0x" + result.Text(16)
}

// IntToHex converts an int to a hex string
func IntToHex(n int) string {
	if n == 0 {
		return "0x0"
	}
	return "0x" + new(big.Int).SetInt64(int64(n)).Text(16)
}

// HexToInt64 parses a hex block number string (e.g. "0x1a2b") to int64.
// Returns 0 on any parse error: callers (rollback paths included) depend on
// the 0 sentinel, do not change this to return an error.
func HexToInt64(hex string) int64 {
	s := strings.TrimPrefix(hex, "0x")
	if s == "" {
		return 0
	}
	n, err := strconv.ParseInt(s, 16, 64)
	if err != nil {
		return 0
	}
	return n
}
