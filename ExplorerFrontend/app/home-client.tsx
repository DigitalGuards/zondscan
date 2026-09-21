'use client';

import TimeDisplay from './components/TimeDisplay';
import AddressText from './components/AddressText';

import * as React from 'react';
import dynamic from 'next/dynamic';
import Link from 'next/link';
import axios from 'axios';
import { useQuery } from '@tanstack/react-query';
import { formatNumberWithCommas, formatStaked, formatGasPrice, truncateHash, formatAddress, NATIVE_UNIT } from './lib/helpers';
import { initialHomeData, loadHomeData, type BlockResult, type HomeData, type HomeStatus, type TxResult } from './lib/homeData';
import config from '../config.js';
import SearchBar from './components/SearchBar';
import TransactionAmount from './components/TransactionAmount';
import { useDisplayCurrency } from './components/useDisplayCurrency';

const Charts = dynamic(() => import('./components/Charts'), {
  loading: () => (
    // Mirrors the rendered Charts card (header strip + 400px body) so the
    // page doesn't reflow when the widget hydrates.
    <div className="card overflow-hidden">
      <div className="panel-header">
        <div className="skeleton h-4 w-32" />
      </div>
      <div className="p-4">
        <div className="h-[400px] mt-2 skeleton flex items-center justify-center">
          <span className="text-text-muted text-sm">Loading chart...</span>
        </div>
      </div>
    </div>
  ),
  ssr: false,
});

// ── Helpers ──────────────────────────────────────────────────────────────────

const SLOTS_PER_EPOCH = 128;

function parseHex(hex: string): number {
  if (!hex) return 0;
  if (hex.startsWith('0x')) return parseInt(hex, 16);
  return parseInt(hex, 10) || 0;
}

function parseTimestamp(ts: string | number | undefined): number {
  if (ts === undefined || ts === null) return 0;
  if (typeof ts === 'number') return ts;
  if (ts.startsWith('0x')) return parseInt(ts, 16);
  return parseInt(ts, 10) || 0;
}

// ── Icons ────────────────────────────────────────────────────────────────────

const icons = {
  epoch: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-3.5 w-3.5" aria-hidden="true" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  ),
  slot: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-3.5 w-3.5" aria-hidden="true" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
    </svg>
  ),
  gas: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-3.5 w-3.5" aria-hidden="true" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
      <path strokeLinecap="round" strokeLinejoin="round" d="M3 21h18" />
    </svg>
  ),
  block: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-3.5 w-3.5" aria-hidden="true" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
    </svg>
  ),
  validators: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-3.5 w-3.5" aria-hidden="true" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
    </svg>
  ),
  transactions: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-3.5 w-3.5" aria-hidden="true" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
    </svg>
  ),
  staked: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-3.5 w-3.5" aria-hidden="true" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  ),
  marketCap: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-3.5 w-3.5" aria-hidden="true" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M13 7h8m0 0v8m0-8l-8 8-4-4-6 6" />
    </svg>
  ),
  circulating: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-3.5 w-3.5" aria-hidden="true" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M16.023 9.348h4.992v-.001M2.985 19.644v-4.992m0 0h4.992m-4.993 0l3.181 3.183a8.25 8.25 0 0013.803-3.7M4.031 9.865a8.25 8.25 0 0113.803-3.7l3.181 3.182m0-4.991v4.99" />
    </svg>
  ),
};

// ── Stat Cluster ─────────────────────────────────────────────────────────────

interface Stat {
  label: string;
  value: string | number | null;
  status: HomeStatus;
  icon: React.ReactNode;
  live?: boolean;
}

