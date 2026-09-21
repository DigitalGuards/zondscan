import { describe, expect, it } from '@jest/globals';

import {
  canonicalizeQrlAddress,
  compactQrlAddress,
  hasQrlAddressShape,
  isValidQrlAddress,
} from './qrlAddress';

const LOWER_BODY = 'a'.repeat(128);
const CHECKSUMMED_ADDRESS =
  'QaaaAAaaaAAaAaaAaAAAAaAAAaAaAaAAaAaaAaaaaAAAAAAAAaAAAAaAaAAaaAAaaaaAaAAAAaaAaAAaaaaaaAaAAaaaaAaAaaaaAaaaAAAAaAAAAaAaAaaAAAaAaaaAA';

describe('QIP-55 address canonicalization', () => {
  it.each([
    `Q${LOWER_BODY}`,
    `q${LOWER_BODY}`,
    `0x${LOWER_BODY}`,
    `0X${LOWER_BODY.toUpperCase()}`,
    LOWER_BODY,
    LOWER_BODY.toUpperCase(),
    CHECKSUMMED_ADDRESS,
    `q${CHECKSUMMED_ADDRESS.slice(1)}`,
  ])('canonicalizes the supported alias %s', (address) => {
    expect(canonicalizeQrlAddress(address)).toBe(CHECKSUMMED_ADDRESS);
    expect(isValidQrlAddress(address)).toBe(true);
  });

  it('trims surrounding whitespace without accepting embedded whitespace', () => {
    expect(canonicalizeQrlAddress(` 0x${LOWER_BODY} `)).toBe(CHECKSUMMED_ADDRESS);
    expect(canonicalizeQrlAddress(`Q${LOWER_BODY.slice(0, 64)} ${LOWER_BODY.slice(64)}`)).toBeNull();
  });

  it('rejects a mixed-case body with an invalid SHAKE256 checksum', () => {
    const invalidMixedCase = `Q${'Ab'.repeat(64)}`;
    expect(hasQrlAddressShape(invalidMixedCase)).toBe(true);
    expect(canonicalizeQrlAddress(invalidMixedCase)).toBeNull();
    expect(isValidQrlAddress(invalidMixedCase)).toBe(false);
  });

  it.each([
    ['Q127', `Q${'a'.repeat(127)}`],
    ['Q129', `Q${'a'.repeat(129)}`],
    ['legacy Q40', `Q${'a'.repeat(40)}`],
    ['non-hex', `Q${'a'.repeat(127)}z`],
    ['empty', ''],
  ])('rejects %s', (_label, address) => {
    expect(hasQrlAddressShape(address)).toBe(false);
    expect(canonicalizeQrlAddress(address)).toBeNull();
  });

  it('builds a canonical first, middle, and final fingerprint for every alias', () => {
    const body = CHECKSUMMED_ADDRESS.slice(1);
    const middleStart = Math.floor((body.length - 8) / 2);
    const expected =
      `Q${body.slice(0, 8)}...` +
      `${body.slice(middleStart, middleStart + 8)}...` +
      body.slice(-8);

    expect(compactQrlAddress(`0X${LOWER_BODY.toUpperCase()}`)).toBe(expected);
  });

  it('leaves malformed and legacy-width display values unchanged', () => {
    const legacy = `Q${'a'.repeat(40)}`;
    expect(compactQrlAddress(legacy)).toBe(legacy);
    expect(compactQrlAddress(null)).toBe('');
  });
});
