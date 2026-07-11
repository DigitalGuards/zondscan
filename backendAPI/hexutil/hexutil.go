// Package hexutil centralises parsing of the hexadecimal quantity strings
// the QRL node and syncer emit (block numbers, gas values, timestamps,
// log indexes).
//
// Both parsers accept an optional lowercase "0x" prefix as well as bare
// hex, matching what the pre-existing per-package helpers accepted. An
// uppercase "0X" prefix is rejected (as it was by those helpers): the
// 'X' is not a hex digit.
//
// Callers with sentinel semantics (clamp-to-zero, clamp-to-max, sort
// keys) keep those decisions at the call site; these functions only
// parse and report.
package hexutil

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// ErrRange reports a value that parsed as valid hex and fits uint64 but
// overflows int64. ParseInt64 returns math.MaxInt64 alongside it, so
// clamping callers can keep the saturated value while fail-hard callers
// propagate the error. Values that do not even fit uint64 return the
// underlying strconv range error (with a 0 value) instead, matching the
// historical "garbage yields zero" behaviour of the helpers this package
// replaced.
var ErrRange = errors.New("hexutil: value overflows int64")

// ParseInt64 parses a hex quantity string ("0x"-prefixed or bare) into
// an int64. The digits are parsed unsigned first: a plain ParseInt would
// reject any value with the high bit set (block numbers near 2^63),
// which would silently break epoch math downstream. Values in
// (MaxInt64, MaxUint64] return (math.MaxInt64, ErrRange); syntax errors
// and uint64 overflow return (0, err) with the strconv error unchanged.
func ParseInt64(hex string) (int64, error) {
	num, err := strconv.ParseUint(strings.TrimPrefix(hex, "0x"), 16, 64)
	if err != nil {
		return 0, err
	}
	if num > math.MaxInt64 {
		return math.MaxInt64, ErrRange
	}
	return int64(num), nil
}

// ParseBig parses a hex quantity string ("0x"-prefixed or bare) into a
// *big.Int. The whole string after the optional prefix must be valid
// hex; partial parses are an error, never a truncated value.
func ParseBig(hex string) (*big.Int, error) {
	b, ok := new(big.Int).SetString(strings.TrimPrefix(hex, "0x"), 16)
	if !ok {
		return nil, fmt.Errorf("hexutil: invalid hex string %q", hex)
	}
	return b, nil
}
