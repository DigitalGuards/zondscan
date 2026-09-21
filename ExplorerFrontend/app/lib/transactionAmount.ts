const PLANCK_PER_QUANTA = BigInt('1000000000000000000');
const DISPLAY_THRESHOLD = BigInt('1000000000000');
const MAX_PLANCK = BigInt(2) ** BigInt(256) - BigInt(1);

export interface TransactionAmountDisplay {
  compact: string;
  quanta: string;
  planck: string;
}

/** API decimals are Quanta; RPC hex quantities are Planck. Preserve every supplied digit. */
function parsePlanck(amount: string | number | null | undefined): bigint | null {
  if (amount === null || amount === undefined) return null;
  const input = String(amount).trim();
  if (!input || input.length > 128) return null;

  if (/^0x[0-9a-f]+$/i.test(input)) return BigInt(input);

  // Scientific notation also occurs in legacy numeric API responses.
  const match = /^(\d+)(?:\.(\d+))?(?:e([+-]?\d+))?$/i.exec(input);
  if (!match) return null;
  const [, whole, fraction = '', exponent = '0'] = match;
  const shift = 18 + Number(exponent) - fraction.length;
  if (!Number.isInteger(shift) || Math.abs(shift) > 128) return null;
  const digits = BigInt(whole + fraction);
  if (shift >= 0) return digits * BigInt(10) ** BigInt(shift);

  const divisor = BigInt(10) ** BigInt(-shift);
  return digits % divisor === BigInt(0) ? digits / divisor : null;
}

/** Compact list display plus exact, copyable values, using integer and string arithmetic. */
export function formatTransactionAmount(
  amount: string | number | null | undefined
): TransactionAmountDisplay | null {
  const planck = parsePlanck(amount);
  if (planck === null || planck > MAX_PLANCK) return null;

  const whole = (planck / PLANCK_PER_QUANTA).toString();
  const fraction = (planck % PLANCK_PER_QUANTA).toString().padStart(18, '0').replace(/0+$/, '');
  const quanta = fraction ? `${whole}.${fraction}` : whole;
  const compactFraction = fraction.slice(0, 6).replace(/0+$/, '');
  const groupedWhole = whole.replace(/\B(?=(\d{3})+(?!\d))/g, ',');
  const compact =
    planck > BigInt(0) && planck < DISPLAY_THRESHOLD
      ? '<0.000001'
      : groupedWhole + (compactFraction ? `.${compactFraction}` : '');

  return { compact, quanta, planck: planck.toString() };
}
