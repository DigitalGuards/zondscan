/**
 * Pure resolver for the SearchBar input. Centralises the rules so the
 * component stays declarative and the rules can be unit-tested.
 *
 * Accepted input shapes (after trim + paste-noise cleanup):
 *   - decimal block number               → /block/<n>
 *   - "0x"-prefixed hex block number     → /block/0x<hex>     (≤ 16 hex chars)
 *   - 66-char tx hash ("0x" + 64 hex)    → /tx/<hash>
 *   - 64-char bare tx hash (no 0x)       → /tx/0x<hash>
 *   - Q/q + 128 hex chars                                     → /address/Q<checksum>
 *   - 0x/0X + 128 hex chars                                   → /address/Q<checksum>
 *   - 128 bare hex chars                                      → /address/Q<checksum>
 *   - conservative ASCII name below .qrl                      → /address/<normalized-name>
 *
 * Anything else returns an `error` string explaining which shape failed
 * and what was expected, so users can self-correct instead of seeing a
 * generic "Invalid input".
 */

import { TX_HASH_RE } from './helpers';
import { canonicalizeQrlAddress, hasQrlAddressShape } from './qrlAddress';
import { hasQnsSuffix, normalizeQnsName } from './qns';

export type SearchResolution =
  | { path: string }
  | { error: string };

const RE_DECIMAL = /^[0-9]+$/;
const RE_HEX_BLOCK = /^0x[0-9a-fA-F]{1,16}$/;
const RE_TX_HASH = TX_HASH_RE;
const RE_BARE_TX_HASH = /^[0-9a-fA-F]{64}$/;

export function resolveSearchPath(raw: string): SearchResolution {
  // Trim whitespace + strip a single leading slash so pasted /tx/<hash>
  // URLs from external sources don't double-up.
  const cleaned = raw.trim().replace(/^\/+/, '').replace(/\/+$/, '');
  if (!cleaned) {
    return { error: 'Enter an address, transaction hash, or block number.' };
  }

  if (RE_DECIMAL.test(cleaned)) {
    return { path: `/block/${cleaned}` };
  }

  if (RE_HEX_BLOCK.test(cleaned) && cleaned.length <= 18) {
    // Length cap 18 = "0x" + 16 hex chars (~uint64 max). Longer hex strings
    // that happen to be ≤66 chars fall through to the tx-hash check below.
    // For ambiguous lengths (e.g. 0x...8 hex chars could be either a tiny
    // hex block or a corrupt input), block lookup wins because the route
    // 404s gracefully if the block doesn't exist.
    return { path: `/block/${cleaned}` };
  }

  if (RE_TX_HASH.test(cleaned)) {
    return { path: `/tx/${cleaned}` };
  }

  if (RE_BARE_TX_HASH.test(cleaned)) {
    return { path: `/tx/0x${cleaned}` };
  }

  const canonicalAddress = canonicalizeQrlAddress(cleaned);
  if (canonicalAddress) {
    return { path: `/address/${canonicalAddress}` };
  }

  if (hasQrlAddressShape(cleaned)) {
    return { error: 'Address has an invalid QIP-55 mixed-case checksum.' };
  }

  const normalizedName = normalizeQnsName(cleaned);
  if (normalizedName) {
    return { path: `/address/${normalizedName}` };
  }

  if (hasQnsSuffix(cleaned)) {
    return { error: 'Invalid QNS name. Use the conservative ASCII .qrl name format.' };
  }

  // Per-shape diagnostics so the user knows what they almost got right.
  // Length-first since that's the most common paste mistake.
  if (cleaned.length === 65 && cleaned.startsWith('0x')) {
    return { error: 'Transaction hash is one character short (need 64 hex chars after 0x).' };
  }
  if (cleaned.length === 67 && cleaned.startsWith('0x')) {
    return { error: 'Transaction hash is one character too long (need 64 hex chars after 0x).' };
  }
  if (cleaned.length === 129 && /^[Qq]/.test(cleaned)) {
    return { error: 'Address has a non-hex character after the Q prefix.' };
  }
  if (cleaned.length === 130 && /^0[xX]/.test(cleaned)) {
    return { error: 'Address has a non-hex character after the 0x prefix.' };
  }
  return {
    error:
      'Unrecognised input. Expected a block number, transaction hash (0x + 64 hex), address (Q + 128 hex), or .qrl name.',
  };
}
