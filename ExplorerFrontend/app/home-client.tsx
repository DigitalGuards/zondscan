'use client';

import * as React from 'react';
import dynamic from 'next/dynamic';
import Link from 'next/link';
import axios from 'axios';
import { formatNumberWithCommas, truncateHash, formatAddress } from './lib/helpers';
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

interface EpochInfo {
  headEpoch: string;
  headSlot: string;
  finalizedEpoch: string;
  justifiedEpoch: string;
  slotsPerEpoch: number;
  secondsPerSlot: number;
  slotInEpoch: number;
  timeToNextEpoch: number;
  updatedAt: number;
}

interface BlockResult {
  number: string;
  timestamp: string;
  hash: string;
  miner: string;
  transactions: any[];
}

interface HomeData {
  blockHeight: number;
  totalTransactions: number;
  validatorCount: number;
  epochInfo: EpochInfo | null;
  blocks: BlockResult[];
  totalStaked: string;
  loading: boolean;
  error: boolean;
  dataInitialized: boolean;
}

// ── Helpers ──────────────────────────────────────────────────────────────────

const SLOTS_PER_EPOCH = 128;
const SECONDS_PER_SLOT = 60;

function timeAgo(unixSeconds: number): string {
  const now = Math.floor(Date.now() / 1000);
  const diff = now - unixSeconds;
  if (diff < 0) return 'just now';
  if (diff < 60) return `${diff}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}

function parseHex(hex: string): number {
  if (!hex) return 0;
  if (hex.startsWith('0x')) return parseInt(hex, 16);
  return parseInt(hex, 10) || 0;
}

/** Derive recent epoch rows from epoch info */
function deriveEpochs(epochInfo: EpochInfo | null): EpochRow[] {
  if (!epochInfo) return [];
  const head = parseInt(epochInfo.headEpoch);
  const finalized = parseInt(epochInfo.finalizedEpoch);
  const justified = parseInt(epochInfo.justifiedEpoch);
  const now = Math.floor(Date.now() / 1000);
  const epochDuration = SLOTS_PER_EPOCH * SECONDS_PER_SLOT; // 7680s

  // Current epoch start time estimate
  const slotInEpoch = epochInfo.slotInEpoch || 0;
  const currentEpochStartTime = now - slotInEpoch * SECONDS_PER_SLOT;

  const rows: EpochRow[] = [];
  const count = Math.min(head + 1, 10);
  for (let i = 0; i < count; i++) {
    const epoch = head - i;
    if (epoch < 0) break;
    const epochStartTime = currentEpochStartTime - i * epochDuration;
    let status: 'finalized' | 'justified' | 'pending';
    if (epoch <= finalized) status = 'finalized';
    else if (epoch <= justified) status = 'justified';
    else status = 'pending';
    rows.push({ epoch, time: epochStartTime, status });
  }
  return rows;
}

interface EpochRow {
  epoch: number;
  time: number;
  status: 'finalized' | 'justified' | 'pending';
}

// ── Icons ────────────────────────────────────────────────────────────────────

const icons = {
  epoch: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
    </svg>
  ),
  slot: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M13 10V3L4 14h7v7l9-11h-7z" />
    </svg>
  ),
  block: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M20 7l-8-4-8 4m16 0l-8 4m8-4v10l-8 4m0-10L4 7m8 4v10M4 7v10l8 4" />
    </svg>
  ),
  validators: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M9 12l2 2 4-4m5.618-4.016A11.955 11.955 0 0112 2.944a11.955 11.955 0 01-8.618 3.04A12.02 12.02 0 003 9c0 5.591 3.824 10.29 9 11.622 5.176-1.332 9-6.03 9-11.622 0-1.042-.133-2.052-.382-3.016z" />
    </svg>
  ),
  transactions: (
    <svg xmlns="http://www.w3.org/2000/svg" className="h-3.5 w-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
      <path strokeLinecap="round" strokeLinejoin="round" d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
    </svg>
  ),
};

// ── Stat Bar ─────────────────────────────────────────────────────────────────

function StatBar({ data }: { data: HomeData }) {
  const stats = [
    { label: 'Epoch', value: data.epochInfo ? data.epochInfo.headEpoch : '—', icon: icons.epoch },
    { label: 'Slot', value: data.epochInfo ? formatNumberWithCommas(data.epochInfo.headSlot) : '—', icon: icons.slot },
    { label: 'Block Height', value: formatNumberWithCommas(data.blockHeight.toString()), icon: icons.block },
    { label: 'Validators', value: formatNumberWithCommas(data.validatorCount.toString()), icon: icons.validators },
    { label: 'Transactions', value: formatNumberWithCommas(data.totalTransactions.toString()), icon: icons.transactions },
  ];

  return (
    <div className="rounded-xl bg-[#1e1e1e] border border-[#2a2a2a] overflow-hidden">
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5">
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

// ── Status Badge ─────────────────────────────────────────────────────────────

function StatusBadge({ status }: { status: 'finalized' | 'justified' | 'pending' | 'proposed' }) {
  const styles: Record<string, string> = {
    finalized: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/20',
    justified: 'bg-blue-500/15 text-blue-400 border-blue-500/20',
    pending: 'bg-yellow-500/15 text-yellow-400 border-yellow-500/20',
    proposed: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/20',
  };
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-[11px] font-medium border ${styles[status]}`}>
      {status.charAt(0).toUpperCase() + status.slice(1)}
    </span>
  );
}

