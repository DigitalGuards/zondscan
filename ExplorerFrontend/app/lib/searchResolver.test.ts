/**
 * Search resolver tests. The SearchBar's logic was inline + untested
 * before iter 12 extracted it; this suite locks in the accepted-input
 * matrix so future tweaks (QNS names, etc.) don't silently break the
 * existing paste-and-navigate flows.
 */

import { describe, it, expect } from '@jest/globals';
import { resolveSearchPath } from './searchResolver';

const ADDR128 = 'a'.repeat(128);
const CHECKSUMMED_ADDRESS =
  'QaaaAAaaaAAaAaaAaAAAAaAAAaAaAaAAaAaaAaaaaAAAAAAAAaAAAAaAaAAaaAAaaaaAaAAAAaaAaAAaaaaaaAaAAaaaaAaAaaaaAaaaAAAAaAAAAaAaAaaAAAaAaaaAA';
const HASH64 = '0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef';

describe('resolveSearchPath', () => {
  it('rejects empty / whitespace-only input', () => {
    expect(resolveSearchPath('')).toEqual({ error: expect.stringMatching(/Enter/) });
    expect(resolveSearchPath('   ')).toEqual({ error: expect.stringMatching(/Enter/) });
    expect(resolveSearchPath('\t\n')).toEqual({ error: expect.stringMatching(/Enter/) });
  });

  it('decimal block number routes to /block/', () => {
    expect(resolveSearchPath('1')).toEqual({ path: '/block/1' });
    expect(resolveSearchPath('74647')).toEqual({ path: '/block/74647' });
  });

  it('hex block number routes to /block/', () => {
    expect(resolveSearchPath('0x1231b')).toEqual({ path: '/block/0x1231b' });
    expect(resolveSearchPath('0xFF')).toEqual({ path: '/block/0xFF' });
  });

  it('66-char "0x" + 64 hex routes to /tx/', () => {
    const tx = '0x' + HASH64;
    expect(resolveSearchPath(tx)).toEqual({ path: `/tx/${tx}` });
  });

  it('bare 64-char hex (no 0x) is normalised to /tx/0x...', () => {
    expect(resolveSearchPath(HASH64)).toEqual({ path: `/tx/0x${HASH64}` });
  });

  it('trims surrounding whitespace before resolving', () => {
    const tx = '0x' + HASH64;
    expect(resolveSearchPath(`  ${tx}  `)).toEqual({ path: `/tx/${tx}` });
    expect(resolveSearchPath('  123  ')).toEqual({ path: '/block/123' });
  });

  it('strips leading/trailing slashes (pasted URLs)', () => {
    expect(resolveSearchPath('/123')).toEqual({ path: '/block/123' });
    expect(resolveSearchPath('123/')).toEqual({ path: '/block/123' });
  });

  it('Q-prefixed aliases route to the canonical checksum address', () => {
    expect(resolveSearchPath(`Q${ADDR128}`)).toEqual({
      path: `/address/${CHECKSUMMED_ADDRESS}`,
    });
    expect(resolveSearchPath(`q${ADDR128.toUpperCase()}`)).toEqual({
      path: `/address/${CHECKSUMMED_ADDRESS}`,
    });
  });

  it('0x and 0X aliases route to the canonical checksum address', () => {
    expect(resolveSearchPath(`0x${ADDR128}`)).toEqual({
      path: `/address/${CHECKSUMMED_ADDRESS}`,
    });
    expect(resolveSearchPath(`0X${ADDR128.toUpperCase()}`)).toEqual({
      path: `/address/${CHECKSUMMED_ADDRESS}`,
    });
  });

  it('bare 128 hex chars route to the canonical checksum address', () => {
    expect(resolveSearchPath(ADDR128.toUpperCase())).toEqual({
      path: `/address/${CHECKSUMMED_ADDRESS}`,
    });
  });

  it('rejects an invalid mixed-case checksum', () => {
    const result = resolveSearchPath(`Q${'Ab'.repeat(64)}`);
    expect('error' in result && result.error).toMatch(/checksum/i);
  });

  it('routes a conservative .qrl name using its normalized spelling', () => {
    expect(resolveSearchPath('MoscowChill.QRL')).toEqual({
      path: '/address/moscowchill.qrl',
    });
    expect(resolveSearchPath('wallet-2.ALICE.QRL')).toEqual({
      path: '/address/wallet-2.alice.qrl',
    });
  });

  it('matches the SDK profile for hyphens in .qrl labels', () => {
    expect(resolveSearchPath('-moscowchill.qrl')).toEqual({
      path: '/address/-moscowchill.qrl',
    });
  });

  it.each(['alice..qrl', 'al_ice.qrl', 'xn--name.qrl', 'café.qrl'])(
    'does not route malformed .qrl input %s',
    (input) => {
      const result = resolveSearchPath(input);
      expect('error' in result && result.error).toMatch(/Invalid QNS name/i);
    },
  );

  it.each([
    ['127 hex characters', `Q${'a'.repeat(127)}`],
    ['129 hex characters', `Q${'a'.repeat(129)}`],
    ['a non-hex character', `Q${'a'.repeat(127)}z`],
    ['a legacy 20-byte address', `Q${'a'.repeat(40)}`],
  ])('rejects a Q-prefixed address with %s', (_label, address) => {
    expect(resolveSearchPath(address)).toEqual({ error: expect.any(String) });
  });

  it('reports a length-specific error for "0x + 63 hex" (one short)', () => {
    const result = resolveSearchPath('0x' + 'a'.repeat(63));
    expect('error' in result && result.error).toMatch(/one character short/i);
  });

  it('reports a length-specific error for "0x + 65 hex" (one too long)', () => {
    const result = resolveSearchPath('0x' + 'a'.repeat(65));
    expect('error' in result && result.error).toMatch(/one character too long/i);
  });

  it('reports a non-hex error for malformed Q-address of right length', () => {
    const result = resolveSearchPath('Q' + 'z'.repeat(128));
    expect('error' in result && result.error).toMatch(/non-hex/i);
  });

  it('reports a non-hex error for malformed 0x-address of right length', () => {
    const result = resolveSearchPath('0x' + 'z'.repeat(128));
    expect('error' in result && result.error).toMatch(/non-hex/i);
  });

  it('falls through to a generic message for unrecognised input', () => {
    const result = resolveSearchPath('hello world');
    expect('error' in result && result.error).toMatch(/Unrecognised/);
  });
});
