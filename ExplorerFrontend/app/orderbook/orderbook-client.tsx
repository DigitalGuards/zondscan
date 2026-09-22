'use client';

import { useEffect, useMemo, useState } from 'react';
import axios from 'axios';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowTopRightOnSquareIcon, PauseIcon, PlayIcon } from '@heroicons/react/24/outline';
import config from '../../config';
import FundFlowPanel from './fund-flow';
import OrderBookLoading from './loading';
import { palette } from '../lib/theme';
import TimeDisplay from '../components/TimeDisplay';
import {
  buildGroupedLadder,
  calculateBookStats,
  formatMarketPrice,
  formatQrlQuantity,
  formatQrlTradeSize,
  marketTradeKey,
  PRICE_GROUPINGS,
  type LadderLevel,
  type MarketOrderBookResponse,
  type PriceGrouping,
  type MarketSide,
  type MarketTrade,
} from '../lib/orderbook';

const ROW_OPTIONS = [12, 25, 50] as const;
const QUERY_KEY = ['market-orderbook', 'QRLUSDT'];
const STALE_AFTER_MS = 15_000;

const numeric = (value: string | number | null | undefined): number => {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : 0;
};

function formatSignedPercent(fraction: string): string {
  const value = Number(fraction) * 100;
  if (!Number.isFinite(value)) return 'Unavailable';
  return `${value > 0 ? '+' : ''}${value.toFixed(2)}%`;
}

function StatCard({
  label,
  value,
  detail,
}: {
  label: string;
  value: string;
  detail: string;
}): JSX.Element {
  return (
    <div className="card min-w-0 p-3 sm:p-4">
      <p className="eyebrow mb-1.5">{label}</p>
      <p className="break-words font-mono text-lg font-medium text-text-primary sm:text-xl">
        {value}
      </p>
      <p className="mt-1 text-xs text-text-muted">{detail}</p>
    </div>
  );
}

function StatusBadge({
  paused,
  isFetching,
  isError,
  stale,
}: {
  paused: boolean;
  isFetching: boolean;
  isError: boolean;
  stale: boolean;
}): JSX.Element {
  const label = paused
    ? 'Updates paused'
    : isError || stale
      ? 'Data delayed'
      : isFetching
        ? 'Updating'
        : 'Live';
  const tone = paused
    ? 'border-border bg-surface-2 text-text-secondary'
    : isError || stale
      ? 'border-warning/30 bg-warning/10 text-warning'
      : 'border-success/30 bg-success/10 text-success';
  return (
    <span
      className={`inline-flex items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-medium ${tone}`}
    >
      <span className="h-1.5 w-1.5 rounded-full bg-current" aria-hidden="true" />
      {label}
    </span>
  );
}

