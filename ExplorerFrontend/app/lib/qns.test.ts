import { describe, expect, it } from '@jest/globals';

import { normalizeQnsName, QNS_MAX_NAME_BYTES } from './qns';

describe('QNS name normalization', () => {
  it.each([
    ['moscowchill.qrl', 'moscowchill.qrl'],
    ['MoscowChill.QRL', 'moscowchill.qrl'],
    ['wallet-2.alice.qrl', 'wallet-2.alice.qrl'],
    ['-wallet.qrl', '-wallet.qrl'],
    ['a--b.qrl', 'a--b.qrl'],
  ])('normalizes %s', (input, expected) => {
    expect(normalizeQnsName(input)).toBe(expected);
  });

  it.each([
    '',
    'qrl',
    '.qrl',
    'alice.qrl.',
    'alice..qrl',
    'alice.eth',
    'al ice.qrl',
    'al_ice.qrl',
    'café.qrl',
    'ab--cd.qrl',
    'xn--name.qrl',
  ])('rejects unsupported input %s', (input) => {
    expect(normalizeQnsName(input)).toBeNull();
  });

  it('matches the backend 255 byte ASCII route bound', () => {
    const maxName = `${'a'.repeat(QNS_MAX_NAME_BYTES - 4)}.qrl`;
    const tooLong = `${'a'.repeat(QNS_MAX_NAME_BYTES - 3)}.qrl`;

    expect(maxName).toHaveLength(QNS_MAX_NAME_BYTES);
    expect(normalizeQnsName(maxName)).toBe(maxName);
    expect(normalizeQnsName(tooLong)).toBeNull();
  });
});
