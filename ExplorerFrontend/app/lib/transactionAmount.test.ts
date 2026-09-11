import { formatTransactionAmount } from './transactionAmount';

describe('formatTransactionAmount', () => {
  it.each(['0', '0.000000000000000000', '0x0', 0])('displays a true zero: %s', (input) => {
    expect(formatTransactionAmount(input)).toEqual({ compact: '0', quanta: '0', planck: '0' });
  });

  it.each(['0.000000000000000016', '0x10', 1.6e-17])('preserves the tiny transfer: %s', (input) => {
    expect(formatTransactionAmount(input)).toEqual({
      compact: '<0.000001',
      quanta: '0.000000000000000016',
      planck: '16',
    });
  });

  it.each([
    ['0.000000000000000001', '<0.000001'],
    ['0.000000999999999999', '<0.000001'],
    ['0.000001000000000000', '0.000001'],
    ['0.010000000000000000', '0.01'],
    ['0.05', '0.05'],
    ['1.999999999999999999', '1.999999'],
    ['1000.123456789123456789', '1,000.123456'],
  ])('bounds the displayed fraction without rounding up: %s', (input, compact) => {
    expect(formatTransactionAmount(input)?.compact).toBe(compact);
  });

  it('preserves digits beyond Number precision', () => {
    expect(formatTransactionAmount('9007199254740993.000000000000000001')).toEqual({
      compact: '9,007,199,254,740,993',
      quanta: '9007199254740993.000000000000000001',
      planck: '9007199254740993000000000000000001',
    });
  });

  it('keeps decimal API amounts in Quanta regardless of magnitude', () => {
    expect(formatTransactionAmount('1000000000000000000')?.quanta).toBe('1000000000000000000');
  });

  it('converts the maximum RPC quantity exactly', () => {
    const planck = BigInt(2) ** BigInt(256) - BigInt(1);
    const result = formatTransactionAmount(`0x${planck.toString(16)}`);
    expect(result?.planck).toBe(planck.toString());
    expect(result?.quanta).toBe(
      '115792089237316195423570985008687907853269984665640564039457.584007913129639935'
    );
  });

  it.each([undefined, null, '', 'bad', '0x', '-1', NaN, Infinity, '1e-19', '1e999', '1e60'])(
    'leaves unavailable or invalid amounts unknown: %s',
    (input) => expect(formatTransactionAmount(input)).toBeNull()
  );
});
