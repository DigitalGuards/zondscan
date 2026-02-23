# Z-to-Q Address Prefix Migration Plan

## Overview

The QRL Zond network is upgrading from Z-prefix addresses to Q-prefix addresses. This document catalogs every change needed across the entire codebase and provides a phased migration strategy.

**Scope:** 30+ files across 3 components (syncer, backend API, frontend) plus infrastructure.

---

## Phase 1: Create Abstraction Layer (Do First)

Before changing any prefix, centralize all prefix logic so the actual switch is a one-line change.

### 1.1 Syncer — New helpers in `Zond2mongoDB/validation/hex.go`

Create these functions to replace all scattered prefix logic:

```go
// Configurable prefix - change this ONE constant when the network upgrades
const AddressPrefix = "Z" // Change to "Q" at network upgrade time

func GetAddressPrefix() string { return AddressPrefix }

func GetZeroAddress() string {
    return AddressPrefix + "0000000000000000000000000000000000000000"
}

func IsZeroAddress(addr string) bool {
    stripped := strings.ToLower(StripAddressPrefix(addr))
    return stripped == "0000000000000000000000000000000000000000" ||
        stripped == "0" || stripped == ""
}

func StripAddressPrefix(address string) string {
    lower := strings.ToLower(address)
    for _, prefix := range []string{"0x", "z", "q"} {
        if strings.HasPrefix(lower, prefix) {
            return address[len(prefix):]
        }
    }
    return address
}

func NormalizeAddress(address string) string {
    hex := strings.ToLower(StripAddressPrefix(address))
    if hex == "" { return "" }
    return strings.ToLower(AddressPrefix) + hex
}

func NormalizeAddressUpper(address string) string {
    hex := strings.ToLower(StripAddressPrefix(address))
    if hex == "" { return "" }
    return AddressPrefix + hex
}

func IsValidAddress(address string) bool {
    lower := strings.ToLower(address)
    if strings.HasPrefix(lower, "z") || strings.HasPrefix(lower, "q") {
        hex := lower[1:]
        if len(hex) != 40 { return false }
        for _, c := range hex {
            if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) { return false }
        }
        return true
    }
    if strings.HasPrefix(lower, "0x") {
        hex := lower[2:]
        if len(hex) != 40 { return false }
        for _, c := range hex {
            if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) { return false }
        }
        return true
    }
    return false
}
```

**Rename:** `ConvertToZAddress()` → call `NormalizeAddressUpper()` instead (update all 3 call sites in `rpc/calls.go`).

**Delete:** `QRLZeroAddress` constant from `configs/const.go` → replace all 6 usages with `validation.GetZeroAddress()`.

### 1.2 Backend API — New helpers in `backendAPI/db/helpers.go`

```go
package db

import "strings"

const AddressPrefix = "Z" // Change to "Q" at network upgrade

func stripPrefix(addr string) string {
    lower := strings.ToLower(addr)
    for _, p := range []string{"0x", "z", "q"} {
        if strings.HasPrefix(lower, p) { return addr[len(p):] }
    }
    return addr
}

func normalizeAddr(addr string) string {
    return strings.ToLower(string(AddressPrefix[0])+strings.ToLower(stripPrefix(addr)))
}

func normalizeAddrUpper(addr string) string {
    return AddressPrefix + strings.ToLower(stripPrefix(addr))
}

func addrVariants(addr string) []string {
    hex := strings.ToLower(stripPrefix(addr))
    return []string{
        "z" + hex, "Z" + hex,
        "q" + hex, "Q" + hex,
    }
}
```

Then refactor:
- `db/token.go` lines 111-125: Replace `normalizeAddress()` and `normalizeAddressBoth()` with these helpers
- `db/address.go` line 32: Use `normalizeAddr()`
- `db/contract.go` lines 98-100: Use `addrVariants()`
- `routes/routes.go` lines 288-289, 616-620: Use `normalizeAddrUpper()`

