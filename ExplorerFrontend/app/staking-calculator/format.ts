/**
 * Display helpers for the staking calculator.
 *
 * `formatNumberWithCommas` in lib/helpers groups every run of three digits in
 * the string, including the fractional part, so 0.5573 comes out as "0.5,573".
 * Its existing callers pass whole numbers. These amounts are fractional, so
 * they go through Intl instead.
 */

/** Thousands-separated, with the fractional part left alone. */
export function num(value: number, digits = 0): string {
  if (!Number.isFinite(value)) return '0';
  return value.toLocaleString('en-US', {
    minimumFractionDigits: digits,
    maximumFractionDigits: digits,
  });
}

export function pct(value: number, digits = 2): string {
  if (!Number.isFinite(value)) return '0%';
  return `${(value * 100).toFixed(digits)}%`;
}

export function usd(value: number): string {
  return value.toLocaleString('en-US', {
    style: 'currency',
    currency: 'USD',
    maximumFractionDigits: value < 100 ? 2 : 0,
  });
}

/** Compact magnitude for tick labels and counters: 20.48M, 1.2K. */
export function compact(value: number): string {
  if (!Number.isFinite(value)) return '0';
  if (value >= 1e9) return `${(value / 1e9).toFixed(2)}B`;
  if (value >= 1e6) return `${(value / 1e6).toFixed(2)}M`;
  if (value >= 1e3) return `${(value / 1e3).toFixed(1)}K`;
  return value.toFixed(0);
}

/**
 * A duration given in years, rendered at whatever unit reads naturally.
 * Proposal intervals land in hours, self-funding a validator lands in years,
 * so the whole range has to work.
 */
export function humanDuration(years: number): string {
  if (!Number.isFinite(years) || years <= 0) return 'never';
  const hours = years * 365.25 * 24;
  if (hours < 1) return `${Math.round(hours * 60)} minutes`;
  if (hours < 48) return `${hours.toFixed(1)} hours`;
  const days = hours / 24;
  if (days < 90) return `${Math.round(days)} days`;
  if (years < 1) return `${Math.round(days / 30.44)} months`;
  return `${years.toFixed(1)} years`;
}

/** Seconds as "2h 8m", for the epoch-length parameter row. */
export function humanSeconds(seconds: number): string {
  const h = Math.floor(seconds / 3600);
  const m = Math.round((seconds % 3600) / 60);
  if (h === 0) return `${m}m`;
  return m === 0 ? `${h}h` : `${h}h ${m}m`;
}
