'use client';

import React from 'react';
import Link from 'next/link';
import axios from 'axios';
import config from '../../../config';
import { useQuery } from '@tanstack/react-query';
import { useRouter } from 'next/navigation';
import { formatNumberWithCommas } from '../../lib/helpers';
import SearchBar from '../../components/SearchBar';

// ── Types ────────────────────────────────────────────────────────────────────

interface EpochItem {
  epoch: string;
  timestamp: number;
  status: string;
  validatorsCount: number;
  activeCount: number;
  totalStaked: string;
}

interface EpochsData {
  epochs: EpochItem[];
  total: number;
  finalizedEpoch: string;
  justifiedEpoch: string;
}

// ── Helpers ──────────────────────────────────────────────────────────────────

const ITEMS_PER_PAGE = 15;

function timeAgo(unixSeconds: number): string {
  const now = Math.floor(Date.now() / 1000);
  const diff = now - unixSeconds;
  if (diff < 0) return 'just now';
  if (diff < 60) return `${diff}s ago`;
  if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
  if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
  return `${Math.floor(diff / 86400)}d ago`;
}

function formatStaked(gwei: string): string {
  try {
    const val = BigInt(gwei);
    const qrl = val / BigInt(1_000_000_000);
    return formatNumberWithCommas(qrl.toString()) + ' QRL';
  } catch {
    return '0 QRL';
  }
}

// ── Status Badge ─────────────────────────────────────────────────────────────

function StatusBadge({ status }: { status: string }) {
  const styles: Record<string, string> = {
    finalized: 'bg-emerald-500/15 text-emerald-400 border-emerald-500/20',
    justified: 'bg-blue-500/15 text-blue-400 border-blue-500/20',
    pending: 'bg-yellow-500/15 text-yellow-400 border-yellow-500/20',
  };
  return (
    <span className={`inline-flex items-center px-2 py-0.5 rounded text-[11px] font-medium border ${styles[status] || styles.pending}`}>
      {status.charAt(0).toUpperCase() + status.slice(1)}
    </span>
  );
}

// ── Data Fetching ────────────────────────────────────────────────────────────

const fetchEpochs = async (page: string): Promise<EpochsData> => {
  const response = await axios.get<EpochsData>(`${config.handlerUrl}/epochs?page=${page}&limit=${ITEMS_PER_PAGE}`);
  return response.data;
};

// ── Component ────────────────────────────────────────────────────────────────

interface EpochsClientProps {
  initialData: EpochsData;
  initialPage: string;
}

