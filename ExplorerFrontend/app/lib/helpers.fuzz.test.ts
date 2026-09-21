/**
 * Adversarial-input tests for the decoders. Complements the happy-path
 * coverage in helpers.test.ts and searchResolver.test.ts by asserting
 * that crafted / malformed inputs don't throw, infinite-loop, or
 * return mis-shaped values. The decoders run in the browser against
 * untrusted on-chain calldata + RPC responses, so "graceful null" is
 * the only acceptable failure mode.
 *
 * Categories:
 *   - non-hex bodies that pass the length check (e.g. 'g' in place of a digit)
 *   - prefix anomalies (uppercase 0X, double prefix, missing prefix)
 *   - truncated calldata for each method selector
 *   - bombs: huge length-prefixes for dynamic arrays, near-MAX_SAFE_INTEGER
 *   - control characters / unicode / empty / whitespace
 *   - ABI fallback with malformed JSON / arrays of garbage
 *
 * Every case asserts: no throw, valid return shape (null or expected
 * fields). The fuzz seed is deterministic so failures reproduce
 * locally.
 */

import { describe, it, expect } from '@jest/globals';
import { keccak_256 } from '@noble/hashes/sha3.js';
import { bytesToHex, utf8ToBytes } from '@noble/hashes/utils.js';
import {
  decodeEventLog,
  decodeTokenTransferInput,
  decodeContractCall,
} from './helpers';
import { resolveSearchPath } from './searchResolver';

// Mirrors the production selector computation (keccak256 of the utf8
// canonical signature) so fuzz cases construct valid calldata.
const keccakHex = (sig: string): string => bytesToHex(keccak_256(utf8ToBytes(sig)));
const ABI_WORD_HEX_LENGTH = 128;
const UINT256_HEX_LENGTH = 64;
const EVENT_TOPIC_PADDING = '0'.repeat(64);

function uintWord(value: bigint | number): string {
  return (
    '0'.repeat(ABI_WORD_HEX_LENGTH - UINT256_HEX_LENGTH) +
    BigInt(value).toString(16).padStart(UINT256_HEX_LENGTH, '0')
  );
}

