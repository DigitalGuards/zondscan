'use client';

import Link from 'next/link';
import { useQuery } from '@tanstack/react-query';
import { BoltIcon } from '@heroicons/react/24/outline';
import config from '../../config';
import { formatPlanckAdaptive } from '../lib/helpers';
import { useDisplayCurrency } from './useDisplayCurrency';

async function getSummary(path: string) {
  const response = await fetch(`${config.handlerUrl}${path}`);
  if (!response.ok) throw new Error('Summary unavailable');
  return response.json();
}

export default function MarketSummary() {
  const fiat = useDisplayCurrency();
  const overview = useQuery<{ currentPrice?: number; priceChange24h?: number }>({
    queryKey: ['header-overview'],
    queryFn: () => getSummary('/overview'),
    enabled: Boolean(config.handlerUrl),
    staleTime: 30_000,
    refetchInterval: 30_000,
    refetchIntervalInBackground: false,
    retry: 1,
  });
  const gas = useQuery<{ avgGasPriceHex?: string }>({
    queryKey: ['header-gas'],
    queryFn: () => getSummary('/gas/summary'),
    enabled: Boolean(config.handlerUrl),
    staleTime: 30_000,
    refetchInterval: 30_000,
    refetchIntervalInBackground: false,
    retry: 1,
  });
  const price = overview.data?.currentPrice;
  const change = overview.data?.priceChange24h;
  const validPrice =
    !overview.isError && typeof price === 'number' && Number.isFinite(price) && price > 0;
  const validChange = validPrice && typeof change === 'number' && Number.isFinite(change);
  const gasHex = gas.data?.avgGasPriceHex;
  const validGas = !gas.isError && typeof gasHex === 'string' && /^0x[0-9a-f]+$/i.test(gasHex);
  const [gasValue, gasUnit] = validGas ? formatPlanckAdaptive(gasHex) : ['-', ''];
  const shortGas = validGas
    ? new Intl.NumberFormat('en-US', { maximumFractionDigits: 3 }).format(Number(gasValue))
    : '-';

  return (
    <div
      lang="en"
      className="flex min-w-0 flex-wrap items-center gap-x-4 gap-y-1 text-[11px] sm:text-xs tabular-nums"
    >
      <Link
        href="/orderbook"
        className="whitespace-nowrap text-text-secondary hover:text-accent"
        title={
          validPrice
            ? `QRL market price in ${fiat.currency}. Testnet Quanta has no market value.${
                fiat.rateDate ? ` ECB reference rate: ${fiat.rateDate}.` : ''
              }${fiat.status === 'unavailable' ? ' Conversion unavailable; showing USD.' : ''}`
            : 'QRL market price is temporarily unavailable'
        }
      >
        QRL:{' '}
        <span className="font-medium text-accent" data-display-currency={fiat.currency}>
          {validPrice
            ? fiat.format(price, {
                minimumFractionDigits: 2,
                maximumFractionDigits: 4,
              })
            : '-'}
        </span>
        {validChange && (
          <span className={`ml-1 ${change >= 0 ? 'text-success' : 'text-error'}`}>
            ({change >= 0 ? '+' : ''}
            {change.toFixed(2)}%)
          </span>
        )}
        {validPrice && fiat.status === 'unavailable' && (
          <span className="ml-1 text-text-muted">(USD fallback)</span>
        )}
      </Link>
      <Link
        href="/gas"
        className="inline-flex items-center gap-1 whitespace-nowrap text-text-secondary hover:text-accent"
        title={
          validGas
            ? `Average recent transaction gas price on QRL Testnet v2: ${gasValue} ${gasUnit}`
            : 'Testnet gas price is temporarily unavailable'
        }
      >
        <BoltIcon className="size-3.5 text-text-muted" aria-hidden="true" />
        Gas:{' '}
        <span className="text-accent">
          {shortGas}
          {gasUnit ? ` ${gasUnit}` : ''}
        </span>
      </Link>
    </div>
  );
}
