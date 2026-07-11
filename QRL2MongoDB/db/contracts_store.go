package db

import (
	"QRL2MongoDB/configs"
	"QRL2MongoDB/models"
	"QRL2MongoDB/validation"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/zap"
)

// maxTokenStringRunes caps the stored length of on-chain token name/symbol
// strings. On-chain name()/symbol() are attacker-controlled, an unbounded
// string is both a storage-bloat and a downstream-rendering hazard.
const maxTokenStringRunes = 128

// sanitizeTokenString cleans an on-chain token name/symbol before it is
// persisted. It is the single ingestion-time chokepoint for these
// attacker-controlled strings: every StoreContract write passes through it,
// so all downstream readers (API JSON, future CSV export, server-rendered
// templates) receive an already-cleaned value rather than relying on a
// specific sink to escape it.
//
// Behavior:
//   - strips ASCII/Unicode control characters (C0/C1, DEL, etc.) that have no
//     legitimate place in a display name and could break a non-JSX sink;
//   - preserves all legitimate Unicode letters/marks/symbols/punctuation;
//   - trims leading/trailing whitespace;
//   - caps the result at maxTokenStringRunes runes (rune-safe, never splits a
//     multibyte codepoint).
func sanitizeTokenString(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return s
	}

	// name()/symbol() are attacker-controlled on-chain strings of unbounded
	// length, so this must not allocate or scan the whole input. Cap the bytes
	// examined first (a rune is at most 4 bytes, so maxTokenStringRunes runes
	// fit in 4x that), then single-pass: drop control characters, keep all
	// other unicode, and stop as soon as the rune budget is reached.
	if len(s) > maxTokenStringRunes*4 {
		s = s[:maxTokenStringRunes*4]
	}

	var b strings.Builder
	b.Grow(len(s))
	runeCount := 0
	for _, r := range s {
		if unicode.IsControl(r) {
			continue
		}
		b.WriteRune(r)
		runeCount++
		if runeCount >= maxTokenStringRunes {
			break
		}
	}

	return strings.TrimSpace(b.String())
}

