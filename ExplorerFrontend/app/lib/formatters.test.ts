/**
 * Pure-formatter unit tests. The decoders + search resolver already
 * have suites; this fills in coverage on the smaller utility helpers
 * that drive every visible amount / address / hash render across the
 * explorer. Each is a pure function with no React / DOM dependency.
 */

import { describe, it, expect } from '@jest/globals';
import {
  hexToBigInt,
  hexToNumber,
  formatBigGas,
  formatStaked,
  formatTokenAmount,
  truncateHash,
  formatAddress,
  smallestUnitToDecimal,
  decimalToSmallestUnit,
  compactTokenIDLabel,
  qNormaliseAbiValue,
  formatNumberWithCommas,
  normalizeHexString,
} from './helpers';

// ─── hex parsers ─────────────────────────────────────────────────────────

describe('hexToBigInt', () => {
  it('parses "0x..." hex correctly', () => {
    expect(hexToBigInt('0x10')).toBe(BigInt(16));
    expect(hexToBigInt('0xff')).toBe(BigInt(255));
  });

  it('parses non-prefixed hex', () => {
    expect(hexToBigInt('a')).toBe(BigInt(10));
  });

  it('returns 0n for empty / null / undefined', () => {
    expect(hexToBigInt('')).toBe(BigInt(0));
    expect(hexToBigInt(null)).toBe(BigInt(0));
    expect(hexToBigInt(undefined)).toBe(BigInt(0));
  });

  it('returns 0n on malformed input rather than throwing', () => {
    expect(hexToBigInt('not-hex')).toBe(BigInt(0));
  });
});

describe('hexToNumber', () => {
  it('parses small hex to JS number', () => {
    expect(hexToNumber('0x1a')).toBe(26);
  });

  it('returns 0 on empty / malformed input', () => {
    expect(hexToNumber('')).toBe(0);
    expect(hexToNumber(null)).toBe(0);
    expect(hexToNumber('garbage')).toBe(0);
  });
});

// ─── gas / staking formatters ────────────────────────────────────────────

describe('formatBigGas', () => {
  it('thousands-separates large gas counts', () => {
    // 0x100000 = 1048576
    expect(formatBigGas('0x100000')).toBe('1,048,576');
  });

  it('returns "0" for zero gas', () => {
    expect(formatBigGas('0x0')).toBe('0');
  });

  it('returns a sentinel on null input', () => {
    // The function's null-safe branch returns the comma sentinel that
    // surfaces as "missing" in the UI without throwing.
    expect(formatBigGas(null)).toBe(',');
    expect(formatBigGas(undefined)).toBe(',');
    expect(formatBigGas('')).toBe(',');
  });
});

describe('formatStaked', () => {
  it('converts Shor (1e9) to Quanta with trailing zeros stripped', () => {
    expect(formatStaked('1000000000')).toBe('1 Quanta');
    expect(formatStaked('1500000000')).toBe('1.5 Quanta');
  });

  it('handles whole-billion balances cleanly', () => {
    expect(formatStaked('32000000000')).toBe('32 Quanta');
  });

  it('returns "0 Quanta" for empty / malformed input', () => {
    expect(formatStaked('')).toBe('0 Quanta');
    expect(formatStaked('not-a-number')).toBe('0 Quanta');
  });
});

// ─── token amount + decimal conversions ──────────────────────────────────

describe('formatTokenAmount', () => {
  it('formats with the configured decimals', () => {
    // 1.5 tokens with 18 decimals = 1.5 * 10^18
    expect(formatTokenAmount('1500000000000000000', 18)).toBe('1.5');
    expect(formatTokenAmount('1000000000000000000', 18)).toBe('1');
  });

  it('keeps long whole parts comma-separated', () => {
    // 1 million tokens with 6 decimals = 1_000_000_000_000
    expect(formatTokenAmount('1000000000000', 6)).toBe('1,000,000');
  });

  it('returns "0" for zero / empty', () => {
    expect(formatTokenAmount('0', 18)).toBe('0');
    expect(formatTokenAmount('', 18)).toBe('0');
    expect(formatTokenAmount(null, 18)).toBe('0');
  });

  it('handles 0 decimals (raw integer)', () => {
    expect(formatTokenAmount('42', 0)).toBe('42');
  });

  it('returns input verbatim on parse failure', () => {
    // BigInt rejects "abc"; the function logs + returns the raw input.
    expect(formatTokenAmount('abc', 18)).toBe('abc');
  });
});

