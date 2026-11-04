import * as abi from '@theqrl/web3-qrl-abi';
import { decodeContractRead, parseContractArg } from './contractCall';
import { canonicalizeQrlAddress } from './qrlAddress';

const BODY = 'ab'.repeat(64);
const transfer = {
  type: 'function',
  name: 'transfer',
  stateMutability: 'nonpayable',
  inputs: [
    { name: 'to', type: 'address' },
    { name: 'amount', type: 'uint256' },
  ],
  outputs: [],
} as const;

function checksummedBody(): string {
  const canonical = canonicalizeQrlAddress(`Q${BODY}`);
  if (!canonical) throw new Error('fixture address must be valid');
  const body = canonical.slice(1);
  if (body === body.toLowerCase() || body === body.toUpperCase()) {
    throw new Error('fixture checksum must be mixed case');
  }
  return body;
}

describe('parseContractArg', () => {
  it('keeps the input shapes the Read and Write tabs accept', () => {
    expect(parseContractArg('', 'bool')).toBe(false);
    expect(parseContractArg(' true ', 'bool')).toBe(true);
    expect(parseContractArg('TRUE', 'bool')).toBe(true);
    expect(parseContractArg('1', 'bool')).toBe(true);
    expect(parseContractArg('false', 'bool')).toBe(false);
    expect(parseContractArg('', 'uint256')).toBe('0');
    expect(parseContractArg('', 'int8')).toBe('0');
    expect(parseContractArg(' 42 ', 'uint256')).toBe('42');
    expect(parseContractArg('', 'string')).toBe('');
    expect(parseContractArg(' text ', 'string')).toBe('text');
    expect(parseContractArg('0x1234', 'bytes32')).toBe('0x1234');
    expect(parseContractArg('[1,2]', 'uint256[]')).toEqual([1, 2]);
    expect(parseContractArg('[1,"x"]', 'tuple')).toEqual([1, 'x']);
    expect(() => parseContractArg('[1,', 'uint256[]')).toThrow(
      'expected JSON for uint256[]: e.g. [1,2,3]'
    );
  });

  it('gives a lowercase q address with a uniform-case body the capital Q', () => {
    const expected = abi.encodeFunctionCall(transfer, [`Q${BODY}`, '1']);
    for (const body of [BODY, BODY.toUpperCase()]) {
      const parsed = parseContractArg(` q${body} `, 'address');
      expect(parsed).toBe(`Q${body}`);
      expect(abi.encodeFunctionCall(transfer, [parsed, '1'])).toBe(expected);
    }
    expect(parseContractArg(`["q${BODY}", "Q${BODY}"]`, 'address[]')).toEqual([
      `Q${BODY}`,
      `Q${BODY}`,
    ]);
    expect(parseContractArg(`[["q${BODY}"]]`, 'address[2][]')).toEqual([[`Q${BODY}`]]);
  });

  it('passes every other address spelling on as typed', () => {
    const mixed = checksummedBody();
    for (const typed of [`Q${BODY}`, `Q${mixed}`, `q${mixed}`, `0x${BODY}`, BODY]) {
      expect(parseContractArg(typed, 'address')).toBe(typed);
    }
    // A checksummed body behind a lowercase q stays refused, as on 1.0.3.
    expect(() =>
      abi.encodeFunctionCall(transfer, [parseContractArg(`q${mixed}`, 'address'), '1'])
    ).toThrow();
    // Only address inputs change: the same text for another type is untouched.
    expect(parseContractArg(`q${BODY}`, 'string')).toBe(`q${BODY}`);
    expect(parseContractArg(`["q${BODY}"]`, 'string[]')).toEqual([`q${BODY}`]);
  });
});

describe('decodeContractRead', () => {
  const word = (hex: string) => hex.padStart(128, '0');

  it('formats a canonical reply', () => {
    expect(
      decodeContractRead(
        [
          { name: 'ok', type: 'bool' },
          { name: '', type: 'uint256' },
        ],
        `0x${word('1')}${word('2a')}`
      )
    ).toEqual({ decoded: true, text: JSON.stringify({ ok: true, _1: '42' }, null, 2) });
  });

  it('shows a decoded address in the explorer canonical checksum form', () => {
    const canonical = canonicalizeQrlAddress(`Q${BODY}`);
    expect(canonical).toBe(`Q${checksummedBody()}`);
    expect(decodeContractRead([{ name: 'owner', type: 'address' }], `0x${BODY}`)).toEqual({
      decoded: true,
      text: JSON.stringify({ owner: canonical }, null, 2),
    });
  });

  it.each([
    ['bool', word('2')],
    ['uint8', word('1ff')],
    ['bytes32', `${'11'.repeat(32)}01${'00'.repeat(31)}`],
  ])('returns a non-canonical %s reply undecoded with its raw hex', (type, body) => {
    const raw = `0x${body}`;
    const read = decodeContractRead([{ name: 'value', type }], raw);
    if (read.decoded) throw new Error(`the codec accepted a non-canonical ${type} reply`);
    expect(read.raw).toBe(raw);
    expect(read.error).toContain(`The reply does not decode as (${type}): `);
  });
});
