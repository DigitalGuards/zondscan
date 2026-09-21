import {
  DEFAULT_PREFERENCES,
  displayAddress,
  formatDateTime,
  isZeroTokenTransfer,
  parsePreferences,
  resolveTheme,
  unixSeconds,
} from './preferences';

describe('saved explorer preferences', () => {
  it.each([
    null,
    '',
    'invalid',
    'null',
    '[]',
    '42',
    '{"theme":"unknown","timeZone":"Mars","expandDetails":"true"}',
  ])('uses safe defaults for %p', (raw) => {
    expect(parsePreferences(raw)).toEqual(DEFAULT_PREFERENCES);
  });
  it('accepts known settings and discards unknown fields', () => {
    expect(
      parsePreferences(
        JSON.stringify({
          theme: 'dim',
          addressDisplay: 'back',
          timeZone: 'local',
          expandDetails: true,
          hideZeroTokenTransfers: false,
          highlightAddresses: false,
          network: 'untrusted',
        })
      )
    ).toEqual({
      theme: 'dim',
      currency: 'USD',
      locale: 'en',
      addressDisplay: 'back',
      timeZone: 'local',
      expandDetails: true,
      hideZeroTokenTransfers: false,
      highlightAddresses: false,
    });
  });
  it('resolves automatic appearance and preserves explicit choices', () => {
    expect(resolveTheme('system', true)).toBe('dark');
    expect(resolveTheme('system', false)).toBe('light');
    expect(resolveTheme('dim', false)).toBe('dim');
    expect(resolveTheme('dark', false)).toBe('dark');
  });
});

describe('display formatting', () => {
  it('shortens opaque addresses without altering the underlying value', () => {
    const address = `Q${'a'.repeat(34)}123456`;
    expect(displayAddress(address, 'middle')).toBe('Qaaaaaaa...123456');
    expect(displayAddress(address, 'back')).toBe('Qaaaaaaaaaaaaa...');
    expect(displayAddress('short', 'back')).toBe('short');
  });
  it('keeps compact lists on one line: two segments for 64-byte addresses in middle mode', () => {
    const address = `Q${'0123456789abcdef'.repeat(8)}`;
    expect(displayAddress(address, 'middle')).toBe(`${address.slice(0, 8)}...${address.slice(-6)}`);
    expect(displayAddress(address, 'back')).toBe(`${address.slice(0, 14)}...`);
  });
  it('accepts decimal and hex timestamps and rejects invalid dates', () => {
    expect(unixSeconds('0x6553f100')).toBe(1700000000);
    expect(unixSeconds('1700000000')).toBe(1700000000);
    for (const invalid of [null, undefined, '', 'bad', Infinity, -1, 9e12])
      expect(unixSeconds(invalid)).toBeNull();
    expect(formatDateTime(1700000000, 'utc')).toMatch(/14 Nov 2023.*22:13:20 UTC/);
  });
});

describe('zero-quantity token filtering', () => {
  it.each(['0', '000', '0.000', '0x0', '0X000', ' 0 '])('recognizes exact zero %s', (amount) => {
    expect(isZeroTokenTransfer({ amount, tokenStandard: 'ERC-20' })).toBe(true);
  });
  it.each(['0.000000000000000001', '1', '0x1', 'invalid', '', '0e0', '9007199254740993'])(
    'preserves nonzero or unrecognized quantity %s',
    (amount) => {
      expect(isZeroTokenTransfer({ amount, tokenStandard: 'ERC-20' })).toBe(false);
    }
  );
  it('preserves NFT transfers, missing amounts, and unrecognized standards', () => {
    for (const tokenStandard of ['ERC-721', 'ERC-1155', 'unknown'])
      expect(isZeroTokenTransfer({ amount: '0', tokenStandard })).toBe(false);
    expect(isZeroTokenTransfer({})).toBe(false);
    expect(isZeroTokenTransfer({ amount: 0 })).toBe(false);
  });
});