function StatBar({ data }: { data: HomeData }) {
  const fiat = useDisplayCurrency();
  const stats: Stat[] = [
    { label: 'Epoch', value: data.epochInfo?.headEpoch ?? null, status: data.status.epochInfo, icon: icons.epoch },
    { label: 'Avg Gas Price', value: data.avgGasPriceHex ? `${formatGasPrice(data.avgGasPriceHex)} Shor` : null, status: data.status.avgGasPriceHex, icon: icons.gas },
    { label: 'Block Height', value: data.blockHeight === null ? null : formatNumberWithCommas(data.blockHeight.toString()), status: data.status.blockHeight, icon: icons.block, live: true },
    { label: 'Validators', value: data.validatorCount === null ? null : formatNumberWithCommas(data.validatorCount.toString()), status: data.status.overview, icon: icons.validators },
    { label: `Staked ${NATIVE_UNIT}`, value: data.totalStaked === null ? null : formatStaked(data.totalStaked), status: data.status.totalStaked, icon: icons.staked },
    { label: 'Transactions', value: data.totalTransactions === null ? null : formatNumberWithCommas(data.totalTransactions.toString()), status: data.status.totalTransactions, icon: icons.transactions },
    { label: `Market Cap (${fiat.currency})`, value: data.marketCap !== null && data.marketCap > 0 ? fiat.format(data.marketCap, { style: 'decimal', maximumFractionDigits: 0 }) : null, status: data.status.overview, icon: icons.marketCap },
    // Unit lives on the label line per the Quanta layout convention; the
    // value stays a bare number so the 8-cell strip keeps its width budget.
    { label: `Circulating ${NATIVE_UNIT}`, value: data.circulating === null ? null : formatNumberWithCommas(data.circulating), status: data.status.overview, icon: icons.circulating },
  ];

  return (
    // gap-px over a hairline backdrop draws uniform dividers at every
    // breakpoint (the old per-index border logic left uneven seams at the
    // sm 4-column layout).
    <div className="rounded-2xl border border-border overflow-hidden bg-border/60 shadow-card">
      <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-8 gap-px">
        {stats.map((stat) => (
          <div
            key={stat.label}
            className="bg-background-secondary px-4 py-4 sm:py-5 flex flex-col items-center justify-center text-center"
          >
            {stat.status === 'loading' ? (
              <div className="skeleton h-7 w-20" aria-label={`Loading ${stat.label}`} />
            ) : (
              <span title={stat.status === 'error' && stat.value !== null ? 'Last available value; refresh failed' : undefined} className="font-display text-lg sm:text-xl font-semibold text-text-primary tabular-nums inline-flex items-center gap-2">
                {stat.live && stat.value !== null && stat.status === 'ready' && <span className="live-dot" aria-hidden="true" />}
                {stat.value ?? <span className="text-sm text-text-muted">Unavailable</span>}
              </span>
            )}
            <span className="flex items-center gap-1.5 text-[11px] text-text-muted mt-1.5 uppercase tracking-wider">
              <span aria-hidden="true">{stat.icon}</span>
              {stat.label}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

// ── Tables (etherscan-inspired card rows) ────────────────────────────────────

const TABLE_ROWS = 8;

const ROW_CLASS = 'flex items-center gap-3 px-4 py-3 border-b border-border last:border-b-0 text-sm';

function TableHeader({ icon, title, viewAllHref, tone }: { icon: React.ReactNode; title: string; viewAllHref: string; tone: 'accent' | 'quantum' }) {
  return (
    <div className="panel-header">
      <h2 className="flex items-center gap-2.5 text-[15px] font-display font-semibold text-text-primary">
        <span className={`inline-flex items-center justify-center w-7 h-7 rounded-lg ${tone === 'accent' ? 'bg-accent/10 text-accent' : 'bg-quantum/10 text-quantum'}`}>
          {icon}
        </span>
        {title}
      </h2>
      <Link href={viewAllHref} className="text-xs link-accent hover:underline">
        View all &rarr;
      </Link>
    </div>
  );
}

function RowIcon({ children, tone }: { children: React.ReactNode; tone: 'accent' | 'quantum' }) {
  return (
    <div className={`flex-shrink-0 w-8 h-8 rounded-lg border flex items-center justify-center
                     ${tone === 'accent'
                       ? 'bg-accent/[0.07] border-accent/10 text-accent/80'
                       : 'bg-quantum/[0.07] border-quantum/10 text-quantum/80'}`}>
      {children}
    </div>
  );
}

function ValueBadge({ children }: { children: React.ReactNode }) {
  return (
    <span className="chip num text-[11px]">
      {children}
    </span>
  );
}

function SkeletonRow() {
  return (
    <div className={ROW_CLASS}>
      <div className="flex-shrink-0 w-8 h-8 rounded-lg skeleton" />
      <div className="flex-1 min-w-0 space-y-1.5">
        <div className="skeleton h-3.5 w-24" />
        <div className="skeleton h-3 w-16" />
      </div>
      <div className="flex-1 min-w-0 space-y-1.5 hidden sm:block">
        <div className="skeleton h-3 w-28" />
        <div className="skeleton h-3 w-28" />
      </div>
      <div className="skeleton h-5 w-16" />
    </div>
  );
}

function getEpochFromBlock(blockNumber: number): number {
  return Math.floor(blockNumber / SLOTS_PER_EPOCH);
}

// ── Block Table ──────────────────────────────────────────────────────────────

function BlockTable({ blocks, status }: { blocks: BlockResult[] | null; status: HomeStatus }) {
  return (
    <section aria-label="Latest blocks" className="card overflow-hidden">
      <TableHeader icon={icons.block} title="Latest Blocks" viewAllHref="/blocks/1" tone="accent" />
      <div>
        {status === 'loading'
          ? Array.from({ length: TABLE_ROWS }).map((_, i) => <SkeletonRow key={i} />)
          : blocks === null ? <p className="p-4 text-sm text-text-muted">Latest blocks unavailable. Retrying automatically.</p>
          : blocks.length === 0 ? <p className="p-4 text-sm text-text-muted">No blocks yet.</p>
          : blocks.slice(0, TABLE_ROWS).map((block, idx) => {
              const blockNum = parseHex(block.number);
              const epoch = getEpochFromBlock(blockNum);
              const timestamp = parseHex(block.timestamp);
              const txCount = block.transactions?.length || 0;
              const miner = block.miner ? formatAddress(block.miner) : '';

              return (
                <div key={`${block.number}-${idx}`} className={`${ROW_CLASS} hover:bg-surface transition-colors`}>
                  <RowIcon tone="accent">{icons.block}</RowIcon>

                  <div className="flex-1 min-w-0">
                    <Link
                      href={`/block/${blockNum}`}
                      className="block link-accent hover:underline font-medium num truncate"
                    >
                      {formatNumberWithCommas(blockNum.toString())}
                    </Link>
                    <span className="text-[11px] text-text-muted tabular-nums"><TimeDisplay timestamp={timestamp} relative /></span>
                  </div>

                  <div className="flex-1 min-w-0 hidden sm:block">
                    {miner ? (
                      <div className="flex items-start gap-1 text-[12px] min-w-0">
                        <span className="text-text-muted">Miner</span>
                        <Link
                          href={`/address/${miner}`}
                          className="text-text-secondary hover:text-accent hover:underline font-mono min-w-0 max-w-full"
                        >
                          <AddressText address={miner} />
                        </Link>
                      </div>
                    ) : null}
                    <div className="text-[11px] text-text-muted tabular-nums">
                      <Link href={`/block/${blockNum}`} className="hover:text-accent hover:underline">
                        {txCount} txn{txCount === 1 ? '' : 's'}
                      </Link>
                      <span className="text-text-muted/70"> · epoch {formatNumberWithCommas(epoch.toString())}</span>
                    </div>
                  </div>

                  <div className="flex-shrink-0">
                    <ValueBadge>{txCount} txn{txCount === 1 ? '' : 's'}</ValueBadge>
                  </div>
                </div>
              );
            })}
      </div>
    </section>
  );
}

// ── Transaction Table ────────────────────────────────────────────────────────

function TransactionTable({ txs, status }: { txs: TxResult[] | null; status: HomeStatus }) {
  return (
    <section aria-label="Latest transactions" className="card overflow-hidden">
      <TableHeader icon={icons.transactions} title="Latest Transactions" viewAllHref="/transactions/1" tone="quantum" />
      <div>
        {status === 'loading'
          ? Array.from({ length: TABLE_ROWS }).map((_, i) => <SkeletonRow key={i} />)
          : txs === null ? <p className="p-4 text-sm text-text-muted">Latest transactions unavailable. Retrying automatically.</p>
          : txs.length === 0 ? <p className="p-4 text-sm text-text-muted">No transactions yet.</p>
          : txs.slice(0, TABLE_ROWS).map((tx, idx) => {
              const timestamp = parseTimestamp(tx.TimeStamp);
              const from = tx.From ? formatAddress(tx.From) : '';
              const to = tx.To ? formatAddress(tx.To) : '';

              return (
                <div key={`${tx.TxHash}-${idx}`} className={`${ROW_CLASS} hover:bg-surface transition-colors`}>
                  <RowIcon tone="quantum">{icons.transactions}</RowIcon>

                  <div className="flex-1 min-w-0">
                    <Link
                      href={`/tx/${tx.TxHash}`}
                      className="block link-accent hover:underline font-mono text-[13px] truncate"
                    >
                      {truncateHash(tx.TxHash, 10, 6)}
                    </Link>
                    <span className="text-[11px] text-text-muted tabular-nums"><TimeDisplay timestamp={timestamp} relative /></span>
                  </div>

                  <div className="flex-1 min-w-0 hidden sm:block">
                    <div className="flex items-start gap-1 text-[12px] min-w-0">
                      <span className="text-text-muted w-8 flex-shrink-0">From</span>
                      {from ? (
                        <Link href={`/address/${from}`} className="text-text-secondary hover:text-accent hover:underline font-mono min-w-0 max-w-full">
                          <AddressText address={from} />
                        </Link>
                      ) : <span className="text-text-muted">…</span>}
                    </div>
                    <div className="flex items-start gap-1 text-[12px] min-w-0">
                      <span className="text-text-muted w-8 flex-shrink-0">To</span>
                      {to ? (
                        <Link href={`/address/${to}`} className="text-text-secondary hover:text-accent hover:underline font-mono min-w-0 max-w-full">
                          <AddressText address={to} />
                        </Link>
                      ) : <span className="text-text-muted">…</span>}
                    </div>
                  </div>

                  <div className="flex-shrink-0">
                    <ValueBadge>
                      <TransactionAmount amount={tx.Amount} />
                    </ValueBadge>
                  </div>
                </div>
              );
            })}
      </div>
    </section>
  );
}

// ── Main Component ───────────────────────────────────────────────────────────

export default function HomeClient(): JSX.Element {
  const [data, setData] = React.useState<HomeData>(initialHomeData);
  const generation = React.useRef(0);

  // Drive the polling via TanStack Query so backgrounded tabs go quiet
  // (every other live page uses this discipline; the previous raw
  // setInterval polled regardless of visibility). The loader writes each
  // response into local state; this query owns the snapshot schedule.
  // Return a timestamp so successive polls see a fresh value.
  useQuery<number>({
    queryKey: ['home-poll'],
    queryFn: async ({ signal }) => {
      const current = ++generation.current;
      await loadHomeData(
        async (path, requestSignal) => (await axios.get(config.handlerUrl + path, { signal: requestSignal, timeout: 15000 })).data,
        signal,
        update => {
          if (!signal.aborted && current === generation.current) setData(update);
        },
      );
      return Date.now();
    },
    refetchInterval: 30000,
    refetchIntervalInBackground: false,
    refetchOnMount: 'always',
    staleTime: 0,
  });

  return (
    <main className="page-content pt-6 lg:pt-10" aria-labelledby="home-heading">
      <div className="max-w-6xl mx-auto">
        {/* Entrance stagger runs once on mount; the 30s polling below only
            swaps row contents inside stable sections, so it never
            re-triggers the animation. */}
        <div className="stagger-children">
          <div className="text-center mb-8 lg:mb-10">
            <p className="eyebrow mb-3">ZondScan · QRL 2.0 · Zond Network</p>
            <h1
              id="home-heading"
              className="font-display text-3xl sm:text-4xl lg:text-5xl font-semibold tracking-tight text-text-primary"
            >
              Explore the <span className="text-gradient-accent">post-quantum</span> chain
            </h1>
          </div>

          {/* Search Bar */}
          <div className="mb-8 max-w-3xl mx-auto">
            <SearchBar />
          </div>

          {/* Init Warning */}
          {data.status.overview === 'ready' && data.dataInitialized === false && (
            <div
              role="status"
              className="mb-4 px-3 py-2.5 rounded-xl bg-warning/10 border border-warning/25 text-warning text-xs sm:text-sm flex items-center gap-2"
            >
              <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              Initializing explorer data... This may take a few minutes.
            </div>
          )}

          {Object.values(data.status).includes('error') && (
            <p role="status" className="mb-4 text-xs sm:text-sm text-text-muted">
              Some explorer data could not be refreshed. Showing available data and retrying automatically.
            </p>
          )}

          {/* Stats Cluster */}
          <section aria-label="Network stats" className="mb-6">
            <StatBar data={data} />
          </section>

          {/* Side-by-Side Tables */}
          <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 lg:gap-5 mb-6">
            <BlockTable blocks={data.blocks} status={data.status.blocks} />
            <TransactionTable txs={data.txs} status={data.status.txs} />
          </div>

          {/* TradingView Chart */}
          <section aria-label="Price chart" className="mb-2">
            <Charts />
          </section>
        </div>
      </div>
    </main>
  );
}
