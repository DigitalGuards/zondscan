'use client';

import { useQuery } from '@tanstack/react-query';
import { usePreferences } from './PreferencesProvider';
import { EXCHANGE_RATES_REFRESH_MS, formatFiat, parseExchangeRates } from '../lib/currency';

export type CurrencyStatus = 'usd' | 'ready' | 'loading' | 'unavailable';

async function fetchExchangeRates() {
  const response = await fetch('/api/exchange-rates', { signal: AbortSignal.timeout(8_000) });
  if (!response.ok) throw new Error('Currency conversion unavailable');
  const rates = parseExchangeRates(await response.json());
  if (!rates) throw new Error('Invalid exchange rates');
  return rates;
}

/** Every fiat surface shares this query, so choosing another currency needs no new request. */
export function useDisplayCurrency() {
  const { preferences } = usePreferences();
  const requestedCurrency = preferences.currency;
  const query = useQuery({
    queryKey: ['exchange-rates', 'USD'],
    queryFn: fetchExchangeRates,
    enabled: requestedCurrency !== 'USD',
    staleTime: EXCHANGE_RATES_REFRESH_MS,
    gcTime: 24 * EXCHANGE_RATES_REFRESH_MS,
    refetchInterval: EXCHANGE_RATES_REFRESH_MS,
    refetchIntervalInBackground: false,
    retry: false,
  });
  const rates = query.isError ? null : parseExchangeRates(query.data);
  const status: CurrencyStatus =
    requestedCurrency === 'USD'
      ? 'usd'
      : rates
        ? 'ready'
        : query.isPending
          ? 'loading'
          : 'unavailable';
  const currency = status === 'ready' ? requestedCurrency : 'USD';
  const rate = currency === 'USD' ? 1 : rates!.rates[currency];
  const locale = preferences.locale === 'en' ? 'en-US' : preferences.locale;
  const format = (usd: number | null | undefined, options?: Intl.NumberFormatOptions): string =>
    formatFiat(typeof usd === 'number' ? usd * rate : usd, currency, locale, options);

  return {
    requestedCurrency,
    currency,
    status,
    rateDate: status === 'ready' ? rates!.date : null,
    format,
  };
}