describe('smallestUnitToDecimal', () => {
  it('matches formatTokenAmount on the same input', () => {
    expect(smallestUnitToDecimal('1500000000000000000', 18)).toBe('1.5');
    expect(smallestUnitToDecimal('1000', 3)).toBe('1');
  });

  it('drops trailing zeros from fractional parts', () => {
    expect(smallestUnitToDecimal('1100000000000000000', 18)).toBe('1.1');
  });

  it('handles fractions smaller than 1', () => {
    expect(smallestUnitToDecimal('500000000000000', 18)).toBe('0.0005');
  });

  it('returns "0" for zero / empty', () => {
    expect(smallestUnitToDecimal('0', 18)).toBe('0');
    expect(smallestUnitToDecimal('', 18)).toBe('0');
  });
});

describe('decimalToSmallestUnit', () => {
  it('round-trips with smallestUnitToDecimal for typical values', () => {
    const decimals = 18;
    const smallest = decimalToSmallestUnit('1.5', decimals);
    expect(smallest).toBe('1500000000000000000');
    expect(smallestUnitToDecimal(smallest, decimals)).toBe('1.5');
  });

  it('handles inputs without a decimal point', () => {
    expect(decimalToSmallestUnit('5', 6)).toBe('5000000');
  });

  it('truncates fractional digits past the decimal limit (no rounding)', () => {
    // 6 decimals, input has 8 fractional digits, only first 6 kept.
    expect(decimalToSmallestUnit('1.12345678', 6)).toBe('1123456');
  });

  it('returns "0" for zero / empty', () => {
    expect(decimalToSmallestUnit('0', 18)).toBe('0');
    expect(decimalToSmallestUnit('', 18)).toBe('0');
  });
});

// ─── hash + address formatters ───────────────────────────────────────────

describe('truncateHash', () => {
  it('returns the input unchanged when shorter than start+end', () => {
    expect(truncateHash('0xabc')).toBe('0xabc');
    expect(truncateHash('short', 6, 4)).toBe('short');
  });

  it('truncates with the default 6+4 layout', () => {
    expect(truncateHash('0x1234567890abcdef1234567890abcdef')).toBe('0x1234...cdef');
  });

  it('respects custom start / end lengths', () => {
    expect(truncateHash('0x1234567890abcdef1234567890abcdef', 8, 6)).toBe('0x123456...90abcdef'.slice(0, 8) + '...' + '0x1234567890abcdef1234567890abcdef'.slice(-6));
  });

  it('returns empty string on null / undefined / empty', () => {
    expect(truncateHash(null)).toBe('');
    expect(truncateHash(undefined)).toBe('');
    expect(truncateHash('')).toBe('');
  });
});

describe('formatAddress', () => {
  it('canonicalises q-prefix to Q', () => {
    expect(formatAddress('qabc123')).toBe('Qabc123');
  });

  it('keeps 0x-prefixed contract addresses on 0x', () => {
    expect(formatAddress('0x7abcdef')).toBe('0x7abcdef');
  });

  it('converts 0x non-7 to Q', () => {
    expect(formatAddress('0xabc')).toBe('Qabc');
  });

  it('adds Q to a bare hex string', () => {
    expect(formatAddress('abcdef')).toBe('Qabcdef');
  });

  it('returns invalid input unchanged', () => {
    expect(formatAddress('not!hex')).toBe('not!hex');
  });

  it('returns empty string for null / undefined / empty', () => {
    expect(formatAddress(null)).toBe('');
    expect(formatAddress(undefined)).toBe('');
    expect(formatAddress('')).toBe('');
  });
});

