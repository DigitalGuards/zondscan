'use client';

import * as React from 'react';
import dynamic from 'next/dynamic';
import Link from 'next/link';
import axios from 'axios';
import { useQuery } from '@tanstack/react-query';
import { formatNumberWithCommas, timeAgo, formatStaked, formatGasPrice, truncateHash, formatAmount, formatAddress } from './lib/helpers';
import type { EpochInfo } from './types';
import config from '../config.js';
import SearchBar from './components/SearchBar';

const Charts = dynamic(() => import('./components/Charts'), {
  loading: () => (
    <div className="h-[400px] bg-[#1e1e1e] rounded-xl border border-[#2a2a2a] animate-pulse flex items-center justify-center">
      <span className="text-gray-500">Loading chart...</span>
    </div>
  ),
  ssr: false,
});

// ── Types ────────────────────────────────────────────────────────────────────

interface BlockResult {
  number: string;
  timestamp: string;
  hash: string;
  miner: string;
  // Only `.length` is read on the home page; the per-tx shape varies by
  // endpoint, so `unknown[]` keeps it honest without inventing a type.
  transactions: unknown[];
}

interface TxResult {
  TxHash: string;
  TimeStamp: string | number;
  From: string;
  To: string;
  Amount: string | number;
  BlockNumber?: string;
  TxType?: string;
}

interface HomeData {
  blockHeight: number;
  totalTransactions: number;
  validatorCount: number;
  epochInfo: EpochInfo | null;
  blocks: BlockResult[];
  txs: TxResult[];
  totalStaked: string;
  marketCap: number;
  avgGasPriceHex: string | null;
  loading: boolean;
  error: boolean;
  dataInitialized: boolean;
}

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

/** Dedupe txs by hash (the network endpoint returns one row per side of a transfer). */
function dedupeTxs(txs: TxResult[], max: number): TxResult[] {
  const seen = new Set<string>();
  const out: TxResult[] = [];
  for (const tx of txs) {
    if (!tx?.TxHash || seen.has(tx.TxHash)) continue;
    seen.add(tx.TxHash);
    out.push(tx);
    if (out.length >= max) break;
  }
  return out;
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
};

// ── Stat Bar ─────────────────────────────────────────────────────────────────

