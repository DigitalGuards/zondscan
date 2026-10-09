import type { CurrencyPreference } from './preferences';
import { isArray, isOneOf, isRecord } from './guards';

export const CURRENCIES: readonly { code: CurrencyPreference; name: string }[] = [
  { code: 'USD', name: 'United States Dollar' },
  { code: 'EUR', name: 'Euro' },
  { code: 'GBP', name: 'British Pound' },
  { code: 'CHF', name: 'Swiss Franc' },
  { code: 'CAD', name: 'Canadian Dollar' },
  { code: 'AUD', name: 'Australian Dollar' },
  { code: 'JPY', name: 'Japanese Yen' },
  { code: 'CNY', name: 'Chinese Yuan' },
];

export interface ExchangeRates {
  base: 'USD';
  date: string;
  rates: Record<CurrencyPreference, number>;
}

export const EXCHANGE_RATES_MAX_AGE_MS = 10 * 24 * 60 * 60 * 1000;
export const EXCHANGE_RATES_REFRESH_MS = 60 * 60 * 1000;

function isValidDate(value: unknown, now: number): value is string {
  if (typeof value !== 'string' || !/^\d{4}-\d{2}-\d{2}$/.test(value)) return false;
  const timestamp = Date.parse(`${value}T00:00:00Z`);
  return (
    Number.isFinite(timestamp) &&
    new Date(timestamp).toISOString().slice(0, 10) === value &&
    timestamp <= now &&
    now - timestamp <= EXCHANGE_RATES_MAX_AGE_MS
  );
}

function isValidRate(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value > 0 && value < 1_000_000;
}

/** Validate both the source and the age before converting any USD amount. */
export function parseExchangeRates(value: unknown, now = Date.now()): ExchangeRates | null {
  if (!isRecord(value)) return null;
  const data = value;
  if (data.base !== 'USD' || !isValidDate(data.date, now)) return null;
  if (!isRecord(data.rates)) return null;
  if (data.rates.USD !== 1) return null;
  const { USD, EUR, GBP, CHF, CAD, AUD, JPY, CNY } = data.rates;
  if (
    !isValidRate(EUR) ||
    !isValidRate(GBP) ||
    !isValidRate(CHF) ||
    !isValidRate(CAD) ||
    !isValidRate(AUD) ||
    !isValidRate(JPY) ||
    !isValidRate(CNY)
  )
    return null;
  const rates = { USD, EUR, GBP, CHF, CAD, AUD, JPY, CNY };
  return { base: 'USD', date: data.date, rates };
}

/** Frankfurter v2 is requested with providers=ECB and one shared USD base. */
export function parseFrankfurterRates(value: unknown, now = Date.now()): ExchangeRates | null {
  if (!isArray(value) || value.length !== CURRENCIES.length - 1) return null;
  const rates: Partial<Record<CurrencyPreference, number>> = { USD: 1 };
  let date: string | undefined;
  for (const entry of value) {
    if (!isRecord(entry)) return null;
    const { base, quote, rate } = entry;
    if (base !== 'USD' || !isOneOf(quote, ['EUR', 'GBP', 'CHF', 'CAD', 'AUD', 'JPY', 'CNY'])) {
      return null;
    }
    if (!isValidDate(entry.date, now) || !isValidRate(rate)) return null;
    if (date && date !== entry.date) return null;
    if (rates[quote] !== undefined) return null;
    date = entry.date;
    rates[quote] = rate;
  }
  return parseExchangeRates({ base: 'USD', date, rates }, now);
}

export function formatFiat(
  amount: number | null | undefined,
  currency: CurrencyPreference,
  locale: string,
  options: Intl.NumberFormatOptions = {}
): string {
  if (typeof amount !== 'number' || !Number.isFinite(amount) || amount < 0) return '-';
  const formatter = new Intl.NumberFormat(locale, {
    style: 'currency',
    currency,
    currencyDisplay: 'code',
    minimumFractionDigits: 0,
    maximumFractionDigits: 2,
    ...options,
  });
  const precision = formatter.resolvedOptions().maximumFractionDigits;
  const threshold = 10 ** -(precision ?? 2);
  // Preserve the distinction between a tiny positive estimate and zero.
  if (amount > 0 && amount < threshold) return `<${formatter.format(threshold)}`;
  return formatter.format(amount);
}
