import { describe, expect, it, jest } from '@jest/globals';

jest.mock('server-only', () => ({}));

import { normalizeQrlAddress } from './faucet';

describe('normalizeQrlAddress', () => {
  const hex = 'a'.repeat(128);
  const checksum =
    'QaaaAAaaaAAaAaaAaAAAAaAAAaAaAaAAaAaaAaaaaAAAAAAAAaAAAAaAaAAaaAAaaaaAaAAAAaaAaAAaaaaaaAaAAaaaaAaAaaaaAaaaAAAAaAAAAaAaAaaAAAaAaaaAA';

  it('canonicalizes the intentionally accepted address aliases', () => {
    expect(normalizeQrlAddress(`Q${hex.toUpperCase()}`)).toBe(checksum);
    expect(normalizeQrlAddress(` q${hex.toUpperCase()} `)).toBe(checksum);
    expect(normalizeQrlAddress(`0x${hex}`)).toBe(checksum);
    expect(normalizeQrlAddress(`0X${hex.toUpperCase()}`)).toBe(checksum);
    expect(normalizeQrlAddress(hex.toUpperCase())).toBe(checksum);
  });

  it.each([
    ['127 hexadecimal characters', `Q${'a'.repeat(127)}`],
    ['129 hexadecimal characters', `Q${'a'.repeat(129)}`],
    ['a non-hex character', `Q${'a'.repeat(127)}z`],
    ['a legacy 20-byte address', `Q${'a'.repeat(40)}`],
    ['an invalid mixed-case checksum', `Q${'Ab'.repeat(64)}`],
    ['an empty value', ''],
  ])('rejects an address with %s', (_label, address) => {
    expect(normalizeQrlAddress(address)).toBeNull();
  });
});