function StatBar({ data }: { data: HomeData }) {
  const stats = [
    { label: 'Epoch', value: data.epochInfo ? data.epochInfo.headEpoch : ',', icon: icons.epoch },
    { label: 'Avg Gas Price', value: data.avgGasPriceHex ? `${formatGasPrice(data.avgGasPriceHex)} Shor` : ',', icon: icons.gas },
    { label: 'Block Height', value: formatNumberWithCommas(data.blockHeight.toString()), icon: icons.block },
    { label: 'Validators', value: formatNumberWithCommas(data.validatorCount.toString()), icon: icons.validators },
    { label: 'Staked QRL', value: data.totalStaked !== '0' ? formatStaked(data.totalStaked) : ',', icon: icons.staked },
    { label: 'Transactions', value: formatNumberWithCommas(data.totalTransactions.toString()), icon: icons.transactions },
    { label: 'Market Cap', value: data.marketCap > 0 ? '$' + formatNumberWithCommas(data.marketCap.toString()) : ',', icon: icons.marketCap },
  ];

  return (
    <div className="rounded-xl bg-[#1e1e1e] border border-[#2a2a2a] overflow-hidden">
      <div className="grid grid-cols-2 sm:grid-cols-4 lg:grid-cols-7">
        {stats.map((stat, i) => (
          <div
            key={stat.label}
            className={`px-4 py-4 sm:py-5 flex flex-col items-center justify-center text-center
                        ${i < stats.length - 1 ? 'border-b lg:border-b-0 lg:border-r border-[#2a2a2a]' : ''}
                        ${i % 2 === 0 && i < stats.length - 1 ? 'border-r sm:border-r border-[#2a2a2a]' : ''}
                        `}
          >
            {data.loading ? (
              <div className="h-7 w-20 bg-[#2a2a2a] rounded animate-pulse" />
            ) : (
              <span className="text-lg sm:text-xl font-semibold text-white tabular-nums">
                {stat.value}
              </span>
            )}
            <span className="flex items-center gap-1.5 text-[11px] text-gray-500 mt-1.5 uppercase tracking-wider">
              <span className="text-gray-600">{stat.icon}</span>
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

const ROW_CLASS = 'flex items-center gap-3 px-4 py-3 border-b border-[#2a2a2a] last:border-b-0 text-sm';

function TableHeader({ icon, title, viewAllHref }: { icon: React.ReactNode; title: string; viewAllHref: string }) {
  return (
    <div className="flex items-center justify-between px-4 py-3 border-b border-[#2a2a2a]">
      <h2 className="flex items-center gap-2 text-[15px] font-semibold text-[#ffa729]">
        {icon}
        {title}
      </h2>
      <Link href={viewAllHref} className="text-xs text-[#ffa729] hover:text-[#ffb954] hover:underline transition-colors">
        View all &rarr;
      </Link>
    </div>
  );
}

function RowIcon({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex-shrink-0 w-8 h-8 rounded-md bg-[#2a2a2a] flex items-center justify-center text-gray-400">
      {children}
    </div>
  );
}

function ValueBadge({ children }: { children: React.ReactNode }) {
  return (
    <span className="inline-flex items-center px-2 py-0.5 rounded-md bg-[#2a2a2a] border border-[#3a3a3a] text-[11px] text-gray-300 tabular-nums">
      {children}
    </span>
  );
}

function SkeletonRow() {
  return (
    <div className={ROW_CLASS}>
      <div className="flex-shrink-0 w-8 h-8 rounded-md bg-[#2a2a2a] animate-pulse" />
      <div className="flex-1 min-w-0 space-y-1.5">
        <div className="h-3.5 w-24 bg-[#2a2a2a] rounded animate-pulse" />
        <div className="h-3 w-16 bg-[#2a2a2a] rounded animate-pulse" />
      </div>
      <div className="flex-1 min-w-0 space-y-1.5 hidden sm:block">
        <div className="h-3 w-28 bg-[#2a2a2a] rounded animate-pulse" />
        <div className="h-3 w-28 bg-[#2a2a2a] rounded animate-pulse" />
      </div>
      <div className="h-5 w-16 bg-[#2a2a2a] rounded animate-pulse" />
    </div>
  );
}

function getEpochFromBlock(blockNumber: number): number {
  return Math.floor(blockNumber / SLOTS_PER_EPOCH);
}

// ── Block Table ──────────────────────────────────────────────────────────────

function BlockTable({ blocks, loading }: { blocks: BlockResult[]; loading: boolean }) {
  return (
    <section aria-label="Latest blocks" className="rounded-xl bg-[#1e1e1e] border border-[#2a2a2a] overflow-hidden">
      <TableHeader icon={icons.block} title="Latest Blocks" viewAllHref="/blocks/1" />
      <div>
        {loading
          ? Array.from({ length: TABLE_ROWS }).map((_, i) => <SkeletonRow key={i} />)
          : blocks.slice(0, TABLE_ROWS).map((block, idx) => {
              const blockNum = parseHex(block.number);
              const epoch = getEpochFromBlock(blockNum);
              const timestamp = parseHex(block.timestamp);
              const txCount = block.transactions?.length || 0;
              const miner = block.miner ? formatAddress(block.miner) : '';

              return (
                <div key={`${block.number}-${idx}`} className={`${ROW_CLASS} hover:bg-[#252525] transition-colors`}>
                  <RowIcon>{icons.block}</RowIcon>

                  <div className="flex-1 min-w-0">
                    <Link
                      href={`/block/${blockNum}`}
                      className="block text-[#ffa729] hover:text-[#ffb954] hover:underline font-medium tabular-nums truncate"
                    >
                      {formatNumberWithCommas(blockNum.toString())}
                    </Link>
                    <span className="text-[11px] text-gray-500 tabular-nums">{timeAgo(timestamp)}</span>
                  </div>

                  <div className="flex-1 min-w-0 hidden sm:block">
                    {miner ? (
                      <div className="flex items-center gap-1 text-[12px] truncate">
                        <span className="text-gray-500">Miner</span>
                        <Link
                          href={`/address/${miner}`}
                          className="text-gray-300 hover:text-[#ffa729] hover:underline font-mono truncate"
                        >
                          {truncateHash(miner, 8, 6)}
                        </Link>
                      </div>
                    ) : null}
                    <div className="text-[11px] text-gray-500 tabular-nums">
                      <Link href={`/block/${blockNum}`} className="hover:text-[#ffa729] hover:underline">
                        {txCount} txn{txCount === 1 ? '' : 's'}
                      </Link>
                      <span className="text-gray-600"> · epoch {formatNumberWithCommas(epoch.toString())}</span>
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

function TransactionTable({ txs, loading }: { txs: TxResult[]; loading: boolean }) {
  return (
    <section aria-label="Latest transactions" className="rounded-xl bg-[#1e1e1e] border border-[#2a2a2a] overflow-hidden">
      <TableHeader icon={icons.transactions} title="Latest Transactions" viewAllHref="/transactions/1" />
      <div>
        {loading
          ? Array.from({ length: TABLE_ROWS }).map((_, i) => <SkeletonRow key={i} />)
          : txs.slice(0, TABLE_ROWS).map((tx, idx) => {
              const timestamp = parseTimestamp(tx.TimeStamp);
              const from = tx.From ? formatAddress(tx.From) : '';
              const to = tx.To ? formatAddress(tx.To) : '';
              const [amount, unit] = formatAmount(tx.Amount);

              return (
                <div key={`${tx.TxHash}-${idx}`} className={`${ROW_CLASS} hover:bg-[#252525] transition-colors`}>
                  <RowIcon>{icons.transactions}</RowIcon>

                  <div className="flex-1 min-w-0">
                    <Link
                      href={`/tx/${tx.TxHash}`}
                      className="block text-[#ffa729] hover:text-[#ffb954] hover:underline font-mono text-[13px] truncate"
                    >
                      {truncateHash(tx.TxHash, 10, 6)}
                    </Link>
                    <span className="text-[11px] text-gray-500 tabular-nums">{timeAgo(timestamp)}</span>
                  </div>

                  <div className="flex-1 min-w-0 hidden sm:block">
                    <div className="flex items-center gap-1 text-[12px] truncate">
                      <span className="text-gray-500 w-8 flex-shrink-0">From</span>
                      {from ? (
                        <Link href={`/address/${from}`} className="text-gray-300 hover:text-[#ffa729] hover:underline font-mono truncate">
                          {truncateHash(from, 8, 6)}
                        </Link>
                      ) : <span className="text-gray-600">,</span>}
                    </div>
                    <div className="flex items-center gap-1 text-[12px] truncate">
                      <span className="text-gray-500 w-8 flex-shrink-0">To</span>
                      {to ? (
                        <Link href={`/address/${to}`} className="text-gray-300 hover:text-[#ffa729] hover:underline font-mono truncate">
                          {truncateHash(to, 8, 6)}
                        </Link>
                      ) : <span className="text-gray-600">,</span>}
                    </div>
                  </div>

                  <div className="flex-shrink-0">
                    <ValueBadge>
                      {amount} <span className="text-gray-500 ml-0.5">{unit}</span>
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

export default function HomeClient({ pageTitle }: { pageTitle: string }): JSX.Element {
  const [data, setData] = React.useState<HomeData>({
    blockHeight: 0,
    totalTransactions: 0,
    validatorCount: 0,
    epochInfo: null,
    blocks: [],
    txs: [],
    totalStaked: '0',
    marketCap: 0,
    avgGasPriceHex: null,
    loading: true,
    error: false,
    dataInitialized: false,
  });

  const fetchData = React.useCallback(async () => {
    try {
      if (!config.handlerUrl) return;

      const [overviewRes, latestBlockRes, txsRes, epochRes, blocksRes, epochsRes, gasRes, latestTxsRes] = await Promise.allSettled([
        axios.get(config.handlerUrl + '/overview'),
        axios.get(config.handlerUrl + '/latestblock'),
        axios.get(config.handlerUrl + '/txs?page=1'),
        axios.get(config.handlerUrl + '/epoch'),
        axios.get(config.handlerUrl + '/blocks?page=1&limit=10'),
        axios.get(config.handlerUrl + '/epochs?page=1&limit=1'),
        axios.get(config.handlerUrl + '/gas/summary'),
        axios.get(config.handlerUrl + '/transactions'),
      ]);

      setData((prev) => {
        const next = { ...prev, loading: false };

        if (overviewRes.status === 'fulfilled') {
          next.validatorCount = overviewRes.value.data.validatorCount || 0;
          next.dataInitialized = overviewRes.value.data.status?.dataInitialized ?? false;
          next.marketCap = overviewRes.value.data.marketcap || 0;
        }
        if (latestBlockRes.status === 'fulfilled') {
          next.blockHeight = latestBlockRes.value.data.blockNumber || 0;
        }
        if (txsRes.status === 'fulfilled') {
          next.totalTransactions = txsRes.value.data.total || 0;
        }
        if (epochRes.status === 'fulfilled' && epochRes.value.data) {
          next.epochInfo = epochRes.value.data;
        }
        if (blocksRes.status === 'fulfilled') {
          next.blocks = blocksRes.value.data.blocks || [];
        }
        if (epochsRes.status === 'fulfilled') {
          const latestEpoch = epochsRes.value.data.epochs?.[0];
          if (latestEpoch?.totalStaked) {
            next.totalStaked = latestEpoch.totalStaked;
          }
        }
        if (gasRes.status === 'fulfilled' && gasRes.value.data?.avgGasPriceHex) {
          next.avgGasPriceHex = gasRes.value.data.avgGasPriceHex;
        }
        if (latestTxsRes.status === 'fulfilled') {
          const rows = (latestTxsRes.value.data?.response || []) as TxResult[];
          next.txs = dedupeTxs(rows, TABLE_ROWS);
        }

        return next;
      });
    } catch (err) {
      console.error('Failed to fetch homepage data:', err);
      setData((prev) => ({ ...prev, loading: false, error: true }));
    }
  }, []);

  // Drive the polling via TanStack Query so backgrounded tabs go quiet
  // (every other live page uses this discipline; the previous raw
  // setInterval polled regardless of visibility). fetchData stays in
  // charge of writing into the local state; this query only owns the
  // schedule. Returns a timestamp so successive renders see a fresh
  // value and don't dedupe.
  useQuery<number>({
    queryKey: ['home-poll'],
    queryFn: async () => {
      await fetchData();
      return Date.now();
    },
    refetchInterval: 30000,
    refetchIntervalInBackground: false,
    refetchOnMount: 'always',
    staleTime: 0,
  });

  return (
    <main className="px-4 lg:px-8 pt-4 lg:pt-6" aria-labelledby="home-heading">
      <div className="max-w-6xl mx-auto">
        {/* Search Bar */}
        <h1 id="home-heading" className="text-base sm:text-lg font-semibold mb-2 text-white">{pageTitle}</h1>
        <div className="mb-6">
          <SearchBar />
        </div>

        {/* Init Warning */}
        {!data.loading && !data.dataInitialized && (
          <div
            role="status"
            className="mb-4 px-3 py-2.5 rounded-lg bg-yellow-500/10 border border-yellow-500/20 text-yellow-500 text-xs sm:text-sm flex items-center gap-2"
          >
            <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            Initializing explorer data... This may take a few minutes.
          </div>
        )}

        {/* Stats Bar */}
        <section aria-label="Network stats" className="mb-6">
          <StatBar data={data} />
        </section>

        {/* Side-by-Side Tables */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-6">
          <BlockTable blocks={data.blocks} loading={data.loading} />
          <TransactionTable txs={data.txs} loading={data.loading} />
        </div>

        {/* TradingView Chart */}
        <section aria-label="Price chart" className="mb-2">
          <Charts />
        </section>
      </div>
    </main>
  );
}