export default function EpochsClient({ initialData, initialPage }: EpochsClientProps): JSX.Element {
  const router = useRouter();
  const currentPage = parseInt(initialPage);

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['epochs', initialPage],
    queryFn: () => fetchEpochs(initialPage),
    staleTime: 60000,
    gcTime: 5 * 60 * 1000,
    retry: 2,
    initialData,
  });

  const totalPages = data ? Math.max(1, Math.ceil(data.total / ITEMS_PER_PAGE)) : 1;

  const goToNextPage = (): void => {
    router.push(`/epochs/${Math.min(currentPage + 1, totalPages)}`);
  };

  const goToPreviousPage = (): void => {
    router.push(`/epochs/${Math.max(currentPage - 1, 1)}`);
  };

  if (isError) {
    return (
      <div className="p-4 sm:p-8 max-w-6xl mx-auto">
        <h1 className="text-xl sm:text-2xl font-bold mb-4 text-[#ffa729]">Epochs</h1>
        <div className="bg-red-900/50 border border-red-500 text-red-200 px-4 py-3 rounded-xl">
          <p className="font-bold">Error:</p>
          <p className="text-sm">{error instanceof Error ? error.message : 'Failed to load epochs'}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-4 sm:p-8 max-w-6xl mx-auto">
      <h1 className="flex items-center gap-2 text-xl sm:text-2xl font-bold mb-4 text-[#ffa729]">
        <svg xmlns="http://www.w3.org/2000/svg" className="h-5 w-5 sm:h-6 sm:w-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={2}>
          <path strokeLinecap="round" strokeLinejoin="round" d="M12 8v4l3 3m6-3a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
        Epochs
      </h1>

      <div className="mb-6">
        <SearchBar />
      </div>

      {/* Epochs Table */}
      <div className="rounded-xl bg-[#1e1e1e] border border-[#2a2a2a] overflow-hidden mb-6">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-[#2a2a2a]">
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Epoch</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Time</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Status</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden sm:table-cell">Validators</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden sm:table-cell">Active</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden md:table-cell">Total Staked</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                Array.from({ length: ITEMS_PER_PAGE }).map((_, i) => (
                  <tr key={i} className="border-b border-[#2a2a2a] last:border-b-0">
                    <td className="px-4 py-3"><div className="h-4 w-12 bg-[#2a2a2a] rounded animate-pulse" /></td>
                    <td className="px-4 py-3"><div className="h-4 w-16 bg-[#2a2a2a] rounded animate-pulse" /></td>
                    <td className="px-4 py-3"><div className="h-4 w-16 bg-[#2a2a2a] rounded animate-pulse" /></td>
                    <td className="px-4 py-3 hidden sm:table-cell"><div className="h-4 w-10 bg-[#2a2a2a] rounded animate-pulse" /></td>
                    <td className="px-4 py-3 hidden sm:table-cell"><div className="h-4 w-10 bg-[#2a2a2a] rounded animate-pulse" /></td>
                    <td className="px-4 py-3 hidden md:table-cell"><div className="h-4 w-24 bg-[#2a2a2a] rounded animate-pulse" /></td>
                  </tr>
                ))
              ) : !data?.epochs?.length ? (
                <tr>
                  <td colSpan={6} className="px-4 py-12 text-center text-gray-500">
                    No epoch data available yet. The explorer is still syncing.
                  </td>
                </tr>
              ) : (
                data.epochs.map((epoch) => (
                  <tr
                    key={epoch.epoch}
                    className="border-b border-[#2a2a2a] last:border-b-0 hover:bg-[#252525] transition-colors"
                  >
                    <td className="px-4 py-3">
                      <Link href={`/epoch/${epoch.epoch}`} className="text-[#ffa729] hover:text-[#ffb954] hover:underline font-medium tabular-nums">
                        {formatNumberWithCommas(epoch.epoch)}
                      </Link>
                    </td>
                    <td className="px-4 py-3 text-gray-400 tabular-nums">{timeAgo(epoch.timestamp)}</td>
                    <td className="px-4 py-3">
                      <StatusBadge status={epoch.status} />
                    </td>
                    <td className="px-4 py-3 text-gray-300 tabular-nums hidden sm:table-cell">
                      {formatNumberWithCommas(epoch.validatorsCount.toString())}
                    </td>
                    <td className="px-4 py-3 text-gray-300 tabular-nums hidden sm:table-cell">
                      {formatNumberWithCommas(epoch.activeCount.toString())}
                    </td>
                    <td className="px-4 py-3 text-gray-400 tabular-nums hidden md:table-cell">
                      {formatStaked(epoch.totalStaked)}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Pagination */}
      <div className="flex justify-center items-center gap-3 text-sm text-gray-300">
        <button
          onClick={goToPreviousPage}
          disabled={currentPage === 1}
          className="px-4 py-2 rounded-lg bg-[#1e1e1e] text-gray-300 border border-[#2a2a2a]
                     hover:border-[#ffa729] disabled:opacity-50 disabled:hover:border-[#2a2a2a]
                     transition-colors"
        >
          Previous
        </button>
        <span className="tabular-nums">Page {currentPage} of {totalPages}</span>
        <button
          onClick={goToNextPage}
          disabled={currentPage >= totalPages}
          className="px-4 py-2 rounded-lg bg-[#1e1e1e] text-gray-300 border border-[#2a2a2a]
                     hover:border-[#ffa729] disabled:opacity-50 disabled:hover:border-[#2a2a2a]
                     transition-colors"
        >
          Next
        </button>
      </div>
    </div>
  );
}
