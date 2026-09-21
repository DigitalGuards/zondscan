import { assertVm64AbiSupport, isVm64AbiCodec } from './vm64Abi';

const address = `Q${'ab'.repeat(64)}`;

describe('VM64 ABI capability gate', () => {
  it('accepts the locally composed VM64 dependency', () => {
    expect(() => assertVm64AbiSupport()).not.toThrow();
  });

  it('accepts a codec that round-trips one full 64-byte address word', () => {
    expect(
      isVm64AbiCodec({
        encodeParameter: (_type, value) => `0x${String(value).slice(1)}`,
        decodeParameter: (_type, encoded) => `Q${encoded.slice(2)}`,
      }),
    ).toBe(true);
  });

  it('rejects legacy, truncating, malformed, and throwing codecs', () => {
    expect(
      isVm64AbiCodec({
        encodeParameter: () => `0x${address.slice(1, 41).padStart(64, '0')}`,
        decodeParameter: () => address,
      }),
    ).toBe(false);
    expect(
      isVm64AbiCodec({
        encodeParameter: () => `0x${'00'.repeat(64)}`,
        decodeParameter: () => address,
      }),
    ).toBe(false);
    expect(
      isVm64AbiCodec({
        encodeParameter: () => {
          throw new Error('unsupported address');
        },
        decodeParameter: () => address,
      }),
    ).toBe(false);
  });
});