### 1.3 Frontend — Update `ExplorerFrontend/app/lib/helpers.ts`

```typescript
// Change this ONE constant when the network upgrades
export const ADDRESS_PREFIX = 'Z'; // Change to 'Q' at network upgrade

export function stripAddressPrefix(addr: string): string {
  if (!addr) return '';
  const lower = addr.toLowerCase();
  if (lower.startsWith('0x')) return addr.slice(2);
  if (lower.startsWith('z') || lower.startsWith('q')) return addr.slice(1);
  return addr;
}

export function normalizeAddress(addr: string): string {
  return ADDRESS_PREFIX + stripAddressPrefix(addr).toLowerCase();
}

export function isZondAddress(addr: string): boolean {
  const lower = addr.toLowerCase();
  return lower.startsWith('z') || lower.startsWith('q');
}
```

Then update:
- `formatAddress()` (lines 224-249): Use `ADDRESS_PREFIX` instead of hardcoded `'Z'` (4 occurrences)
- `normalizeHexString()` (lines 146-148): Use `isZondAddress()` check
- `decodeTokenTransferInput()` (lines 286, 314, 318): Use `ADDRESS_PREFIX` instead of `'Z'`

---

## Phase 2: Update All Hardcoded Z References

### 2.1 Syncer (`Zond2mongoDB/`)

| File | Lines | Current Code | Change To |
|------|-------|-------------|-----------|
| `configs/const.go` | 13 | `QRLZeroAddress = "Z000..."` | Delete, use `validation.GetZeroAddress()` |
| `validation/hex.go` | 30-46 | `HasPrefix(addr, "Z")` | Use new `IsValidAddress()` |
| `validation/hex.go` | 96-105 | `StripAddressPrefix` with hardcoded Z | Use new version supporting Z/Q |
| `validation/hex.go` | 107-121 | `ConvertToZAddress()` | Replace with `NormalizeAddressUpper()` |
| `rpc/calls.go` | 346, 354, 416 | `ConvertToZAddress()` calls | `NormalizeAddressUpper()` |
| `rpc/tokenscalls.go` | 49-52 | `HasPrefix(addr, "Z")` + `"Z" + addr` | `NormalizeAddressUpper()` |
| `rpc/tokenscalls.go` | 329-334 | 6 hardcoded zero addr checks | `IsZeroAddress()` |
| `rpc/tokenscalls.go` | 341-346, 354-357 | Z prefix enforcement | `NormalizeAddressUpper()` |
| `rpc/tokenscalls.go` | 441-442 | `"Z" + TrimLeftZeros(...)` | `AddressPrefix + TrimLeftZeros(...)` |
| `rpc/tokenscalls.go` | 560-566 | Z prefix in ParseTransferEvent | Use helper |
| `rpc/tokenscalls.go` | 631 | `"Z" + addressHex` | `AddressPrefix + addressHex` |
| `db/tokentransfers.go` | 38, 42, 229, 233 | `configs.QRLZeroAddress` | `validation.GetZeroAddress()` |
| `db/tokentransfers.go` | 199-201 | `"Z" + TrimLeftZeros(...)` | `AddressPrefix + TrimLeftZeros(...)` |
| `db/tokentransfers.go` | 228, 232 | `from == "Z"` comparison | `from == AddressPrefix` |
| `db/tokenbalances.go` | 32-35 | Hardcoded zero addr checks | `IsZeroAddress()` |
| `db/tokenbalances.go` | 33 | `configs.QRLZeroAddress` | `validation.GetZeroAddress()` |
| `scripts/reindex_tokens.py` | 95-96 | `'Z' + topics[1][-40:]` | Make prefix configurable |
| `scripts/reindex_tokens.py` | 133, 158 | Z zero address comparisons | Use Q zero address |
| `scripts/reindex_contracts.py` | 147, 236-241 | `"Z" + hex` | Make prefix configurable |

