/**
 * Pure resolver for the SearchBar input. Centralises the rules so the
 * component stays declarative and the rules can be unit-tested.
 *
 * Paste-noise cleanup removes zero-width characters, byte-order marks and
 * bidi controls anywhere in the input, turns no-break spaces into spaces,
 * trims, strips surrounding slashes, and removes whitespace inside a value
 * that is otherwise a hash- or address-length hex string.
 *
 * Accepted input shapes (after that cleanup):
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
const RE_PREFIXED_HEX_BODY = /^0[xX]([0-9a-fA-F]+)$/;
// Hash or address shaped: optional prefix, then a long hex body. Short
// values such as "12 34" keep their spaces and stay unrecognised.
const RE_LONG_HEX_LIKE = /^(0[xX]|[Qq])?[0-9a-fA-F]{48,}$/;
const TX_HASH_HEX_LENGTH = 64;
// A body this close to 64 hex characters is most likely a mangled
// transaction hash; longer bodies belong to the address diagnostics.
// Keep the lower bound equal to the RE_LONG_HEX_LIKE minimum.
const TX_HASH_NEAR_MIN = 48;
const TX_HASH_NEAR_MAX = 80;

// Zero-width space/non-joiner/joiner, word joiner, byte-order mark, and the
// bidi marks, embeddings, overrides and isolates that copy/paste carries.
const RE_INVISIBLE = /[\u200B-\u200F\u202A-\u202E\u2060\u2066-\u2069\uFEFF]/g;
const RE_NBSP = /[\u00A0\u202F]/g;

/**
 * Remove paste noise: invisible characters, no-break spaces, surrounding
 * whitespace and slashes, and whitespace inside a hex value.
 */
export function cleanSearchInput(raw: string): string {
  const visible = raw.replace(RE_INVISIBLE, '').replace(RE_NBSP, ' ');
  // Strip surrounding slashes so pasted /tx/<hash> URLs don't double up.
  const trimmed = visible.trim().replace(/^\/+/, '').replace(/\/+$/, '').trim();
  const compact = trimmed.replace(/\s+/g, '');
  const joined = compact !== trimmed && RE_LONG_HEX_LIKE.test(compact) ? compact : trimmed;
  // Hashes route on a lowercase 0x prefix; an uppercase 0X is the same value.
  return /^0X[0-9a-fA-F]+$/.test(joined) ? `0x${joined.slice(2)}` : joined;
}

export function resolveSearchPath(raw: string): SearchResolution {
  const cleaned = cleanSearchInput(raw);
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
  const hexBody = RE_PREFIXED_HEX_BODY.exec(cleaned)?.[1];
  if (
    hexBody !== undefined &&
    hexBody.length >= TX_HASH_NEAR_MIN &&
    hexBody.length <= TX_HASH_NEAR_MAX
  ) {
    const count = hexBody.length;
    const unit = count === 1 ? 'character' : 'characters';
    return {
      error: `Transaction hash has ${count} hex ${unit} after 0x; a transaction hash has ${TX_HASH_HEX_LENGTH}.`,
    };
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
