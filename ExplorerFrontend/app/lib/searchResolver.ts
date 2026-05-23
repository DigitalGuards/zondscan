/**
 * Pure resolver for the SearchBar input. Centralises the rules so the
 * component stays declarative and the rules can be unit-tested.
 *
 * Accepted input shapes (after trim + paste-noise cleanup):
 *   - decimal block number               → /block/<n>
 *   - "0x"-prefixed hex block number     → /block/0x<hex>     (≤ 16 hex chars)
 *   - 66-char tx hash ("0x" + 64 hex)    → /tx/<hash>
 *   - 64-char bare tx hash (no 0x)       → /tx/0x<hash>
 *   - Q + 40 hex chars (case-insensitive Q prefix)            → /address/Q<hex>
 *   - 0x + 40 hex chars (contract / hex-form address)          → /address/<addr>
 *   - 40 bare hex chars (no prefix, ambiguous: treat as Q address)
 *
 * Anything else returns an `error` string explaining which shape failed
 * and what was expected, so users can self-correct instead of seeing a
 * generic "Invalid input".
 */

export type SearchResolution =
  | { path: string }
  | { error: string };

const RE_DECIMAL = /^[0-9]+$/;
const RE_HEX_BLOCK = /^0x[0-9a-fA-F]{1,16}$/;
const RE_TX_HASH = /^0x[0-9a-fA-F]{64}$/;
const RE_BARE_TX_HASH = /^[0-9a-fA-F]{64}$/;
const RE_Q_ADDR = /^[Qq][0-9a-fA-F]{40}$/;
const RE_HEX_ADDR = /^0x[0-9a-fA-F]{40}$/;
const RE_BARE_ADDR = /^[0-9a-fA-F]{40}$/;

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

  if (RE_Q_ADDR.test(cleaned)) {
    const normalized = 'Q' + cleaned.slice(1).toLowerCase();
    return { path: `/address/${normalized}` };
  }

  if (RE_HEX_ADDR.test(cleaned)) {
    return { path: `/address/${cleaned}` };
  }

  if (RE_BARE_ADDR.test(cleaned)) {
    return { path: `/address/Q${cleaned.toLowerCase()}` };
  }

  // Per-shape diagnostics so the user knows what they almost got right.
  // Length-first since that's the most common paste mistake.
  if (cleaned.length === 65 && cleaned.startsWith('0x')) {
    return { error: 'Transaction hash is one character short (need 64 hex chars after 0x).' };
  }
  if (cleaned.length === 67 && cleaned.startsWith('0x')) {
    return { error: 'Transaction hash is one character too long (need 64 hex chars after 0x).' };
  }
  if (cleaned.length === 41 && (cleaned.startsWith('Q') || cleaned.startsWith('q'))) {
    return { error: 'Address has a non-hex character after the Q prefix.' };
  }
  if (cleaned.length === 42 && cleaned.startsWith('0x')) {
    return { error: 'Address has a non-hex character after the 0x prefix.' };
  }
  return { error: 'Unrecognised input. Expected a block number, transaction hash (0x + 64 hex), or address (Q + 40 hex).' };
}