**No changes needed** (already prefix-agnostic via `.ToLower()`):
- `db/transactions.go` — uses `strings.ToLower()` normalization
- `db/coinbase.go` — uses `strings.ToLower()` normalization
- `db/contracts.go` — uses `strings.ToLower()` normalization

### 2.2 Backend API (`backendAPI/`)

| File | Lines | Current Code | Change To |
|------|-------|-------------|-----------|
| `db/address.go` | 32 | `strings.ToLower(query)` | Use `normalizeAddr()` |
| `db/address.go` | 104 | `TrimPrefix(addressHex, "z")` | `stripPrefix()` |
| `db/address.go` | 149-150 | Lowercase z→Z for RPC | `normalizeAddrUpper()` |
| `db/transaction.go` | 71-73, 172-174, 355, 564 | `HasPrefix(addr, "Z")` then prepend Z | `normalizeAddrUpper()` |
| `db/transaction.go` | 76-82 | TrimPrefix Z + case variants | Use `stripPrefix()` |
| `db/transaction.go` | 313 | `TrimPrefix(addr, "Z"), "z")` | `stripPrefix()` |
| `db/contract.go` | 34-40 | Search with z prefix | Use `addrVariants()` |
| `db/contract.go` | 98-100 | Both Z and z variants | Use `addrVariants()` |
| `db/token.go` | 111-125 | `normalizeAddress/Both` | Replace with new helpers |
| `routes/routes.go` | 288-289 | Lowercase z→Z normalization | `normalizeAddrUpper()` |
| `routes/routes.go` | 616-620 | Contract addr normalization | `normalizeAddrUpper()` |

### 2.3 Frontend (`ExplorerFrontend/`)

| File | Lines | Current Code | Change To |
|------|-------|-------------|-----------|
| `app/lib/helpers.ts` | 146-148 | `startsWith('Z') \|\| startsWith('z')` | `isZondAddress()` |
| `app/lib/helpers.ts` | 229, 239, 244 | `'Z' + ...` in formatAddress | `ADDRESS_PREFIX + ...` |
| `app/lib/helpers.ts` | 286, 314, 318 | `'Z' + hex` in decodeTokenTransfer | `ADDRESS_PREFIX + hex` |
| `app/components/SearchBar.tsx` | 14-16 | `startsWith('Z')` + regex `^Z[hex]{40}$` | Support both Z and Q |
| `app/components/SearchBar.tsx` | 77 | Placeholder `"Zxx"` | Update text |
| `app/address/[query]/page.tsx` | 89 | `startsWith('z')` → `'Z' + ...` | Handle both z/q → ADDRESS_PREFIX |
| `app/address/[query]/address-view.tsx` | 62, 69 | `startsWith("Z")` → "Zond Address" | Support Q prefix too |
| `app/validators/validators-client.tsx` | 104 | `startsWith('Z') ? addr : 'Z' + addr` | Use `normalizeAddress()` |
| `app/validators/components/ValidatorTable.tsx` | 231, 234 | Hardcoded `Z{addr.slice(...)}` | Remove hardcoded prefix |

---

## Phase 3: MongoDB Data Migration

### 3.1 Collections to migrate

All collections storing addresses with z/Z prefix need updating:

```
addresses.id
transactionByAddress.from, .to
internalTransactionByAddress.from, .to
contractCode.address, .creatorAddress
tokenTransfers.from, .to, .contractAddress
tokenBalances.holderAddress, .contractAddress
pending_transactions.from, .to
```

### 3.2 Migration script (run at cutover)

