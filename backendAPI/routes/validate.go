package routes

import (
	"backendAPI/db"
	"regexp"
	"strconv"
	"strings"
)

// This file collects the input-format validators used by the route
// handlers. Three address/hex validators coexist ON PURPOSE, each with a
// different accepted grammar:
//   - isValidAddressParam: Q/q or 0x/0X + 40 hex; the permissive form for
//     read-route path params, both address shapes the frontend forwards.
//   - isValidAddress: canonical uppercase "Q" + 40 hex only; reserved for
//     the contract-write endpoints (verify/call/explain) whose inputs are
//     never 0x-form.
//   - isValidHex: 0x + even-length lowercase hex; validates calldata, not
//     an address.
//
// Do not merge them: loosening isValidAddress or tightening
// isValidAddressParam changes which requests 400.

// Path-param format guards. These reject malformed input at the route
// boundary (HTTP 400) before it reaches the db layer, instead of letting
// a junk string fan out into Mongo filters / RPC calls. The accepted
// shapes mirror what the frontend search resolver emits (see
// ExplorerFrontend/app/lib/searchResolver.ts).
var (
	// txHashRe matches a "0x"-prefixed 32-byte hash (the only tx/block
	// hash form the explorer surfaces).
	txHashRe = regexp.MustCompile(`^0x[0-9a-fA-F]{64}$`)
	// addrParamRe matches both address forms the frontend forwards: the
	// canonical "Q" + 40 hex (case-insensitive Q) and the "0x" + 40 hex
	// contract form. db.normalizeAddress canonicalises both downstream.
	addrParamRe = regexp.MustCompile(`^([Qq]|0x|0X)[0-9a-fA-F]{40}$`)
	// validatorPubkeyRe matches a hex public-key lookup key (optional 0x
	// prefix). Even-length and an upper bound are enforced in
	// isValidValidatorID rather than the pattern, because Go's regexp
	// rejects the nested-repeat form that would express "even, bounded"
	// inline. The decimal-index form is handled separately so plain
	// integers still resolve.
	validatorPubkeyRe = regexp.MustCompile(`^(0x|0X)?[0-9a-fA-F]+$`)
)

// isValidTxHash reports whether s is a "0x"-prefixed 32-byte hash.
func isValidTxHash(s string) bool {
	return txHashRe.MatchString(s)
}

// isValidAddressParam reports whether s is one of the address forms a
// route path param legitimately carries (Q + 40 hex or 0x + 40 hex).
// Use this for :address / :query route guards; the stricter Q-only
// isValidAddress stays reserved for the contract-write endpoints whose
// inputs are never 0x-form.
func isValidAddressParam(s string) bool {
	return addrParamRe.MatchString(s)
}

// isValidValidatorID reports whether s is a decimal validator index or a
// hex public-key key. GetValidatorByID looks up by _id (decimal index)
// then falls back to publicKeyHex, so both shapes are legitimate.
func isValidValidatorID(s string) bool {
	if s == "" {
		return false
	}
	if _, err := strconv.ParseUint(s, 10, 64); err == nil {
		return true
	}
	if !validatorPubkeyRe.MatchString(s) {
		return false
	}
	// Strip an optional 0x prefix, then require an even-length hex body of
	// a sane size (1..2000 hex chars covers any real pubkey scheme).
	body := s
	if len(body) >= 2 && (body[:2] == "0x" || body[:2] == "0X") {
		body = body[2:]
	}
	return len(body) > 0 && len(body)%2 == 0 && len(body) <= 2000
}

// normalizeContractAddr maps any input address form (0x-hex, q-lower, bare
// hex, Q-canonical) to the canonical "Q" + lowercase hex shape the syncer
// stores in contractCode. Mirrors db.normalizeAddress (package-private)
// so route handlers can resolve map keys returned by
// db.GetContractsByAddresses without re-importing the db pkg's logic.
func normalizeContractAddr(addr string) string {
	if addr == "" {
		return ""
	}
	return db.NormalizeAddress(addr)
}

// isValidAddress is a permissive Q-prefix check. The full validation
// happens at the storage layer (normalizeAddress canonicalises case).
func isValidAddress(a string) bool {
	if !strings.HasPrefix(a, "Q") {
		return false
	}
	if len(a) != 41 {
		return false
	}
	for _, r := range a[1:] {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f') || (r >= 'A' && r <= 'F')) {
			return false
		}
	}
	return true
}

// isValidHex returns true for "0x"-prefixed even-length strings of hex
// digits. Empty payloads (0x by itself) are allowed since some view
// functions take no arguments and call data is just the 4-byte selector.
func isValidHex(s string) bool {
	if !strings.HasPrefix(s, "0x") {
		return false
	}
	hex := s[2:]
	if len(hex)%2 != 0 {
		return false
	}
	for _, r := range hex {
		if !((r >= '0' && r <= '9') || (r >= 'a' && r <= 'f')) {
			return false
		}
	}
	return true
}
