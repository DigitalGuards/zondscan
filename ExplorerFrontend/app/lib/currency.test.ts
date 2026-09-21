import {
  CURRENCIES,
  EXCHANGE_RATES_MAX_AGE_MS,
  formatFiat,
  parseExchangeRates,
  parseFrankfurterRates,
} from './currency';

const now = Date.parse('2026-09-10T12:00:00Z');
const sourceRows = [
  { base: 'USD', quote: 'EUR', rate: 0.85 },
  { base: 'USD', quote: 'GBP', rate: 0.74 },
  { base: 'USD', quote: 'CHF', rate: 0.8 },
  { base: 'USD', quote: 'CAD', rate: 1.37 },
  { base: 'USD', quote: 'AUD', rate: 1.38 },
  { base: 'USD', quote: 'JPY', rate: 153 },
  { base: 'USD', quote: 'CNY', rate: 6.7 },
].map((row) => ({ ...row, date: '2026-09-09' }));

describe('exchange rate validation', () => {
  it('preserves actual rates and adds the USD identity rate', () => {
    const snapshot = parseFrankfurterRates(sourceRows, now);
    expect(snapshot).toEqual({
      base: 'USD',
      date: '2026-09-09',
      rates: { USD: 1, EUR: 0.85, GBP: 0.74, CHF: 0.8, CAD: 1.37, AUD: 1.38, JPY: 153, CNY: 6.7 },
    });
    expect(parseExchangeRates(snapshot, now)).toEqual(snapshot);
    expect(Object.keys(snapshot!.rates).sort()).toEqual(CURRENCIES.map(({ code }) => code).sort());
  });

  it('rejects a missing quote, duplicate quote, or unexpected currency', () => {
    expect(parseFrankfurterRates(sourceRows.slice(1), now)).toBeNull();
    expect(parseFrankfurterRates([...sourceRows.slice(1), sourceRows[1]], now)).toBeNull();
    expect(
      parseFrankfurterRates([{ ...sourceRows[0], quote: 'ABC' }, ...sourceRows.slice(1)], now)
    ).toBeNull();
  });

  it('requires the USD base and a shared date across every quote', () => {
    expect(
      parseFrankfurterRates([{ ...sourceRows[0], base: 'EUR' }, ...sourceRows.slice(1)], now)
    ).toBeNull();
    expect(
      parseFrankfurterRates([{ ...sourceRows[0], date: '2026-09-08' }, ...sourceRows.slice(1)], now)
    ).toBeNull();
  });

  it.each([0, -1, NaN, Infinity, 1_000_000, '0.85', null])(
    'rejects the malformed rate %s',
    (rate) => {
      expect(
        parseFrankfurterRates([{ ...sourceRows[0], rate }, ...sourceRows.slice(1)], now)
      ).toBeNull();
    }
  );

  it.each(['2026-02-30', '2026-08-01', '2026-09-11', '09/09/2026', '', null])(
    'rejects invalid, expired, and future rate dates: %s',
    (date) => {
      expect(
        parseFrankfurterRates(
          sourceRows.map((row) => ({ ...row, date })),
          now
        )
      ).toBeNull();
    }
  );

  it('permits weekends and holidays within the reference rate age limit', () => {
    const published = Date.parse('2026-09-09T00:00:00Z');
    expect(parseFrankfurterRates(sourceRows, published + EXCHANGE_RATES_MAX_AGE_MS)).not.toBeNull();
    expect(parseFrankfurterRates(sourceRows, published + EXCHANGE_RATES_MAX_AGE_MS + 1)).toBeNull();
  });

  it('validates the same invariants on the browser-facing snapshot', () => {
    const valid = parseFrankfurterRates(sourceRows, now)!;
    expect(parseExchangeRates({ ...valid, base: 'EUR' }, now)).toBeNull();
    expect(parseExchangeRates({ ...valid, rates: { ...valid.rates, USD: 2 } }, now)).toBeNull();
    expect(
      parseExchangeRates({ ...valid, rates: { ...valid.rates, EUR: undefined } }, now)
    ).toBeNull();
    expect(parseExchangeRates({ ...valid, rates: [] }, now)).toBeNull();
    expect(parseExchangeRates(null, now)).toBeNull();
    expect(parseExchangeRates([], now)).toBeNull();
  });
});

describe('fiat presentation', () => {
  it('displays an explicit currency code and respects the chosen locale', () => {
    expect(formatFiat(1234.56, 'CAD', 'en-US')).toMatch(/CAD\s+1,234\.56/);
    expect(formatFiat(12345.67, 'EUR', 'es')).toMatch(/12\.345,67\s+EUR/);
    expect(formatFiat(1234.56, 'CNY', 'zh')).toContain('CNY');
    expect(formatFiat(1234.56, 'USD', 'ru')).toContain('USD');
  });

  it('keeps a tiny positive cost visibly distinct from zero', () => {
    const options = { maximumFractionDigits: 8 };
    expect(formatFiat(0.000000000001, 'USD', 'en-US', options)).toMatch(/^<USD\s+0\.00000001$/);
    expect(formatFiat(0, 'USD', 'en-US', options)).toMatch(/^USD\s+0$/);
    expect(formatFiat(0.00000001, 'USD', 'en-US', options)).toMatch(/^USD\s+0\.00000001$/);
  });

  it.each([null, undefined, -1, NaN, Infinity])(
    'marks an unavailable or invalid estimate %s',
    (amount) => {
      expect(formatFiat(amount, 'EUR', 'en-US')).toBe('-');
    }
  );

  it('supports a number-only value when the currency is given in the adjacent label', () => {
    expect(
      formatFiat(12_345.67, 'EUR', 'en-US', { style: 'decimal', maximumFractionDigits: 0 })
    ).toBe('12,346');
  });
});