// StoreContract stores or merges contract information in the database.
//
// **Merge-safety invariant**: the syncer is the only writer of the
// syncer-owned fields enumerated by `syncerOwnedSet` below; the backend
// verify endpoint is the only writer of the verification fields
// (verified / sourceCode / abi / contractName / compilerVersion /
// optimizationEnabled / optimizationRuns / evmVersion /
// constructorArguments / libraries / license / verificationMethod /
// verifiedAt). The two write paths NEVER overlap.
//
// Previously this function did `$set: updateData` (whole-doc), which
// re-introduced a race: if the syncer's FindOne read predated a
// concurrent verification write, the whole-doc $set would clobber the
// freshly-written verification fields with stale empties from the
// in-memory copy. Switching to a field-scoped $set listing only the
// syncer-owned fields makes that race impossible by construction ,
// the syncer literally cannot touch verification keys.
func StoreContract(contract models.ContractInfo) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	// Normalize addresses to canonical Q-prefix form
	contract.Address = validation.ConvertToQAddress(contract.Address)
	contract.CreatorAddress = validation.ConvertToQAddress(contract.CreatorAddress)

	// Sanitize attacker-controlled on-chain token strings at the single
	// storage chokepoint so every write path (creation tx, reprocess,
	// classifier) persists a control-char-free, length-capped value.
	contract.Name = sanitizeTokenString(contract.Name)
	contract.Symbol = sanitizeTokenString(contract.Symbol)

	collection := configs.GetContractsCollection()
	filter := bson.M{"address": contract.Address}

	// Attempt to find existing contract document
	var existingContract models.ContractInfo
	err := collection.FindOne(ctx, filter).Decode(&existingContract)

	merged := contract

	if err == nil {
		// Existing contract found, merge new data into it
		configs.Logger.Debug("Found existing contract, merging data", zap.String("address", contract.Address))
		merged = existingContract

		// Merge fields from the new 'contract' object, only if the new value is non-empty/non-zero
		// and the existing value *is* empty/zero. This prioritizes data from the creation tx.
		// Treat bare "Q" (from legacy ConvertToQAddress("")) as empty.
		if (merged.CreatorAddress == "" || merged.CreatorAddress == "Q") && contract.CreatorAddress != "" && contract.CreatorAddress != "Q" {
			merged.CreatorAddress = contract.CreatorAddress
		}
		if merged.CreationTransaction == "" && contract.CreationTransaction != "" {
			merged.CreationTransaction = contract.CreationTransaction
		}
		if merged.CreationBlockNumber == "" && contract.CreationBlockNumber != "" {
			merged.CreationBlockNumber = contract.CreationBlockNumber
		}
		if merged.ContractCode == "" && contract.ContractCode != "" && contract.ContractCode != "0x" {
			merged.ContractCode = contract.ContractCode
		}
		if merged.Status == "" && contract.Status != "" {
			merged.Status = contract.Status
		} else if contract.Status != "" && merged.Status != contract.Status {
			merged.Status = contract.Status
		}

		// Token classification is promote-only: once a contract has been
		// identified as a token (Name/Symbol/Decimals populated from a
		// successful RPC probe), we never demote it back. The previous
		// logic clobbered Name/Symbol/Decimals/TotalSupply to empty whenever
		// `GetTokenInfo` returned `isToken=false`, and that path also
		// returns false on every *transient* RPC error (name()/symbol()
		// timeout, decode failure, node failover blip). Real-world symptom
		// was real ERC-20 tokens flickering to empty `name`/`symbol` in the
		// explorer during node hiccups, then restoring on the next
		// interaction that succeeded. Now the merge:
		//   - flips IsToken only false → true;
		//   - copies Name/Symbol/Decimals/TotalSupply only when the existing
		//     value is empty (fills gaps from a richer probe);
		//   - never clears non-empty token metadata.
		// Re-classification (true → false) is intentionally NOT a side
		// effect of any read path, it must be an explicit operator action.
		if contract.IsToken {
			merged.IsToken = true
			if merged.Name == "" && contract.Name != "" {
				merged.Name = contract.Name
			}
			if merged.Symbol == "" && contract.Symbol != "" {
				merged.Symbol = contract.Symbol
			}
			if merged.Decimals == 0 && contract.Decimals != 0 {
				merged.Decimals = contract.Decimals
			}
			if merged.TotalSupply == "" && contract.TotalSupply != "" {
				merged.TotalSupply = contract.TotalSupply
			}
		}
		// `contract.IsToken == false` is a no-op for IsToken + token
		// metadata; keep existing values intact.

		// TokenStandard promotion ladder: "" → ERC-20 → ERC-721/1155.
		// Same promote-only rationale as IsToken: a transient RPC blip
		// must never demote a previously-classified contract. ERC-1155
		// outranks ERC-721 so dual-impl edge cases (rare) pick the
		// broader standard. The merge never moves DOWN the ladder.
		if standardRank(contract.TokenStandard) > standardRank(merged.TokenStandard) {
			merged.TokenStandard = contract.TokenStandard
		}
		// HasERC165 latches true forever, a contract that ever responded
		// to supportsInterface didn't UN-implement ERC-165 later. Skip the
		// flag flip on transient probe failures (those return HasERC165=false).
		if contract.HasERC165 {
			merged.HasERC165 = true
		}
		// BaseURI is gap-fill only (Phase 3 will populate it from tokenURI
		// / uri probes). Never overwrite or clear.
		if merged.BaseURI == "" && contract.BaseURI != "" {
			merged.BaseURI = contract.BaseURI
		}
		// Phase 3a collection metadata: gap-fill only for MetadataURI from
		// the classifier path. The metadata fetcher service has its own
		// write path (UpdateContractMetadata) for the *resolved* fields
		// (Name/Description/Image/ExternalURL/FetchedAt/FetchError); the
		// classifier never touches those, so a transient classification
		// blip can't clobber a previously-resolved metadata document.
		if merged.MetadataURI == "" && contract.MetadataURI != "" {
			merged.MetadataURI = contract.MetadataURI
		}
	} else if !errors.Is(err, mongo.ErrNoDocuments) {
		configs.Logger.Error("Failed to check for existing contract",
			zap.String("address", contract.Address),
			zap.Error(err))
		return err
	}

	// IsToken back-compat: any classified standard implies isToken=true so
	// the legacy `?isToken=true` API filter continues to surface NFT
	// collections alongside ERC-20s. Applied in BOTH the merge and the
	// fresh-write path, callers that set TokenStandard without IsToken
	// (e.g. backfill scripts) still produce correct rows.
	if merged.TokenStandard != "" {
		merged.IsToken = true
	}

	// Stamp updatedAt once for both code paths (merge + first-write).
	// Live indexing, the wall clock is the right source here.
	merged.UpdatedAt = time.Now().UTC().Format(time.RFC3339)

	opts := options.Update().SetUpsert(true)
	update := bson.M{"$set": syncerOwnedSet(merged)}

	_, err = collection.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		configs.Logger.Error("Failed to store/merge contract",
			zap.String("address", contract.Address),
			zap.Error(err))
		return err
	}

	configs.Logger.Info("Successfully stored/merged contract", zap.String("address", merged.Address))
	return nil
}

