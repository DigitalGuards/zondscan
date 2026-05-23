/**
 * Search resolver tests. The SearchBar's logic was inline + untested
 * before iter 12 extracted it; this suite locks in the accepted-input
 * matrix so future tweaks (QNS names, etc.) don't silently break the
 * existing paste-and-navigate flows.
 */

import { describe, it, expect } from '@jest/globals';
import { resolveSearchPath } from './searchResolver';

const ADDR40 = 'aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa';
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

  it('Q-prefixed address routes to /address/ with Q normalised + lowercase hex', () => {
    expect(resolveSearchPath(`Q${ADDR40}`)).toEqual({ path: `/address/Q${ADDR40}` });
    expect(resolveSearchPath(`q${ADDR40.toUpperCase()}`)).toEqual({ path: `/address/Q${ADDR40}` });
  });

  it('0x-prefixed address routes to /address/ verbatim', () => {
    expect(resolveSearchPath(`0x${ADDR40}`)).toEqual({ path: `/address/0x${ADDR40}` });
  });

  it('bare 40 hex chars routes to /address/Q...', () => {
    expect(resolveSearchPath(ADDR40)).toEqual({ path: `/address/Q${ADDR40}` });
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
    const result = resolveSearchPath('Q' + 'z'.repeat(40));
    expect('error' in result && result.error).toMatch(/non-hex/i);
  });

  it('reports a non-hex error for malformed 0x-address of right length', () => {
    const result = resolveSearchPath('0x' + 'z'.repeat(40));
    expect('error' in result && result.error).toMatch(/non-hex/i);
  });

  it('falls through to a generic message for unrecognised input', () => {
    const result = resolveSearchPath('hello world');
    expect('error' in result && result.error).toMatch(/Unrecognised/);
  });
});