// ── Epoch Table ──────────────────────────────────────────────────────────────

function EpochTable({ epochs, loading }: { epochs: EpochRow[]; loading: boolean }) {
  return (
    <div className="rounded-xl bg-[#1e1e1e] border border-[#2a2a2a] overflow-hidden flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-[#2a2a2a]">
        <h2 className="flex items-center gap-2 text-[15px] font-semibold text-white">
          <span className="text-[#ffa729]">{icons.epoch}</span>
          Latest Epochs
        </h2>
        <Link href="/validators" className="text-xs text-[#ffa729] hover:text-[#ffb954] hover:underline transition-colors">
          View all &rarr;
        </Link>
      </div>

      {/* Table */}
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-[#2a2a2a]">
              <th className="text-left px-4 py-2.5 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Epoch</th>
              <th className="text-left px-4 py-2.5 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Time</th>
              <th className="text-left px-4 py-2.5 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Status</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              Array.from({ length: 8 }).map((_, i) => (
                <tr key={i} className="border-b border-[#2a2a2a] last:border-b-0">
                  <td className="px-4 py-2.5"><div className="h-4 w-12 bg-[#2a2a2a] rounded animate-pulse" /></td>
                  <td className="px-4 py-2.5"><div className="h-4 w-16 bg-[#2a2a2a] rounded animate-pulse" /></td>
                  <td className="px-4 py-2.5"><div className="h-4 w-16 bg-[#2a2a2a] rounded animate-pulse" /></td>
                </tr>
              ))
            ) : epochs.length === 0 ? (
              <tr>
                <td colSpan={3} className="px-4 py-8 text-center text-gray-500">No epoch data available</td>
              </tr>
            ) : (
              epochs.map((epoch) => (
                <tr
                  key={epoch.epoch}
                  className="border-b border-[#2a2a2a] last:border-b-0 hover:bg-[#252525] transition-colors"
                >
                  <td className="px-4 py-2.5">
                    <Link href="/validators" className="text-[#ffa729] hover:text-[#ffb954] hover:underline font-medium tabular-nums">
                      {formatNumberWithCommas(epoch.epoch.toString())}
                    </Link>
                  </td>
                  <td className="px-4 py-2.5 text-gray-400 tabular-nums">{timeAgo(epoch.time)}</td>
                  <td className="px-4 py-2.5">
                    <StatusBadge status={epoch.status} />
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
}

// ── Block Table ──────────────────────────────────────────────────────────────

function BlockTable({ blocks, epochInfo, loading }: { blocks: BlockResult[]; epochInfo: EpochInfo | null; loading: boolean }) {
  function getSlotFromBlock(blockNumber: string): number {
    return parseHex(blockNumber);
  }

  function getEpochFromSlot(slot: number): number {
    return Math.floor(slot / SLOTS_PER_EPOCH);
  }

  return (
    <div className="rounded-xl bg-[#1e1e1e] border border-[#2a2a2a] overflow-hidden flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-[#2a2a2a]">
        <h2 className="flex items-center gap-2 text-[15px] font-semibold text-white">
          <span className="text-[#ffa729]">{icons.block}</span>
          Latest Blocks
        </h2>
        <Link href="/blocks/1" className="text-xs text-[#ffa729] hover:text-[#ffb954] hover:underline transition-colors">
          View all &rarr;
        </Link>
      </div>

      {/* Table */}
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-[#2a2a2a]">
              <th className="text-left px-4 py-2.5 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Block</th>
              <th className="text-left px-4 py-2.5 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden sm:table-cell">Epoch</th>
              <th className="text-left px-4 py-2.5 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Time</th>
              <th className="text-left px-4 py-2.5 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Txns</th>
              <th className="text-left px-4 py-2.5 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden md:table-cell">Proposer</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              Array.from({ length: 8 }).map((_, i) => (
                <tr key={i} className="border-b border-[#2a2a2a] last:border-b-0">
                  <td className="px-4 py-2.5"><div className="h-4 w-14 bg-[#2a2a2a] rounded animate-pulse" /></td>
                  <td className="px-4 py-2.5 hidden sm:table-cell"><div className="h-4 w-10 bg-[#2a2a2a] rounded animate-pulse" /></td>
                  <td className="px-4 py-2.5"><div className="h-4 w-14 bg-[#2a2a2a] rounded animate-pulse" /></td>
                  <td className="px-4 py-2.5"><div className="h-4 w-6 bg-[#2a2a2a] rounded animate-pulse" /></td>
                  <td className="px-4 py-2.5 hidden md:table-cell"><div className="h-4 w-24 bg-[#2a2a2a] rounded animate-pulse" /></td>
                </tr>
              ))
            ) : blocks.length === 0 ? (
              <tr>
                <td colSpan={5} className="px-4 py-8 text-center text-gray-500">No blocks found</td>
              </tr>
            ) : (
              blocks.map((block) => {
                const blockNum = parseHex(block.number);
                const slot = getSlotFromBlock(block.number);
                const epoch = getEpochFromSlot(slot);
                const timestamp = parseHex(block.timestamp);
                const txCount = block.transactions?.length || 0;
                const proposer = formatAddress(block.miner);

                return (
                  <tr
                    key={block.hash}
                    className="border-b border-[#2a2a2a] last:border-b-0 hover:bg-[#252525] transition-colors"
                  >
                    <td className="px-4 py-2.5">
                      <Link href={`/block/${blockNum}`} className="text-[#ffa729] hover:text-[#ffb954] hover:underline font-medium tabular-nums">
                        {formatNumberWithCommas(blockNum.toString())}
                      </Link>
                    </td>
                    <td className="px-4 py-2.5 text-gray-400 tabular-nums hidden sm:table-cell">
                      {formatNumberWithCommas(epoch.toString())}
                    </td>
                    <td className="px-4 py-2.5 text-gray-400 tabular-nums">{timeAgo(timestamp)}</td>
                    <td className="px-4 py-2.5 text-gray-300 tabular-nums">{txCount}</td>
                    <td className="px-4 py-2.5 hidden md:table-cell">
                      <Link href={`/address/${proposer}`} className="text-gray-400 hover:text-[#ffa729] hover:underline font-mono text-xs transition-colors">
                        {truncateHash(proposer, 8, 6)}
                      </Link>
                    </td>
                  </tr>
                );
              })
            )}
          </tbody>
        </table>
      </div>
    </div>
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
    totalStaked: '0',
    loading: true,
    error: false,
    dataInitialized: false,
  });

  const fetchData = React.useCallback(async () => {
    try {
      if (!config.handlerUrl) return;

      const [overviewRes, latestBlockRes, txsRes, epochRes, blocksRes] = await Promise.allSettled([
        axios.get(config.handlerUrl + '/overview'),
        axios.get(config.handlerUrl + '/latestblock'),
        axios.get(config.handlerUrl + '/txs?page=1'),
        axios.get(config.handlerUrl + '/epoch'),
        axios.get(config.handlerUrl + '/blocks?page=1&limit=10'),
      ]);

      setData((prev) => {
        const next = { ...prev, loading: false };

        if (overviewRes.status === 'fulfilled') {
          next.validatorCount = overviewRes.value.data.validatorCount || 0;
          next.dataInitialized = overviewRes.value.data.status?.dataInitialized ?? false;
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

        return next;
      });
    } catch (err) {
      console.error('Failed to fetch homepage data:', err);
      setData((prev) => ({ ...prev, loading: false, error: true }));
    }
  }, []);

  React.useEffect(() => {
    fetchData();
    // Refresh every 30s (approx half a slot)
    const interval = setInterval(fetchData, 30000);
    return () => clearInterval(interval);
  }, [fetchData]);

  const epochs = React.useMemo(() => deriveEpochs(data.epochInfo), [data.epochInfo]);

  return (
    <div className="px-4 lg:px-8 pt-4 lg:pt-6">
      <div className="max-w-6xl mx-auto">
        {/* Search Bar */}
        <h1 className="text-base sm:text-lg font-semibold mb-2 text-white">{pageTitle}</h1>
        <div className="mb-6">
          <SearchBar />
        </div>

        {/* Init Warning */}
        {!data.loading && !data.dataInitialized && (
          <div className="mb-4 px-3 py-2.5 rounded-lg bg-yellow-500/10 border border-yellow-500/20 text-yellow-500 text-xs sm:text-sm flex items-center gap-2">
            <svg xmlns="http://www.w3.org/2000/svg" className="h-4 w-4 flex-shrink-0" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            Initializing explorer data... This may take a few minutes.
          </div>
        )}

        {/* Stats Bar */}
        <div className="mb-6">
          <StatBar data={data} />
        </div>

        {/* Side-by-Side Tables */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 mb-6">
          <EpochTable epochs={epochs} loading={data.loading} />
          <BlockTable blocks={data.blocks} epochInfo={data.epochInfo} loading={data.loading} />
        </div>

        {/* TradingView Chart */}
        <div className="mb-8">
          <Charts />
        </div>
      </div>
    </div>
  );
}