```javascript
// migrate-z-to-q.js
// Run: mongosh qrldata-z migrate-z-to-q.js

const collections = {
  'addresses': ['id'],
  'contractCode': ['address', 'creatorAddress'],
  'tokenTransfers': ['from', 'to', 'contractAddress'],
  'tokenBalances': ['holderAddress', 'contractAddress'],
  'transactionByAddress': ['from', 'to'],
  'pending_transactions': ['from', 'to'],
};

for (const [collName, fields] of Object.entries(collections)) {
  const coll = db.getCollection(collName);
  for (const field of fields) {
    // Lowercase z → lowercase q
    const filter = {};
    filter[field] = /^z/;
    const count = coll.countDocuments(filter);
    print(`${collName}.${field}: ${count} documents with z-prefix`);

    if (count > 0) {
      coll.find(filter).forEach(doc => {
        const update = {};
        update[field] = 'q' + doc[field].slice(1);
        coll.updateOne({_id: doc._id}, {$set: update});
      });
      print(`  -> migrated ${count} documents`);
    }

    // Uppercase Z → lowercase q
    const filterUpper = {};
    filterUpper[field] = /^Z/;
    const countUpper = coll.countDocuments(filterUpper);
    print(`${collName}.${field}: ${countUpper} documents with Z-prefix`);

    if (countUpper > 0) {
      coll.find(filterUpper).forEach(doc => {
        const update = {};
        update[field] = 'q' + doc[field].slice(1).toLowerCase();
        coll.updateOne({_id: doc._id}, {$set: update});
      });
      print(`  -> migrated ${countUpper} documents`);
    }
  }
}

print('Migration complete!');
```

### 3.3 Index rebuild

After migration, rebuild indexes on address fields:

```javascript
db.addresses.reIndex();
db.contractCode.reIndex();
db.tokenTransfers.reIndex();
db.tokenBalances.reIndex();
db.transactionByAddress.reIndex();
```

---

## Phase 4: The Actual Switch

Once all abstraction is in place, the switch is just changing constants:

1. **Syncer:** `validation/hex.go` → `const AddressPrefix = "Q"`
2. **Backend:** `db/helpers.go` → `const AddressPrefix = "Q"`
3. **Frontend:** `app/lib/helpers.ts` → `export const ADDRESS_PREFIX = 'Q'`
4. **Run MongoDB migration script**
5. **Rebuild & restart all services**

---

## Phase 5: Transition Period (Support Both)

During the transition, the system should accept both Z and Q prefixes from users:

- **SearchBar:** Accept both `Z...` and `Q...` as valid addresses
- **Backend routes:** Normalize both to the current canonical prefix
- **DB queries:** Use `addrVariants()` to search all prefix variants
- **Display:** Always show the canonical prefix (Q after migration)

This is already partially implemented (the codebase handles both Z and z). Extending to Q/q follows the same pattern.

---

## Deployment Order

1. Deploy **backend** with dual Z/Q support (Phase 1-2)
2. Deploy **frontend** with dual Z/Q support (Phase 1-2)
3. **Coordinate with node upgrade** — confirm node returns Q-prefix addresses
4. Deploy **syncer** with Q-prefix output
5. Run **MongoDB migration** for historical data
6. Flip the `AddressPrefix` constant to `"Q"`
7. Rebuild and restart all services
8. After stabilization, remove Z-prefix backward compat code (Phase 5 cleanup)

---

## Risk Mitigation

- **Backup MongoDB** before running migration: `mongodump --db qrldata-z`
- **Test in staging** with a copy of production data
- **Feature flag approach**: The `AddressPrefix` constant acts as a feature flag
- **Rollback**: Change constant back to `"Z"`, re-run migration in reverse
- **Monitor**: Watch PM2 logs for address-related errors after deployment

---

## File Count Summary

| Component | Files to Change | Critical | Already Compatible |
|-----------|----------------|----------|--------------------|
| Syncer (Zond2mongoDB) | 10 | 5 | 3 (transactions, coinbase, contracts) |
| Backend (backendAPI) | 6 | 4 | 0 |
| Frontend (ExplorerFrontend) | 6 | 3 | rest (depend on helpers.ts) |
| Infrastructure | 2 (Python scripts) | 2 | deploy scripts (no change needed) |
| **Total** | **24 files** | **14 critical** | **3 already compatible** |