// syncerOwnedSet returns the $set payload for fields the syncer is allowed
// to write. Verification fields (verified / sourceCode / abi / ...) are
// deliberately absent, they belong exclusively to the backend verify
// endpoint. Keep this list in sync with the non-verification fields on
// models.ContractInfo; any field added to the model that the syncer also
// populates must be added here.
//
// We use an explicit allow-list rather than marshal-and-delete-known-keys
// for two reasons:
//
//  1. Default-deny is the safer posture for a write boundary protecting
//     trust-critical fields. Adding a new SYNCER-owned field and
//     forgetting to add it here results in a visible omission (easy to
//     spot in tests / data sampling). The inverted shape would default
//     to "syncer may write any new field", a new VERIFICATION field
//     forgotten in the delete list would get silently clobbered. Silent
//     trust-store corruption is the much worse failure mode.
//  2. bson.Marshal + bson.Unmarshal per write is real overhead in the
//     hot path (every contract creation, every reprocess pass).
//
// The optional CustomERC20 caps respect their model `omitempty` semantics:
// empty values are not written, matching the pre-refactor on-disk shape so
// `$exists` queries (if any are ever added) behave the same.
func syncerOwnedSet(c models.ContractInfo) bson.M {
	m := bson.M{
		"address":             c.Address,
		"status":              c.Status,
		"isToken":             c.IsToken,
		"name":                c.Name,
		"symbol":              c.Symbol,
		"decimals":            c.Decimals,
		"totalSupply":         c.TotalSupply,
		"contractCode":        c.ContractCode,
		"creatorAddress":      c.CreatorAddress,
		"creationTransaction": c.CreationTransaction,
		"creationBlockNumber": c.CreationBlockNumber,
		"updatedAt":           c.UpdatedAt,
	}
	if c.MaxSupply != "" {
		m["maxSupply"] = c.MaxSupply
	}
	if c.MaxWalletAmount != "" {
		m["maxWalletAmount"] = c.MaxWalletAmount
	}
	if c.MaxTxLimit != "" {
		m["maxTxLimit"] = c.MaxTxLimit
	}
	// NFT classification, omit when empty/false so the document stays clean
	// for non-token contracts AND a transient blip that produces zero values
	// can't clobber a previously-set value (writing `false` would overwrite
	// `true`; omitting the key leaves the existing value untouched).
	if c.TokenStandard != "" {
		m["tokenStandard"] = c.TokenStandard
	}
	if c.HasERC165 {
		m["hasERC165"] = true
	}
	if c.BaseURI != "" {
		m["baseURI"] = c.BaseURI
	}
	// Phase 3a collection metadata: classifier writes only the URI; the
	// resolved fields are owned by the fetcher and updated through
	// UpdateContractMetadata (contracts_metadata.go). The omitempty pattern
	// protects existing fetcher state from being clobbered on classifier
	// passes.
	if c.MetadataURI != "" {
		m["metadataURI"] = c.MetadataURI
	}
	return m
}

// standardRank orders TokenStandard values for promote-only merging.
// Higher rank = stricter / more specific classification. Promotions go
// up the ladder; demotions are silently dropped.
func standardRank(s string) int {
	switch s {
	case "ERC-1155":
		return 3
	case "ERC-721":
		return 2
	case "ERC-20":
		return 1
	default:
		return 0
	}
}

// GetContract retrieves contract information from the database
func GetContract(address string) (*models.ContractInfo, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	// Normalize address to canonical Q-prefix form
	address = validation.ConvertToQAddress(address)

	var contract models.ContractInfo
	err := configs.GetContractsCollection().FindOne(ctx, bson.M{"address": address}).Decode(&contract)
	if err != nil {
		return nil, fmt.Errorf("failed to get contract: %v", err)
	}

	return &contract, nil
}
