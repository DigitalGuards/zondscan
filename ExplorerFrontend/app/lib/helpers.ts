export function timeAgo(unixSeconds: number): string {
  const now = Math.floor(Date.now() / 1000);
  const diff = now - unixSeconds;
  if (diff < 0) return 'just now';
  if (diff < 60) return `${diff}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}

// Validator effective balances on the QRL beacon chain are Shor-denominated
// (10^-9 QRL / "Quanta"). Convert to whole-QRL display.
export function formatStaked(shor: string): string {
  try {
    const val = BigInt(shor || '0');
    const qrlBase = val / BigInt(1_000_000_000);
    const remainder = val % BigInt(1_000_000_000);
    const decimalStr = remainder.toString().padStart(9, '0').replace(/0+$/, '');
    const result = formatNumberWithCommas(qrlBase.toString());
    return decimalStr.length > 0 ? `${result}.${decimalStr} QRL` : `${result} QRL`;
  } catch {
    return '0 QRL';
  }
}

export function decodeToHex(input: string, format?: string): string {
  if (!input) return '';
  try {
    const binary = atob(input);
    const bytes = new Uint8Array(binary.length);
    for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
    return Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('');
  } catch {
    return '';
  }
}

export function toFixed(x: number | string | undefined | null): string {
  if (x === undefined || x === null) {
    return "0";
  }

  // Convert to number if it's a string
  const num = typeof x === 'string' ? parseFloat(x) : x;

  // Check if it's a valid number
  if (isNaN(num)) {
    return "0";
  }

  if (Math.abs(num) < 1.0) {
    const e = parseInt(num.toString().split('e-')[1]);
    if (e) {
      const val = num * Math.pow(10, e - 1);
      return '0.' + (new Array(e)).join('0') + val.toString().substring(2);
    }
  } else if (num.toString().includes('e+')) {
    // For large numbers in scientific notation, use BigInt to format correctly.
    // This assumes the number is effectively an integer.
    try {
      return BigInt(Math.round(num)).toString();
    } catch {
      // Fallback if not a clean integer
      return num.toString();
    }
  }
  return num.toString();
}

/**
 * Parse a "0x"-prefixed hex string into a bigint. Returns 0n on missing or
 * malformed input, gas-stats math should never throw on a dropped field.
 */
export function hexToBigInt(s: string | undefined | null): bigint {
  if (!s) return BigInt(0);
  try {
    return BigInt(s.startsWith('0x') ? s : '0x' + s);
  } catch {
    return BigInt(0);
  }
}

/**
 * Parse a "0x"-prefixed hex string into a number. Lossy for values > 2^53,
 * but block numbers / gas-used values fit comfortably in that range.
 */
export function hexToNumber(s: string | undefined | null): number {
  if (!s) return 0;
  try {
    return Number(BigInt(s.startsWith('0x') ? s : '0x' + s));
  } catch {
    return 0;
  }
}

/**
 * Format a hex gas count (gasUsed / gasLimit) as a thousands-separated decimal.
 */
export function formatBigGas(s: string | undefined | null): string {
  if (!s) return ',';
  const v = hexToBigInt(s);
  if (v === BigInt(0)) return '0';
  return v.toLocaleString('en-US');
}

// Convert a gas price expressed in Planck (10^-18 QRL) to Shor (10^-9 QRL).
// Callers append the " Shor" unit label themselves. Precision is adaptive:
// at typical mempool magnitudes (single-digit Shor) we want 2 decimals
// ("3.75 Shor"), not 9 ("3.750000012 Shor"). Sub-Shor values get more
// decimals so they don't round to 0; sub-mShor values fall through to
// exponential.
export function formatGasPrice(planck: number | string | undefined | null): string {
  if (planck === undefined || planck === null || planck === 0 || planck === '0' || planck === '0x0') {
    return '0';
  }
  try {
    const value = typeof planck === 'string' && planck.startsWith('0x') ? BigInt(planck) : BigInt(String(planck));
    const shor = Number(value) / 1e9;
    if (shor < 0.001) return shor.toExponential(2);
    if (shor < 1) return parseFloat(shor.toFixed(4)).toString();
    return shor.toFixed(2);
  } catch {
    return '0';
  }
}

// Format a Planck-denominated value with an adaptive unit suffix so the
// number is human-readable. Base fee on a quiet chain (≤ 7 Planck) shows
// as "7 Planck" rather than "7.00e-9 Shor"; a typical gas price (~3 Shor)
// shows as "3.13 Shor"; an oversized value would show in Quanta (= QRL).
// Returns [value_string, unit_string] so callers can style the unit
// independently.
export function formatPlanckAdaptive(planck: number | string | undefined | null): [string, string] {
  if (planck === undefined || planck === null || planck === 0 || planck === '0' || planck === '0x0') {
    return ['0', 'Planck'];
  }
  try {
    const v = typeof planck === 'string' && planck.startsWith('0x') ? BigInt(planck) : BigInt(String(planck));
    if (v === BigInt(0)) return ['0', 'Planck'];
    const SHOR = BigInt(1_000_000_000);
    const QUANTA = BigInt('1000000000000000000');
    if (v < SHOR) {
      return [v.toString(), 'Planck'];
    }
    if (v < QUANTA) {
      const shor = Number(v) / 1e9;
      return [parseFloat(shor.toFixed(9)).toString(), 'Shor'];
    }
    const quanta = Number(v) / 1e18;
    return [parseFloat(quanta.toFixed(9)).toString(), 'Quanta'];
  } catch {
    return ['0', 'Planck'];
  }
}

export function formatAmount(amount: number | string | undefined | null): [string, string] {
  // Handle undefined or null
  if (amount === undefined || amount === null) {
    return ['0.00', 'QRL'];
  }

  // Handle zero amount
  if (amount === 0 || amount === '0' || amount === '0x0') {
    return ['0.00', 'QRL'];
  }

  let totalNum: number;
  try {
    // Handle hex strings (e.g., "0x123") from node
    if (typeof amount === 'string' && amount.startsWith('0x')) {
      const value = BigInt(amount);
      const divisor = BigInt('1000000000000000000'); // 10^18
      const wholePart = value / divisor;
      const fractionalPart = value % divisor;
      totalNum = Number(wholePart) + Number(fractionalPart) / Number(divisor);
    }
    // Handle decimal numbers (convert to planck/shor format first)
    else if (typeof amount === 'number' || (typeof amount === 'string' && !isNaN(Number(amount)))) {
      const floatValue = parseFloat(String(amount));
      if (floatValue < 1000000000000000000) { // If number is already in QRL format
        totalNum = floatValue;
      } else {
        const value = BigInt(Math.floor(floatValue));
        const divisor = BigInt('1000000000000000000'); // 10^18
        const wholePart = value / divisor;
        const fractionalPart = value % divisor;
        totalNum = Number(wholePart) + Number(fractionalPart) / Number(divisor);
      }
    }
    // Handle other formats (assuming they're in planck/shor)
    else {
      throw new Error('Invalid amount format');
    }
  } catch (error) {
    console.error('Error converting amount:', error, amount);
    return ['0.00', 'QRL'];
  }

  // Format with appropriate decimal places, avoiding scientific notation
  if (totalNum === 0) {
    return ['0.00', 'QRL'];
  } else if (totalNum < 0.000001) {
    // For very small numbers, show all significant digits without trailing zeros
    return [totalNum.toFixed(18).replace(/\.?0+$/, ''), 'QRL'];
  } else if (totalNum < 1) {
    // For numbers less than 1, show up to 6 decimal places
    return [totalNum.toFixed(6).replace(/\.?0+$/, ''), 'QRL'];
  } else if (totalNum < 1000) {
    // For numbers between 1 and 999, show up to 4 decimal places
    return [totalNum.toFixed(4).replace(/\.?0+$/, ''), 'QRL'];
  } else {
    // For large numbers, show 2 decimal places
    return [totalNum.toFixed(2).replace(/\.?0+$/, ''), 'QRL'];
  }
}

export function normalizeHexString(hexData: string | undefined | null): string {
  if (!hexData) return '';

  // If it starts with 0x, remove the prefix
  if (typeof hexData === 'string' && hexData.startsWith('0x')) {
    return hexData.slice(2);
  }

  // If it starts with Q or q, remove the prefix
  if (typeof hexData === 'string' && (hexData.startsWith('Q') || hexData.startsWith('q'))) {
    return hexData.slice(1);
  }

  // If it's a valid hex string without prefix, return as is
  if (typeof hexData === 'string' && /^[0-9a-fA-F]+$/.test(hexData)) {
    return hexData;
  }

  console.error('Invalid hex string:', hexData);
  return '';
}

export function epochToISO(timestamp: number | undefined | null): string {
  if (!timestamp) return '1970-01-01';
  const date = new Date(timestamp * 1000);
  const datePart = date.toISOString().split('T')[0];
  return datePart;
}

export function formatTimestamp(timestamp: number | undefined | null): string {
  if (!timestamp) return '';
  const date = new Date(timestamp * 1000);
  // Use UTC to avoid hydration mismatch between server and client
  const day = date.getUTCDate();
  const month = date.getUTCMonth() + 1;
  const year = date.getUTCFullYear();
  const hours = date.getUTCHours();
  const minutes = date.getUTCMinutes().toString().padStart(2, '0');
  const seconds = date.getUTCSeconds().toString().padStart(2, '0');
  const ampm = hours >= 12 ? 'PM' : 'AM';
  const hour12 = hours % 12 || 12;
  return `${month}/${day}/${year}, ${hour12}:${minutes}:${seconds} ${ampm} UTC`;
}

export function formatNumberWithCommas(x: number | string | undefined | null): string {
  if (x === undefined || x === null) {
    return "0";
  }
  return x.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ",");
}

export function epochsToDays(epochs: number): number {
  // Each epoch is 128 slots
  // Each slot takes 60 seconds
  // So each epoch is 128 * 60 seconds
  // Convert to days
  return (epochs * 128 * 60) / (24 * 60 * 60);
}

export function truncateHash(hash: string | undefined | null, startLength = 6, endLength = 4): string {
  if (!hash || hash.length < startLength + endLength) return hash || '';
  return `${hash.slice(0, startLength)}...${hash.slice(-endLength)}`;
}

/**
 * Formats an address to ensure it has the correct prefix (Q for QRL addresses, 0x for contract addresses)
 */
export function formatAddress(address: string | undefined | null): string {
  if (!address) return '';

  // If already has Q/q prefix, normalize to uppercase Q
  if (address.startsWith('Q') || address.startsWith('q')) {
    return 'Q' + address.slice(1);
  }

  // If has 0x prefix
  if (address.startsWith('0x')) {
    // For contract addresses (starting with 0x7), keep the 0x prefix
    if (address.startsWith('0x7')) {
      return address;
    }
    // For regular addresses, convert to Q prefix
    return 'Q' + address.slice(2);
  }

  // If no prefix but is a valid hex string, add Q prefix
  if (/^[0-9a-fA-F]+$/.test(address)) {
    return 'Q' + address;
  }

  // If invalid format, return as is
  return address;
}

/**
 * Decoded token call (transfer-like or approval) from a transaction's
 * input data. The `standard` discriminator drives the badge in
 * pending-transaction-view; per-call shape is sparse: only the fields
 * that apply to the matched selector are populated. ERC-1155 batch is
 * the only path that fills `ids` + `values`.
 *
 * Caveat: 0x23b872dd is shared by ERC-20 transferFrom and ERC-721
 * transferFrom. Without the target contract we can't tell them apart,
 * so we tag it as ERC-20 (its original meaning) and leave the
 * ambiguity visible via the raw amount field. The user lands on the
 * confirmed tx page once mined, where the syncer's tokenStandard tag
 * resolves it correctly.
 */
export type DecodedTokenStandard = 'ERC-20' | 'ERC-721' | 'ERC-1155';

export interface DecodedTokenTransfer {
  standard: DecodedTokenStandard;
  methodName: 'transfer' | 'transferFrom' | 'safeTransferFrom' | 'safeBatchTransferFrom' | 'setApprovalForAll';
  to?: string;
  from?: string;
  /** ERC-20 raw uint256 amount, as a decimal string. */
  amount?: string;
  /** ERC-721 / ERC-1155 single tokenID, as a decimal string. */
  tokenID?: string;
  /** ERC-1155 single transfer value (qty), as a decimal string. */
  value?: string;
  /** ERC-1155 batch ids and per-id values (decimal strings). */
  ids?: string[];
  values?: string[];
  /** setApprovalForAll operator + flag. */
  operator?: string;
  approved?: boolean;
}

// Decode one 32-byte ABI slot as Q-prefixed address (last 20 bytes).
function abiAddress(data: string, offset: number): string {
  return 'Q' + data.slice(offset, offset + 64).slice(-40);
}

// Decode one 32-byte ABI slot as a uint256 decimal string.
function abiUint(data: string, offset: number): string {
  return BigInt('0x' + data.slice(offset, offset + 64)).toString();
}

// Decode a dynamic uint256[] starting at byte offset `arrOffset` (in
// chars from the start of the calldata, not from the args block).
// Returns the decoded values as decimal strings.
//
// Defence-in-depth: the length prefix comes from attacker-controlled
// calldata, so we cap it at 1024 (real ERC-1155 batches are
// orders of magnitude smaller, the largest mainnet batches we've
// seen are dozens of entries) and bound-check the declared payload
// against the actual data length. Without the cap a crafted tx
// could declare a 2^256-sized array and freeze the browser tab
// when this runs in the pending-tx view.
function abiUintArray(data: string, arrOffset: number): string[] {
  const lenHex = data.slice(arrOffset, arrOffset + 64);
  if (lenHex.length < 64) return [];
  const lenBig = BigInt('0x' + lenHex);
  const MAX_BATCH = 1024;
  if (lenBig > BigInt(MAX_BATCH)) return [];
  const len = Number(lenBig);
  if (arrOffset + 64 + len * 64 > data.length) return [];
  const out: string[] = [];
  for (let i = 0; i < len; i++) {
    out.push(abiUint(data, arrOffset + 64 + i * 64));
  }
  return out;
}

/**
 * Decode a transaction's input data into a structured token-call descriptor.
 * Returns null for any non-token-call input. Selectors covered:
 *
 *   ERC-20:   transfer (0xa9059cbb), transferFrom (0x23b872dd)
 *   ERC-721:  safeTransferFrom (0x42842e0e, 0xb88d4fde)
 *   ERC-1155: safeTransferFrom (0xf242432a), safeBatchTransferFrom (0x2eb2c2d6)
 *   Both NFT: setApprovalForAll (0xa22cb465)
 */
export function decodeTokenTransferInput(inputData: string | undefined | null): DecodedTokenTransfer | null {
  if (!inputData || inputData === '0x' || inputData.length < 10) {
    return null;
  }

  const data = inputData.toLowerCase();
  const selector = data.slice(0, 10);
  // ABI args start at byte 10 (skip "0x" + 4-byte selector).
  const args = 10;

  try {
    switch (selector) {
      // ─── ERC-20 ────────────────────────────────────────────────────────
      case '0xa9059cbb': {
        // transfer(address,uint256), 4 + 32 + 32 = 68 bytes → 138 chars
        if (data.length !== 138) return null;
        return {
          standard: 'ERC-20',
          methodName: 'transfer',
          to: abiAddress(data, args),
          amount: abiUint(data, args + 64),
        };
      }
      case '0x23b872dd': {
        // transferFrom(address,address,uint256), 4 + 32*3 = 100 bytes → 202 chars.
        // Shared selector with ERC-721 transferFrom, see DecodedTokenTransfer
        // docstring: we tag it as ERC-20 because we lack the contract's
        // standard in mempool context.
        if (data.length !== 202) return null;
        return {
          standard: 'ERC-20',
          methodName: 'transferFrom',
          from: abiAddress(data, args),
          to: abiAddress(data, args + 64),
          amount: abiUint(data, args + 128),
        };
      }

      // ─── ERC-721 ───────────────────────────────────────────────────────
      case '0x42842e0e': {
        // safeTransferFrom(address,address,uint256), same shape as 0x23b872dd
        // but ERC-721-only by selector, so the tokenID slot is unambiguous.
        if (data.length !== 202) return null;
        return {
          standard: 'ERC-721',
          methodName: 'safeTransferFrom',
          from: abiAddress(data, args),
          to: abiAddress(data, args + 64),
          tokenID: abiUint(data, args + 128),
        };
      }
      case '0xb88d4fde': {
        // safeTransferFrom(address,address,uint256,bytes). Static head is
        // 4 + 32*4 = 132 bytes; the trailing bytes payload is dynamic and
        // we don't surface it.
        if (data.length < args + 64 * 4) return null;
        return {
          standard: 'ERC-721',
          methodName: 'safeTransferFrom',
          from: abiAddress(data, args),
          to: abiAddress(data, args + 64),
          tokenID: abiUint(data, args + 128),
        };
      }

      // ─── ERC-1155 ──────────────────────────────────────────────────────
      case '0xf242432a': {
        // safeTransferFrom(address,address,uint256,uint256,bytes). Static
        // head is 4 + 32*5 = 164 bytes; data payload trails.
        if (data.length < args + 64 * 5) return null;
        return {
          standard: 'ERC-1155',
          methodName: 'safeTransferFrom',
          from: abiAddress(data, args),
          to: abiAddress(data, args + 64),
          tokenID: abiUint(data, args + 128),
          value: abiUint(data, args + 192),
        };
      }
      case '0x2eb2c2d6': {
        // safeBatchTransferFrom(address,address,uint256[],uint256[],bytes).
        // Static head: from, to, ids_offset, values_offset, data_offset.
        // Both arrays are dynamic and their offsets are relative to the
        // start of the *args block*, not the calldata. Convert to char
        // positions in the calldata.
        if (data.length < args + 64 * 5) return null;
        const from = abiAddress(data, args);
        const to = abiAddress(data, args + 64);
        const idsOff = Number(BigInt('0x' + data.slice(args + 128, args + 192))) * 2;
        const valsOff = Number(BigInt('0x' + data.slice(args + 192, args + 256))) * 2;
        const ids = abiUintArray(data, args + idsOff);
        const values = abiUintArray(data, args + valsOff);
        if (ids.length !== values.length) return null;
        return {
          standard: 'ERC-1155',
          methodName: 'safeBatchTransferFrom',
          from,
          to,
          ids,
          values,
        };
      }

      // ─── Both NFT standards ────────────────────────────────────────────
      case '0xa22cb465': {
        // setApprovalForAll(address,bool). Same selector for ERC-721 and
        // ERC-1155; we can't disambiguate from mempool input alone, so
        // we mark it ERC-721 (the older standard). Cosmetic only.
        if (data.length !== 138) return null;
        const operator = abiAddress(data, args);
        const flagSlot = BigInt('0x' + data.slice(args + 64, args + 128));
        return {
          standard: 'ERC-721',
          methodName: 'setApprovalForAll',
          operator,
          approved: flagSlot !== BigInt(0),
        };
      }

      default:
        return null;
    }
  } catch (error) {
    console.error('Error decoding token input:', selector, error);
    return null;
  }
}

/**
 * One decoded receipt-log argument. The frontend renders each arg per its
 * `type`: addresses get a /address link, uint256 gets a mono-formatted
 * value (locale-grouped when large), bool gets a badge, uint256[] gets a
 * batch list. Unknown types fall back to the raw `value` string.
 */
export interface DecodedEventArg {
  label: string;
  type: 'address' | 'uint256' | 'bool' | 'uint256[]';
  value?: string;
  values?: string[];
}

export interface DecodedEvent {
  /** Canonical event name (e.g. "Transfer", "ApprovalForAll"). */
  name: string;
  /** Event signature, e.g. "Transfer(address,address,uint256)". */
  signature: string;
  /** ERC-20 / ERC-721 / ERC-1155 hint for Transfer/Approval rendering. */
  standard?: 'ERC-20' | 'ERC-721' | 'ERC-1155';
  args: DecodedEventArg[];
}

// keccak256 topic hashes for the well-known token events. These are
// stable across every EVM chain that follows the standards.
const TOPIC_TRANSFER = '0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef';
const TOPIC_TRANSFER_SINGLE = '0xc3d58168c5ae7397731d063d5bbf3d657854427343f4c083240f7aacaa2d0f62';
const TOPIC_TRANSFER_BATCH = '0x4a39dc06d4c0dbc64b70af90fd698a233a518aa5d07e595d983b8c0526c8f7fb';
const TOPIC_APPROVAL = '0x8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925';
const TOPIC_APPROVAL_FOR_ALL = '0x17307eab39ab6107e8899845ad3d59bd9653f200f220920489ca2b5937696c31';

// Decode a 32-byte address slot ("0x" + 64 hex chars) to Q-prefix form.
function topicToAddress(t: string): string {
  if (!t) return '';
  const hex = t.startsWith('0x') ? t.slice(2) : t;
  return 'Q' + hex.slice(-40);
}

// Decode a 32-byte uint256 slot to a decimal string. Safe on missing input.
function topicToUint(t: string): string {
  if (!t) return '0';
  try {
    return BigInt(t.startsWith('0x') ? t : '0x' + t).toString();
  } catch {
    return '0';
  }
}

// Slice a uint256 out of an ABI-encoded data field at the given char offset
// (`data` is "0x..." prefixed; offset is from the start of the string).
function dataUint(data: string, charOffset: number): string {
  try {
    return BigInt('0x' + data.slice(charOffset, charOffset + 64)).toString();
  } catch {
    return '0';
  }
}

// Decode a dynamic uint256[] starting at `arrCharOffset` in `data`. Same
// bounded/safe shape as abiUintArray above. Returns [] on bad input.
function dataUintArray(data: string, arrCharOffset: number): string[] {
  const lenHex = data.slice(arrCharOffset, arrCharOffset + 64);
  if (lenHex.length < 64) return [];
  let lenBig: bigint;
  try {
    lenBig = BigInt('0x' + lenHex);
  } catch {
    return [];
  }
  const MAX_ITEMS = 1024;
  if (lenBig > BigInt(MAX_ITEMS)) return [];
  const len = Number(lenBig);
  if (arrCharOffset + 64 + len * 64 > data.length) return [];
  const out: string[] = [];
  for (let i = 0; i < len; i++) {
    out.push(dataUint(data, arrCharOffset + 64 + i * 64));
  }
  return out;
}

/**
 * Decode a single receipt-log topics/data pair into a labelled event.
 * Recognises the five well-known token signatures; returns null for
 * anything else so the view can render a raw fallback (topics + data).
 *
 * Disambiguation:
 *   - Transfer / Approval are shared between ERC-20 and ERC-721. ERC-721
 *     indexes all three params (4 topics, empty data); ERC-20 indexes the
 *     two addresses only (3 topics, value in data). We branch on
 *     topics.length to pick the right decoder + standard tag.
 */
export function decodeEventLog(topics: string[], data: string): DecodedEvent | null {
  if (!topics || topics.length === 0) return null;
  const sig = (topics[0] || '').toLowerCase();
  const safeData = (data || '0x').toLowerCase();

  switch (sig) {
    case TOPIC_TRANSFER: {
      // ERC-721 indexes the tokenId → 4 topics; ERC-20 keeps value in data.
      if (topics.length === 4) {
        return {
          name: 'Transfer',
          signature: 'Transfer(address from, address to, uint256 tokenId)',
          standard: 'ERC-721',
          args: [
            { label: 'from', type: 'address', value: topicToAddress(topics[1]) },
            { label: 'to', type: 'address', value: topicToAddress(topics[2]) },
            { label: 'tokenId', type: 'uint256', value: topicToUint(topics[3]) },
          ],
        };
      }
      return {
        name: 'Transfer',
        signature: 'Transfer(address from, address to, uint256 value)',
        standard: 'ERC-20',
        args: [
          { label: 'from', type: 'address', value: topicToAddress(topics[1]) },
          { label: 'to', type: 'address', value: topicToAddress(topics[2]) },
          { label: 'value', type: 'uint256', value: dataUint(safeData, 2) },
        ],
      };
    }

    case TOPIC_APPROVAL: {
      // Same ERC-20 vs ERC-721 split as Transfer.
      if (topics.length === 4) {
        return {
          name: 'Approval',
          signature: 'Approval(address owner, address approved, uint256 tokenId)',
          standard: 'ERC-721',
          args: [
            { label: 'owner', type: 'address', value: topicToAddress(topics[1]) },
            { label: 'approved', type: 'address', value: topicToAddress(topics[2]) },
            { label: 'tokenId', type: 'uint256', value: topicToUint(topics[3]) },
          ],
        };
      }
      return {
        name: 'Approval',
        signature: 'Approval(address owner, address spender, uint256 value)',
        standard: 'ERC-20',
        args: [
          { label: 'owner', type: 'address', value: topicToAddress(topics[1]) },
          { label: 'spender', type: 'address', value: topicToAddress(topics[2]) },
          { label: 'value', type: 'uint256', value: dataUint(safeData, 2) },
        ],
      };
    }

    case TOPIC_APPROVAL_FOR_ALL: {
      // ERC-721 + ERC-1155 share this signature exactly; we don't try to
      // tell them apart since the rendered fields are identical.
      let approved = false;
      try {
        approved = BigInt('0x' + safeData.slice(2, 66)) !== BigInt(0);
      } catch {
        approved = false;
      }
      return {
        name: 'ApprovalForAll',
        signature: 'ApprovalForAll(address owner, address operator, bool approved)',
        args: [
          { label: 'owner', type: 'address', value: topicToAddress(topics[1]) },
          { label: 'operator', type: 'address', value: topicToAddress(topics[2]) },
          { label: 'approved', type: 'bool', value: approved ? 'true' : 'false' },
        ],
      };
    }

    case TOPIC_TRANSFER_SINGLE: {
      // operator + from + to are indexed; data = abi.encode(id, value)
      return {
        name: 'TransferSingle',
        signature: 'TransferSingle(address operator, address from, address to, uint256 id, uint256 value)',
        standard: 'ERC-1155',
        args: [
          { label: 'operator', type: 'address', value: topicToAddress(topics[1]) },
          { label: 'from', type: 'address', value: topicToAddress(topics[2]) },
          { label: 'to', type: 'address', value: topicToAddress(topics[3]) },
          { label: 'id', type: 'uint256', value: dataUint(safeData, 2) },
          { label: 'value', type: 'uint256', value: dataUint(safeData, 66) },
        ],
      };
    }

    case TOPIC_TRANSFER_BATCH: {
      // operator + from + to indexed; data = abi.encode(uint256[] ids, uint256[] values).
      // Two leading offsets point to the length+elements of each array,
      // measured in bytes from the start of the args block. data is
      // "0x"-prefixed so we add 2 to convert byte offsets to char positions.
      const idsOff = (() => { try { return Number(BigInt('0x' + safeData.slice(2, 66))) * 2 + 2; } catch { return -1; } })();
      const valsOff = (() => { try { return Number(BigInt('0x' + safeData.slice(66, 130))) * 2 + 2; } catch { return -1; } })();
      const ids = idsOff > 0 ? dataUintArray(safeData, idsOff) : [];
      const values = valsOff > 0 ? dataUintArray(safeData, valsOff) : [];
      return {
        name: 'TransferBatch',
        signature: 'TransferBatch(address operator, address from, address to, uint256[] ids, uint256[] values)',
        standard: 'ERC-1155',
        args: [
          { label: 'operator', type: 'address', value: topicToAddress(topics[1]) },
          { label: 'from', type: 'address', value: topicToAddress(topics[2]) },
          { label: 'to', type: 'address', value: topicToAddress(topics[3]) },
          { label: 'ids', type: 'uint256[]', values: ids },
          { label: 'values', type: 'uint256[]', values: values },
        ],
      };
    }

    default:
      return null;
  }
}

/**
 * Converts a smallest-unit integer string to a decimal string using BigInt (no floating-point).
 * E.g. smallestUnitToDecimal("200000000000", 18) => "0.0000002"
 */
export function smallestUnitToDecimal(amount: string, decimals: number = 18): string {
  if (!amount || amount === '0') return '0';
  try {
    const raw = BigInt(amount);
    if (raw === BigInt(0)) return '0';
    const divisor = BigInt(10) ** BigInt(decimals);
    const wholePart = raw / divisor;
    const fractionalPart = raw % divisor;
    let fracStr = fractionalPart.toString().padStart(decimals, '0').replace(/0+$/, '');
    return fracStr ? `${wholePart}.${fracStr}` : wholePart.toString();
  } catch {
    return '0';
  }
}

/**
 * Converts a decimal string to a smallest-unit integer string using string manipulation (no floating-point).
 * E.g. decimalToSmallestUnit("0.0000002", 18) => "200000000000"
 */
export function decimalToSmallestUnit(value: string, decimals: number = 18): string {
  if (!value || value === '0') return '0';
  const parts = value.split('.');
  const intPart = parts[0] || '0';
  const fracPart = (parts[1] || '').slice(0, decimals).padEnd(decimals, '0');
  return (intPart + fracPart).replace(/^0+/, '') || '0';
}

/**
 * Formats a token amount with proper decimals
 * @param amount - The raw token amount (as string)
 * @param decimals - The number of decimals for the token
 * @returns Formatted amount string
 */
export function formatTokenAmount(amount: string | undefined | null, decimals: number = 18): string {
  if (!amount || amount === '0' || amount === '0x0') {
    return '0';
  }

  try {
    // BigInt handles both decimal strings and hex strings with 0x prefix
    const rawAmount = BigInt(amount);

    if (rawAmount === BigInt(0)) {
      return '0';
    }

    // Convert to decimal representation
    const divisor = BigInt(10 ** decimals);
    const wholePart = rawAmount / divisor;
    const fractionalPart = rawAmount % divisor;

    // Format the fractional part with leading zeros
    let fractionalStr = fractionalPart.toString().padStart(decimals, '0');
    // Remove trailing zeros
    fractionalStr = fractionalStr.replace(/0+$/, '');

    if (fractionalStr === '') {
      // Format whole part with thousand separators
      return wholePart.toLocaleString('en-US');
    }

    // Format whole part with thousand separators and add fractional part
    return `${wholePart.toLocaleString('en-US')}.${fractionalStr}`;
  } catch (error) {
    console.error('Error formatting token amount:', error, amount);
    return amount;
  }
}

/**
 * Compact representation of a long decimal tokenID for a small fixed-size
 * tile (e.g. a 40px monogram). Returns the id unchanged when it already
 * fits, otherwise an ellipsis-suffix form like "1234…78" that preserves
 * the most significant + least significant digits and signals truncation
 * explicitly. Empty input becomes "?".
 *
 * Used by the Tokens tab when a token's off-chain metadata image hasn't
 * been fetched yet and we render an id-based placeholder.
 */
export function compactTokenIDLabel(tokenID: string, maxLen = 5): string {
  const s = (tokenID ?? '').trim();
  if (!s) return '?';
  if (s.length <= maxLen) return s;
  // Show the first chunk + the last 2 chars + an ellipsis between them.
  const tail = 2;
  const head = Math.max(1, maxLen - tail - 1); // -1 for the ellipsis char
  return s.slice(0, head) + '…' + s.slice(-tail);
}

/**
 * Normalise an ABI-decoded value tree to use the canonical Q-prefix for
 * QRL v2 addresses. The 0.3.x @theqrl/web3-zond-abi decoder still emits
 * the legacy Z prefix for `address` outputs; every other surface in
 * zondscan uses Q. Mutually recursive on array types so nested shapes
 * like `address[][]` flatten correctly.
 */
export function qNormaliseAbiValue(v: unknown, type: string): unknown {
  if (type === 'address' && typeof v === 'string' && /^Z[0-9a-fA-F]{40}$/.test(v)) {
    return 'Q' + v.slice(1);
  }
  // Strip the last `[N]` or `[]` dimension and recurse so nested array
  // types (e.g. `address[][]`) get fully unwound.
  if (/\[\d*\]$/.test(type) && Array.isArray(v)) {
    const innerType = type.replace(/\[\d*\]$/, '');
    return v.map(x => qNormaliseAbiValue(x, innerType));
  }
  return v;
}