// ─── token ID + ABI value helpers ────────────────────────────────────────

describe('compactTokenIDLabel', () => {
  it('returns the id unchanged when within maxLen', () => {
    expect(compactTokenIDLabel('42')).toBe('42');
    expect(compactTokenIDLabel('12345')).toBe('12345');
  });

  it('truncates with ellipsis when longer than maxLen', () => {
    // head 2 + ellipsis + tail 2 = 5 chars total for default maxLen.
    expect(compactTokenIDLabel('1234567890')).toBe('12…90');
  });

  it('returns "?" on empty / whitespace input', () => {
    expect(compactTokenIDLabel('')).toBe('?');
    expect(compactTokenIDLabel('   ')).toBe('?');
  });

  it('respects a custom maxLen', () => {
    expect(compactTokenIDLabel('123456789', 7)).toBe('1234…89');
  });
});

describe('qNormaliseAbiValue', () => {
  it('rewrites Z-prefixed addresses to Q', () => {
    expect(qNormaliseAbiValue('Z0123456789abcdef0123456789abcdef01234567', 'address'))
      .toBe('Q0123456789abcdef0123456789abcdef01234567');
  });

  it('leaves non-address types untouched', () => {
    expect(qNormaliseAbiValue('Z01234567', 'string')).toBe('Z01234567');
    expect(qNormaliseAbiValue(BigInt(42), 'uint256')).toBe(BigInt(42));
  });

  it('recurses into address arrays', () => {
    const addrs = ['Z0123456789abcdef0123456789abcdef01234567', 'Z9999999999999999999999999999999999999999'];
    expect(qNormaliseAbiValue(addrs, 'address[]')).toEqual([
      'Q0123456789abcdef0123456789abcdef01234567',
      'Q9999999999999999999999999999999999999999',
    ]);
  });

  it('recurses into nested address[][]', () => {
    const nested = [
      ['Z0123456789abcdef0123456789abcdef01234567'],
      ['Z9999999999999999999999999999999999999999'],
    ];
    expect(qNormaliseAbiValue(nested, 'address[][]')).toEqual([
      ['Q0123456789abcdef0123456789abcdef01234567'],
      ['Q9999999999999999999999999999999999999999'],
    ]);
  });

  it('returns Z-string unchanged when type doesn\'t mark it as address', () => {
    // Defensive: don't rewrite arbitrary strings that happen to start
    // with "Z" + 40 hex chars unless the ABI type says address.
    expect(qNormaliseAbiValue('Z0123456789abcdef0123456789abcdef01234567', 'bytes'))
      .toBe('Z0123456789abcdef0123456789abcdef01234567');
  });
});

// ─── misc formatters ─────────────────────────────────────────────────────

describe('formatNumberWithCommas', () => {
  it('inserts grouping separators', () => {
    expect(formatNumberWithCommas('1000000')).toBe('1,000,000');
    expect(formatNumberWithCommas(1234567)).toBe('1,234,567');
  });

  it('leaves small numbers untouched', () => {
    expect(formatNumberWithCommas('42')).toBe('42');
    expect(formatNumberWithCommas(0)).toBe('0');
  });

  it('returns "0" for null / undefined', () => {
    expect(formatNumberWithCommas(null)).toBe('0');
    expect(formatNumberWithCommas(undefined)).toBe('0');
  });
});

describe('normalizeHexString', () => {
  it('strips the 0x prefix', () => {
    expect(normalizeHexString('0xabc')).toBe('abc');
  });

  it('strips the Q prefix', () => {
    expect(normalizeHexString('Qabc')).toBe('abc');
    expect(normalizeHexString('qabc')).toBe('abc');
  });

  it('returns valid bare hex unchanged', () => {
    expect(normalizeHexString('abcdef')).toBe('abcdef');
  });

  it('returns empty string for empty / null', () => {
    expect(normalizeHexString('')).toBe('');
    expect(normalizeHexString(null)).toBe('');
    expect(normalizeHexString(undefined)).toBe('');
  });
});