function DepthTable({
  levels,
  side,
  maxCumulative,
}: {
  levels: LadderLevel[];
  side: MarketSide;
  maxCumulative: number;
}): JSX.Element {
  const title = side === 'buy' ? 'Bids' : 'Asks';
  const tone = side === 'buy' ? 'text-success' : 'text-error';
  const color = `color-mix(in srgb, ${side === 'buy' ? palette.success : palette.error} 10%, transparent)`;
  return (
    <div className="min-w-0">
      <h4 className={`px-4 py-2 text-xs font-semibold ${tone}`}>
        {title}{' '}
        <span className="font-normal text-text-muted">({side === 'buy' ? 'Buy' : 'Sell'})</span>
      </h4>
      <div
        className="max-h-[26rem] overflow-auto px-4 pb-2"
        tabIndex={0}
        role="region"
        aria-label={`${title} price levels`}
      >
        <table className="w-full table-fixed text-xs" aria-label={`${title} QRL USDT order book`}>
          <thead className="sticky top-0 bg-surface">
            <tr className="text-[10px] text-text-muted sm:text-[11px]">
              <th scope="col" className="py-2 pr-1 text-left font-medium">
                Price (USDT)
              </th>
              <th scope="col" className="px-1 py-2 text-right font-medium">
                Amount (QRL)
              </th>
              <th scope="col" className="py-2 pl-1 text-right font-medium">
                Total (QRL)
              </th>
            </tr>
          </thead>
          <tbody>
            {levels.map((level) => {
              const width = `${Math.min(100, (level.cumulativeQuantity / maxCumulative) * 100)}%`;
              return (
                <tr
                  key={level.raw.price}
                  className="font-mono tabular-nums"
                  style={{
                    backgroundImage: `linear-gradient(to left, ${color} ${width}, transparent ${width})`,
                  }}
                >
                  <td className={`py-1.5 pr-1 ${tone}`}>{formatMarketPrice(level.price)}</td>
                  <td className="px-1 py-1.5 text-right text-text-primary">
                    {formatQrlQuantity(level.quantity)}
                  </td>
                  <td className="py-1.5 pl-1 text-right text-text-secondary">
                    {formatQrlQuantity(level.cumulativeQuantity)}
                  </td>
                </tr>
              );
            })}
            {levels.length === 0 && (
              <tr>
                <td colSpan={3} className="py-8 text-center text-text-muted">
                  No {title.toLowerCase()} in this snapshot.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function DepthLadder({
  bids,
  asks,
  grouping,
  onGroupingChange,
  rowsPerSide,
  onRowsChange,
  spread,
  spreadBps,
}: {
  bids: LadderLevel[];
  asks: LadderLevel[];
  grouping: PriceGrouping;
  onGroupingChange: (grouping: PriceGrouping) => void;
  rowsPerSide: number;
  onRowsChange: (rows: number) => void;
  spread: number;
  spreadBps: number;
}): JSX.Element {
  const maxCumulative = Math.max(
    bids[bids.length - 1]?.cumulativeQuantity ?? 0,
    asks[asks.length - 1]?.cumulativeQuantity ?? 0,
    1
  );
  return (
    <section className="card min-w-0 overflow-hidden" aria-labelledby="order-book-heading">
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-4 py-3">
        <h3
          id="order-book-heading"
          className="font-display text-sm font-semibold text-text-primary"
        >
          Order book
        </h3>
        <div className="flex flex-wrap items-center gap-3">
          <label className="flex items-center gap-2 text-xs text-text-muted">
            Group
            <select
              value={grouping}
              onChange={(event) => onGroupingChange(event.target.value as PriceGrouping)}
              className="min-h-9 rounded-md border border-border bg-surface-2 px-2 font-mono text-xs text-text-primary focus:border-accent"
              aria-label="Price grouping in USDT"
            >
              {PRICE_GROUPINGS.map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </select>
          </label>
          <label className="flex items-center gap-2 text-xs text-text-muted">
            Rows
            <select
              value={rowsPerSide}
              onChange={(event) => onRowsChange(Number(event.target.value))}
              className="min-h-9 rounded-md border border-border bg-surface-2 px-2 font-mono text-xs text-text-primary focus:border-accent"
              aria-label="Order book rows per side"
            >
              {ROW_OPTIONS.map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </select>
          </label>
        </div>
      </div>
      <div className="flex flex-wrap justify-between gap-x-4 gap-y-1 border-b border-border px-4 py-2 text-xs text-text-muted">
        <span>
          Spread{' '}
          <span className="font-mono text-text-primary">
            {spread === 0 ? '0.00' : formatMarketPrice(spread)} USDT
          </span>
        </span>
        <span className="font-mono">{spreadBps.toFixed(1)} bps</span>
      </div>
      <div className="grid divide-y divide-border sm:grid-cols-2 sm:divide-x sm:divide-y-0">
        <DepthTable levels={bids} side="buy" maxCumulative={maxCumulative} />
        <DepthTable levels={asks} side="sell" maxCumulative={maxCumulative} />
      </div>
      <p className="border-t border-border px-4 py-2 text-[11px] text-text-muted">
        Total is cumulative QRL from the best price. Depth bars share the same scale.
      </p>
    </section>
  );
}

function RecentTrades({ trades }: { trades: MarketTrade[] }): JSX.Element {
  const ordered = [...trades].sort((a, b) => b.time - a.time).slice(0, 20);
  return (
    <section className="card min-w-0 overflow-hidden" aria-labelledby="recent-trades-heading">
      <div className="border-b border-border px-4 py-3">
        <h3
          id="recent-trades-heading"
          className="font-display text-sm font-semibold text-text-primary"
        >
          Recent trades
        </h3>
        <p className="mt-1 text-xs text-text-muted">Buy and sell indicate the taker side.</p>
      </div>
      <div
        className="max-h-[30rem] overflow-auto px-4 py-2"
        tabIndex={0}
        role="region"
        aria-label="Recent trades list"
      >
        <table className="w-full text-xs" aria-label="Recent QRL USDT trades">
          <thead className="sticky top-0 bg-surface">
            <tr className="text-[10px] text-text-muted sm:text-[11px]">
              <th scope="col" className="py-2 text-left font-medium">
                Time
              </th>
              <th scope="col" className="px-2 py-2 text-left font-medium">
                Side
              </th>
              <th scope="col" className="py-2 text-right font-medium">
                Price (USDT)
              </th>
              <th scope="col" className="py-2 pl-2 text-right font-medium">
                Amount (QRL)
              </th>
            </tr>
          </thead>
          <tbody>
            {ordered.map((trade) => (
              <tr
                key={marketTradeKey(trade)}
                className="border-t border-border/50 font-mono tabular-nums"
              >
                <td className="whitespace-nowrap py-1.5 text-text-muted">
                  <TimeDisplay timestamp={trade.time / 1000} clockOnly />
                </td>
                <td
                  className={`px-2 py-1.5 ${trade.aggressorSide === 'buy' ? 'text-success' : 'text-error'}`}
                >
                  {trade.aggressorSide === 'buy' ? 'Buy' : 'Sell'}
                </td>
                <td className="py-1.5 text-right text-text-primary">
                  {formatMarketPrice(numeric(trade.price))}
                </td>
                <td className="py-1.5 pl-2 text-right text-text-secondary">
                  {formatQrlTradeSize(numeric(trade.quantity))}
                </td>
              </tr>
            ))}
            {ordered.length === 0 && (
              <tr>
                <td colSpan={4} className="py-8 text-center text-text-muted">
                  No recent trades available.
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    </section>
  );
}

export default function OrderBookClient(): JSX.Element {
  const [paused, setPaused] = useState(false);
  const [priceGrouping, setPriceGrouping] = useState<PriceGrouping>('0.001');
  const [rowsPerSide, setRowsPerSide] = useState<number>(12);
  const [now, setNow] = useState(0);
  const queryClient = useQueryClient();

  useEffect(() => {
    const timer = window.setInterval(() => setNow(Date.now()), 5_000);
    return () => window.clearInterval(timer);
  }, []);

  const orderBookQuery = useQuery<MarketOrderBookResponse>({
    queryKey: QUERY_KEY,
    queryFn: async ({ signal }) => {
      const response = await axios.get<MarketOrderBookResponse>(
        `${config.handlerUrl}/market/orderbook`,
        { timeout: 8_000, signal }
      );
      return response.data;
    },
    enabled: !paused,
    refetchInterval: paused ? false : 3_000,
    refetchIntervalInBackground: false,
    refetchOnWindowFocus: !paused,
    staleTime: 1_500,
    retry: 2,
  });

  const data = orderBookQuery.data;
  const stats = useMemo(() => (data ? calculateBookStats(data) : null), [data]);
  const bids = useMemo(
    () => (data ? buildGroupedLadder(data.bids, 'buy', priceGrouping, rowsPerSide) : []),
    [data, priceGrouping, rowsPerSide]
  );
  const asks = useMemo(
    () => (data ? buildGroupedLadder(data.asks, 'sell', priceGrouping, rowsPerSide) : []),
    [data, priceGrouping, rowsPerSide]
  );

  if (!data || !stats) {
    if (orderBookQuery.isError) {
      return (
        <div className="page-content py-10">
          <div className="card mx-auto max-w-xl p-6 text-center" role="alert">
            <h2 className="font-display text-xl font-semibold text-text-primary">
              Market feed unavailable
            </h2>
            <p className="mt-2 text-sm text-text-secondary">
              The MEXC QRL/USDT market data could not be loaded.
            </p>
            <button
              type="button"
              onClick={() => void orderBookQuery.refetch()}
              className="mt-5 min-h-11 rounded-lg bg-accent px-4 py-2 text-sm font-semibold text-background hover:bg-accent-hover"
            >
              Retry
            </button>
          </div>
        </div>
      );
    }
    return <OrderBookLoading />;
  }

  const snapshotTime = Date.parse(data.fetchedAt);
  const stale =
    !Number.isFinite(snapshotTime) ||
    Math.max(now, orderBookQuery.dataUpdatedAt) - snapshotTime > STALE_AFTER_MS;
  const depthTotal = stats.bidQuantityInBand + stats.askQuantityInBand;
  const bidShare = depthTotal > 0 ? (stats.bidQuantityInBand / depthTotal) * 100 : 50;
  const toggleUpdates = (): void => {
    if (!paused) void queryClient.cancelQueries({ queryKey: QUERY_KEY });
    setPaused((value) => !value);
  };

  return (
    <div className="page-content py-4 sm:py-6 lg:py-8">
      <header className="mb-5 flex flex-col gap-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <div className="flex flex-wrap items-center gap-3">
            <h2 className="section-title">QRL / USDT</h2>
            <StatusBadge
              paused={paused}
              isFetching={orderBookQuery.isFetching}
              isError={orderBookQuery.isError}
              stale={stale}
            />
          </div>
          <p className="mt-2 text-sm text-text-secondary">
            Spot order book, recent trades and fund flow
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-3 text-xs">
          <a
            href="https://www.mexc.com/exchange/QRL_USDT"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex min-h-11 items-center gap-1.5 text-accent hover:text-accent-hover"
          >
            MEXC Spot <ArrowTopRightOnSquareIcon className="h-3.5 w-3.5" aria-hidden="true" />
          </a>
          <button
            type="button"
            onClick={toggleUpdates}
            aria-pressed={paused}
            className="inline-flex min-h-11 items-center gap-2 rounded-lg border border-border bg-surface px-3 py-2 font-medium text-text-secondary hover:text-text-primary"
          >
            {paused ? (
              <PlayIcon className="h-4 w-4" aria-hidden="true" />
            ) : (
              <PauseIcon className="h-4 w-4" aria-hidden="true" />
            )}
            {paused ? 'Resume order book' : 'Pause order book'}
          </button>
        </div>
      </header>

      {(orderBookQuery.isError || stale) && !paused && (
        <div
          className="mb-4 flex flex-wrap items-center justify-between gap-3 rounded-xl border border-warning/30 bg-warning/10 px-4 py-3 text-sm text-warning"
          role="status"
        >
          <span>Market data is delayed. Showing the last available snapshot.</span>
          <button
            type="button"
            onClick={() => void orderBookQuery.refetch()}
            disabled={orderBookQuery.isFetching}
            className="min-h-9 rounded border border-warning/40 px-3 py-1 font-medium disabled:opacity-50"
          >
            Retry
          </button>
        </div>
      )}

      <section className="mb-5 grid grid-cols-2 gap-3 lg:grid-cols-4" aria-label="Market summary">
        <StatCard
          label="Last price"
          value={`${formatMarketPrice(stats.last)} USDT`}
          detail={`${formatSignedPercent(data.ticker.changePercent)} in 24h`}
        />
        <StatCard label="24h high" value={formatMarketPrice(stats.high24h)} detail="USDT" />
        <StatCard label="24h low" value={formatMarketPrice(stats.low24h)} detail="USDT" />
        <StatCard
          label="24h volume"
          value={`${formatQrlQuantity(numeric(data.ticker.baseVolume))} QRL`}
          detail={
            data.ticker.quoteVolume === null
              ? 'Quote volume unavailable'
              : `${formatQrlQuantity(numeric(data.ticker.quoteVolume))} USDT`
          }
        />
      </section>

      <div className="mb-3 flex flex-wrap justify-between gap-x-4 gap-y-1 text-xs text-text-muted">
        <span>
          Best bid{' '}
          <span className="font-mono text-success">{formatMarketPrice(stats.bestBid)}</span>{' '}
          <span className="mx-2">/</span> Best ask{' '}
          <span className="font-mono text-error">{formatMarketPrice(stats.bestAsk)}</span> USDT
        </span>
        <span>
          Snapshot{' '}
          {Number.isFinite(snapshotTime) ? (
            <TimeDisplay timestamp={snapshotTime / 1000} clockOnly />
          ) : (
            'unavailable'
          )}
          {paused ? ' · paused' : ' · refreshes every 3s'}
        </span>
      </div>
      <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]">
        <DepthLadder
          bids={bids}
          asks={asks}
          grouping={priceGrouping}
          onGroupingChange={setPriceGrouping}
          rowsPerSide={rowsPerSide}
          onRowsChange={setRowsPerSide}
          spread={stats.spread}
          spreadBps={stats.spreadBps}
        />
        <RecentTrades trades={data.recentTrades} />
      </div>

      <section className="card mt-4 px-4 py-3" aria-label="Visible order book liquidity">
        <div className="flex flex-wrap justify-between gap-2 text-xs">
          <p className="text-text-muted">Visible depth within {stats.bandPercent}% of midpoint</p>
          <p>
            <span className="text-success">
              Bids {formatQrlQuantity(stats.bidQuantityInBand)} QRL
            </span>
            <span className="mx-3 text-text-muted">/</span>
            <span className="text-error">
              Asks {formatQrlQuantity(stats.askQuantityInBand)} QRL
            </span>
          </p>
        </div>
        {depthTotal > 0 && (
          <div
            className="mt-2 flex h-1.5 overflow-hidden rounded-full bg-error/70"
            role="img"
            aria-label={`${bidShare.toFixed(1)}% bid depth and ${(100 - bidShare).toFixed(1)}% ask depth within ${stats.bandPercent}% of midpoint`}
          >
            <div className="h-full bg-success" style={{ width: `${bidShare}%` }} />
          </div>
        )}
      </section>

      <div className="mt-6">
        <FundFlowPanel />
      </div>
      <aside className="mt-6 text-xs leading-relaxed text-text-muted">
        Market data covers the MEXC QRL/USDT spot market. Displayed orders can change or be
        cancelled before execution. Fund-flow analysis measures collected trades at this venue; it
        does not measure exchange deposits, withdrawals or network activity.
      </aside>
    </div>
  );
}