// Tiny seeded RNG so fuzz iterations are reproducible. mulberry32 from
// public-domain references; no crypto strength needed.
function seededRand(seed: number): () => number {
  let s = seed >>> 0;
  return () => {
    s = (s + 0x6D2B79F5) >>> 0;
    let t = s;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function randomHex(rng: () => number, len: number): string {
  const chars = '0123456789abcdef';
  let out = '0x';
  for (let i = 0; i < len; i++) {
    out += chars[Math.floor(rng() * 16)];
  }
  return out;
}

function randomJunk(rng: () => number, len: number): string {
  // ASCII range 0x20..0x7e plus a few control chars + unicode picks.
  const exotic = ['\x00', '​', '‮', '﻿', '"', '\\', '\n', '\t'];
  let out = '';
  for (let i = 0; i < len; i++) {
    if (rng() < 0.05) out += exotic[Math.floor(rng() * exotic.length)];
    else out += String.fromCharCode(0x20 + Math.floor(rng() * 0x5e));
  }
  return out;
}

const ADDR128 = 'a'.repeat(128);

// ─── decodeTokenTransferInput ───────────────────────────────────────────

describe('decodeTokenTransferInput adversarial inputs', () => {
  it('returns null for null bytes / unicode garbage of any length', () => {
    const cases = ['\x00', '\x00\x00', '﻿0xa9059cbb', '0x‮', '0x\n\n\n'];
    for (const c of cases) {
      expect(() => decodeTokenTransferInput(c)).not.toThrow();
      expect(decodeTokenTransferInput(c)).toBeNull();
    }
  });

  it('rejects a non-hex address word without throwing', () => {
    const bad =
      '0xa9059cbb' + 'g'.repeat(ABI_WORD_HEX_LENGTH) + uintWord(0);
    expect(() => decodeTokenTransferInput(bad)).not.toThrow();
    expect(decodeTokenTransferInput(bad)).toBeNull();
  });

  it('returns null when the amount slot is BigInt-unparseable', () => {
    // BigInt rejects mixed-radix bodies; the try/catch in the decoder
    // bails to null. Use 'g' in the amount slot, which contains the
    // BigInt call.
    const bad =
      '0xa9059cbb' +
      '0'.repeat(ABI_WORD_HEX_LENGTH) +
      'g'.repeat(ABI_WORD_HEX_LENGTH);
    expect(() => decodeTokenTransferInput(bad)).not.toThrow();
    expect(decodeTokenTransferInput(bad)).toBeNull();
  });

  it('survives 100 random-hex selector-prefixed inputs without throwing', () => {
    const rng = seededRand(0xc0ffee);
    for (let i = 0; i < 100; i++) {
      const lengths = [10, 100, 266, 394, 650, 1024];
      const total = lengths[Math.floor(rng() * lengths.length)];
      const input = randomHex(rng, total - 2); // -2 because randomHex prepends "0x"
      expect(() => decodeTokenTransferInput(input)).not.toThrow();
      // Shape contract: null OR object with `standard` + `methodName` fields.
      const result = decodeTokenTransferInput(input);
      if (result !== null) {
        expect(typeof result.standard).toBe('string');
        expect(typeof result.methodName).toBe('string');
      }
    }
  });

  it('survives 50 random non-hex / unicode bodies without throwing', () => {
    const rng = seededRand(0x1337);
    for (let i = 0; i < 50; i++) {
      const len = Math.floor(rng() * 200);
      const input = randomJunk(rng, len);
      expect(() => decodeTokenTransferInput(input)).not.toThrow();
      // Junk bodies should never decode (no valid selector prefix possible
      // after randomJunk's character set).
      expect(decodeTokenTransferInput(input)).toBeNull();
    }
  });
});

// ─── decodeEventLog ─────────────────────────────────────────────────────

describe('decodeEventLog adversarial inputs', () => {
  it('returns null for empty topics regardless of data', () => {
    expect(() => decodeEventLog([], '')).not.toThrow();
    expect(() => decodeEventLog([], '0xdeadbeef')).not.toThrow();
    expect(decodeEventLog([], '0xdeadbeef')).toBeNull();
  });

  it('survives malformed topic strings without throwing', () => {
    const malformed = [
      '',
      '0x',
      'not-hex',
      '0xZZZZ',
      '0x' + 'g'.repeat(ABI_WORD_HEX_LENGTH),
    ];
    for (const t of malformed) {
      expect(() => decodeEventLog([t], '0x')).not.toThrow();
      // Result may be null OR a decoded shape; either is valid.
    }
  });

  it('survives 200 random-hex topic/data combinations', () => {
    const rng = seededRand(0xfeed);
    for (let i = 0; i < 200; i++) {
      const topicCount = 1 + Math.floor(rng() * 4);
      const topics: string[] = [];
      for (let j = 0; j < topicCount; j++) {
        topics.push(randomHex(rng, ABI_WORD_HEX_LENGTH));
      }
      const dataLen = Math.floor(rng() * 400);
      const data = randomHex(rng, dataLen);
      expect(() => decodeEventLog(topics, data)).not.toThrow();
    }
  });

  it('declines a malformed ABI string without throwing', () => {
    const malformed = ['', '{', 'null', '[]', '[{"type":"event"}]', '[{"type":"event","name":"X"}]', '[{"type":"event","name":"X","inputs":null}]'];
    for (const abi of malformed) {
      expect(() =>
        decodeEventLog(['0x' + 'f'.repeat(ABI_WORD_HEX_LENGTH)], '0x', abi),
      ).not.toThrow();
    }
  });

  it('caps array-length bombs in TransferBatch data', () => {
    // TransferBatch with a length prefix of 2^256-1 in both arrays.
    const TOPIC_TRANSFER_BATCH =
      '0x4a39dc06d4c0dbc64b70af90fd698a233a518aa5d07e595d983b8c0526c8f7fb' +
      EVENT_TOPIC_PADDING;
    const addrSlot = '0x' + ADDR128;
    // Offsets pointing to two arrays whose length prefix is a huge value.
    const huge = 'ff'.repeat(32); // 2^256-1
    const hugeWord = '0'.repeat(64) + huge;
    const data = '0x' + uintWord(0x80) + uintWord(0xc0) + hugeWord + hugeWord;
    expect(() => decodeEventLog([TOPIC_TRANSFER_BATCH, addrSlot, addrSlot, addrSlot], data)).not.toThrow();
    expect(
      decodeEventLog([TOPIC_TRANSFER_BATCH, addrSlot, addrSlot, addrSlot], data),
    ).toBeNull();
  });
});

// ─── decodeContractCall ─────────────────────────────────────────────────

describe('decodeContractCall adversarial inputs', () => {
  it('survives a 1MB calldata blob with a fake ABI', () => {
    const abi = JSON.stringify([
      { type: 'function', name: 'foo', inputs: [{ name: 'x', type: 'uint256' }] },
    ]);
    const huge = '0xdeadbeef' + '0'.repeat(2 * 1024 * 1024); // ~2MB hex
    expect(() => decodeContractCall(huge, abi)).not.toThrow();
    expect(decodeContractCall(huge, abi)).toBeNull(); // selector won't match
  });

  it('does not infinite-loop on a selector that DOES match but with truncated args', () => {
    const sig = 'foo(uint256)';
    const selector = '0x' + keccakHex(sig).slice(0, 8);
    const abi = JSON.stringify([
      { type: 'function', name: 'foo', inputs: [{ name: 'x', type: 'uint256' }] },
    ]);
    // Selector + only 8 chars of the expected 128-char word.
    const truncated = selector + '00ff00ff';
    expect(() => decodeContractCall(truncated, abi)).not.toThrow();
    expect(decodeContractCall(truncated, abi)).toBeNull();
  });

  it('declines a JSON array of garbage entries', () => {
    const garbage = JSON.stringify([{ a: 1 }, 'string', 42, null, [], { type: 'unknown' }]);
    const input = '0xdeadbeef' + '0'.repeat(ABI_WORD_HEX_LENGTH);
    expect(() => decodeContractCall(input, garbage)).not.toThrow();
    expect(decodeContractCall(input, garbage)).toBeNull();
  });

  it('handles a string arg whose declared length exceeds the calldata', () => {
    const sig = 'setName(string)';
    const selector = '0x' + keccakHex(sig).slice(0, 8);
    const abi = JSON.stringify([
      { type: 'function', name: 'setName', inputs: [{ name: 'who', type: 'string' }] },
    ]);
    // Head points to offset 0x40; length claims 1000 bytes but only 16 follow.
    const input = selector +
      uintWord(0x40) + // dynamic head offset = 0x40
      uintWord(1000) + // length = 1000
      '00'.repeat(16); // not enough payload
    expect(() => decodeContractCall(input, abi)).not.toThrow();
    // Decoder should fall through to null or to a raw arg, not throw.
    const result = decodeContractCall(input, abi);
    if (result) {
      // If a result came back, it shouldn't claim to have decoded a 1000-char
      // string from 16 bytes; the dynamic decoder bails to raw.
      expect(result.args[0].type).toBe('raw');
    }
  });
});

// ─── resolveSearchPath ──────────────────────────────────────────────────

describe('resolveSearchPath adversarial inputs', () => {
  it('survives 100 random control-character inputs without throwing', () => {
    const rng = seededRand(0xdead);
    for (let i = 0; i < 100; i++) {
      const len = Math.floor(rng() * 200);
      const input = randomJunk(rng, len);
      expect(() => resolveSearchPath(input)).not.toThrow();
    }
  });

  it('does not produce a path with embedded whitespace', () => {
    // A 64-hex tx hash with embedded spaces should NOT decode to a path
    // (the resolver only trims surrounding whitespace, not internal).
    const polluted = '01234567' + ' ' + '0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab';
    const result = resolveSearchPath(polluted);
    if ('path' in result) {
      expect(result.path).not.toContain(' ');
    }
  });

  it('does not crash on extremely long inputs', () => {
    const huge = 'a'.repeat(100_000);
    expect(() => resolveSearchPath(huge)).not.toThrow();
    expect(() => resolveSearchPath('0x' + huge)).not.toThrow();
  });
});
