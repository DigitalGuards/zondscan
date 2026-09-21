import {
  EXCHANGE_RATES_REFRESH_MS,
  parseFrankfurterRates,
  type ExchangeRates,
} from '../../lib/currency';

export const dynamic = 'force-dynamic';

// Fixed public source: callers cannot select another URL, base, or provider.
// https://frankfurter.dev/#rates documents the ECB provider and quote filters.
const RATES_URL =
  'https://api.frankfurter.dev/v2/rates?base=USD&quotes=EUR,GBP,CHF,CAD,AUD,JPY,CNY&providers=ECB';
const MAX_RESPONSE_BYTES = 16_384;
let cached: { rates: ExchangeRates; expires: number } | null = null;
let inflight: Promise<ExchangeRates> | null = null;
let retryAfter = 0;

async function readRates(response: Response): Promise<unknown> {
  if (!response.body) throw new Error('Empty exchange rate response');
  const reader = response.body.getReader();
  const chunks: Uint8Array[] = [];
  let length = 0;
  try {
    for (;;) {
      const { value, done } = await reader.read();
      if (done) break;
      length += value.byteLength;
      if (length > MAX_RESPONSE_BYTES) throw new Error('Exchange rate response too large');
      chunks.push(value);
    }
  } finally {
    await reader.cancel();
  }
  const bytes = new Uint8Array(length);
  let offset = 0;
  for (const chunk of chunks) {
    bytes.set(chunk, offset);
    offset += chunk.byteLength;
  }
  return JSON.parse(new TextDecoder().decode(bytes));
}

async function loadRates(): Promise<ExchangeRates> {
  const response = await fetch(RATES_URL, {
    signal: AbortSignal.timeout(5_000),
    cache: 'no-store',
    redirect: 'error',
    headers: { Accept: 'application/json' },
  });
  if (!response.ok) throw new Error('Exchange rate provider unavailable');
  const rates = parseFrankfurterRates(await readRates(response));
  if (!rates) throw new Error('Invalid exchange rate response');
  cached = { rates, expires: Date.now() + EXCHANGE_RATES_REFRESH_MS };
  return rates;
}

export async function GET() {
  try {
    let rates = cached && cached.expires > Date.now() ? cached.rates : null;
    if (!rates) {
      if (Date.now() < retryAfter) throw new Error('Exchange rate provider cooling down');
      inflight ??= loadRates()
        .catch((error: unknown) => {
          retryAfter = Date.now() + 60_000;
          throw error;
        })
        .finally(() => {
          inflight = null;
        });
      rates = await inflight;
    }
    return Response.json(rates, {
      headers: { 'Cache-Control': 'public, max-age=300, s-maxage=3600' },
    });
  } catch {
    return Response.json(
      { error: 'Currency conversion temporarily unavailable' },
      { status: 503, headers: { 'Cache-Control': 'no-store', 'Retry-After': '60' } }
    );
  }
}
