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

  it('routes an uppercase 0X transaction hash like 0x', () => {
    const hash = 'ab'.repeat(32);
    expect(resolveSearchPath(`0X${hash}`)).toEqual({ path: `/tx/0x${hash}` });
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

  it.each([63, 65])('reports the actual count for "0x + %i hex"', (count) => {
    const result = resolveSearchPath('0x' + 'a'.repeat(count));
    expect(result).toEqual({
      error: `Transaction hash has ${count} hex characters after 0x; a transaction hash has 64.`,
    });
  });

  it.each([48, 60, 70, 80])('reports the actual count for a near-hash body of %i hex', (count) => {
    const result = resolveSearchPath('0x' + 'b'.repeat(count));
    expect('error' in result && result.error).toContain(`has ${count} hex characters after 0x`);
  });

  it('routes the reported QNS name hash to /tx/ unchanged', () => {
    const node = '0x2874b520ad0679a7c812be276fb0a234ca05373d48a02119afa825be09845e99';
    expect(resolveSearchPath(node)).toEqual({ path: `/tx/${node}` });
  });

  it.each([
    ['zero-width space', '\u200B'],
    ['zero-width non-joiner', '\u200C'],
    ['zero-width joiner', '\u200D'],
    ['word joiner', '\u2060'],
    ['byte-order mark', '\uFEFF'],
    ['left-to-right mark', '\u200E'],
    ['right-to-left mark', '\u200F'],
    ['left-to-right embedding', '\u202A'],
    ['pop directional formatting', '\u202C'],
    ['right-to-left override', '\u202E'],
    ['left-to-right isolate', '\u2066'],
    ['pop directional isolate', '\u2069'],
  ])('strips a %s anywhere in a tx hash', (_label, mark) => {
    const tx = '0x' + HASH64;
    const noisy = `${mark}${tx.slice(0, 20)}${mark}${tx.slice(20)}${mark}`;
    expect(resolveSearchPath(noisy)).toEqual({ path: `/tx/${tx}` });
  });

  it('strips no-break spaces around and inside a tx hash', () => {
    const tx = '0x' + HASH64;
    const noisy = `\u00A0${tx.slice(0, 30)}\u00A0${tx.slice(30)}\u00A0`;
    expect(resolveSearchPath(noisy)).toEqual({ path: `/tx/${tx}` });
  });

  it('joins a tx hash or address split by ordinary whitespace', () => {
    const tx = '0x' + HASH64;
    expect(resolveSearchPath(`${tx.slice(0, 34)} \n${tx.slice(34)}`)).toEqual({ path: `/tx/${tx}` });
    expect(resolveSearchPath(`Q${ADDR128.slice(0, 64)} ${ADDR128.slice(64)}`)).toEqual({
      path: `/address/${CHECKSUMMED_ADDRESS}`,
    });
  });

  it('keeps spaces inside short values', () => {
    const result = resolveSearchPath('12 34');
    expect('error' in result && result.error).toMatch(/Unrecognised/);
  });

  it('counts only visible hex after removing paste noise', () => {
    const result = resolveSearchPath('\u200B0x' + 'a'.repeat(64) + '\u200Ba');
    expect(result).toEqual({
      error: 'Transaction hash has 65 hex characters after 0x; a transaction hash has 64.',
    });
  });

  it('keeps unrelated non-ASCII characters', () => {
    const result = resolveSearchPath('caf\u00E9.qrl');
    expect('error' in result && result.error).toMatch(/Invalid QNS name/i);
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
