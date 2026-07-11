package verification

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"backendAPI/db"
)

// MatchOutcome is the result of comparing fresh-compile bytecode against
// on-chain runtime code. NotFound separates "no contract code at this
// address" from a clean mismatch so the caller can pick the right error
// message.
type MatchOutcome struct {
	Matched         bool
	CompiledLen     int
	OnChainLen      int
	OnChainHex      string // the authoritative runtime code (canonical, no 0x)
	CompiledHex     string // the masked compile output (no 0x)
	DiffByteOffset  int    // first divergent byte (only set when !Matched); -1 = identical prefix, longer side has extra bytes
	NotFound        bool   // true when qrl_getCode returned empty / 0x
	ImmutablesCount int
}

// FetchOnChainCode pulls the authoritative runtime code via qrl_getCode.
// Never trust the syncer's base64-encoded contractCode field for matching:
// the syncer source might lag, and any byte-shape difference between
// "what's actually deployed" and "what the indexer captured" would silently
// invalidate verification.
//
// Transport is delegated to db.NodeRPC so the verifier shares the
// process-wide RPC client (connection pool, timeout policy) and the
// single NODE_URL resolution with every other node caller.
func FetchOnChainCode(ctx context.Context, address string) (string, error) {
	raw, rpcErr, err := db.NodeRPC(ctx, "qrl_getCode", []interface{}{address, "latest"})
	if err != nil {
		return "", err
	}
	if rpcErr != nil {
		return "", fmt.Errorf("node RPC error %d: %s", rpcErr.Code, rpcErr.Message)
	}

	var code string
	if err := json.Unmarshal(raw, &code); err != nil {
		return "", fmt.Errorf("node RPC body: %w", err)
	}
	return code, nil
}

// Match compares freshly-compiled deployedBytecode against authoritative
// on-chain runtime code, masking any `immutable`-backed byte ranges in
// both before comparing.
//
// Hyperion v0.0.2 emits a Solidity-style CBOR metadata trailer at the end
// of the deployed bytecode (last 2 bytes = trailer length, preceding that
// length is a CBOR map carrying the IPFS hash of source metadata + the
// hypc build id). The trailer's IPFS hash mixes in the source file path
// used at compile time, so a verifier that compiles via "Foo.hyp" cannot
// recover the same hash as a deployer that compiled via "contract.hyp"
// even when the source content is byte-identical. We strip the trailer
// from both sides before comparing, the masking step that follows then
// gives us a byte-equal compare of the code body, which is the security-
// relevant payload.
func Match(compiledHex, onChainHex string, immRefs map[string][]ImmutableRange) (MatchOutcome, error) {
	compiledHex = strings.ToLower(strings.TrimPrefix(compiledHex, "0x"))
	onChainHex = strings.ToLower(strings.TrimPrefix(onChainHex, "0x"))

	out := MatchOutcome{
		CompiledHex: compiledHex,
		OnChainHex:  onChainHex,
		CompiledLen: len(compiledHex) / 2,
		OnChainLen:  len(onChainHex) / 2,
	}

	if onChainHex == "" || onChainHex == "0" {
		out.NotFound = true
		return out, nil
	}

	cb, err := hex.DecodeString(compiledHex)
	if err != nil {
		return out, fmt.Errorf("decode compiled hex: %w", err)
	}
	ob, err := hex.DecodeString(onChainHex)
	if err != nil {
		return out, fmt.Errorf("decode on-chain hex: %w", err)
	}

	cb = stripCBORTrailer(cb)
	ob = stripCBORTrailer(ob)

	immutablesCount := 0
	for _, ranges := range immRefs {
		immutablesCount += len(ranges)
	}
	out.ImmutablesCount = immutablesCount

	// Mask immutable ranges on BOTH sides. Bounds-checked so a malformed
	// immutableReferences map can't OOB.
	for _, ranges := range immRefs {
		for _, r := range ranges {
			if r.Start < 0 || r.Length < 0 {
				continue
			}
			endA := r.Start + r.Length
			if endA > len(cb) {
				endA = len(cb)
			}
			endB := r.Start + r.Length
			if endB > len(ob) {
				endB = len(ob)
			}
			for i := r.Start; i < endA; i++ {
				cb[i] = 0
			}
			for i := r.Start; i < endB; i++ {
				ob[i] = 0
			}
		}
	}

	if len(cb) != len(ob) {
		// Length mismatch is a hard fail; report the first divergent
		// byte for diagnostics.
		min := len(cb)
		if len(ob) < min {
			min = len(ob)
		}
		out.DiffByteOffset = -1
		for i := 0; i < min; i++ {
			if cb[i] != ob[i] {
				out.DiffByteOffset = i
				break
			}
		}
		return out, nil
	}

	if bytes.Equal(cb, ob) {
		out.Matched = true
		return out, nil
	}

	// Same length but bytes differ, surface the first diff offset.
	for i := range cb {
		if cb[i] != ob[i] {
			out.DiffByteOffset = i
			return out, nil
		}
	}
	return out, errors.New("byte slices unequal but no diff found")
}

// stripCBORTrailer removes the Solidity/Hyperion-style metadata trailer
// from the end of deployed bytecode. The convention: the LAST 2 bytes are
// a big-endian uint16 length of the preceding CBOR map; we strip those
// length bytes + the map.
//
// Sanity gates (return input unchanged if any fails):
//
//   - len(b) < 2                    no length bytes
//   - cborLen <= 0                  garbage / no trailer claimed
//   - cborLen+2 >= len(b)           would strip the entire code body ,
//     two such inputs would compare equal
//     even when their code bodies differ
//   - cborLen > 1024                real metadata is well under 100 B; a
//     length larger than this is almost
//     certainly the wrong interpretation of
//     two random tail bytes
//
// Either condition leaves the bytes alone so the existing length+masked
// equal compare still catches real differences.
const maxCBORTrailerLen = 1024

func stripCBORTrailer(b []byte) []byte {
	if len(b) < 2 {
		return b
	}
	cborLen := int(b[len(b)-2])<<8 | int(b[len(b)-1])
	if cborLen <= 0 || cborLen+2 >= len(b) || cborLen > maxCBORTrailerLen {
		return b
	}
	return b[:len(b)-cborLen-2]
}
