'use client';

import axios from 'axios';
import React, { useState } from 'react';
import config from '../../../config';
import { useQuery } from '@tanstack/react-query';
import { useRouter } from 'next/navigation';
import Link from 'next/link';
import { formatAmount, formatGasPrice, timeAgo, truncateHash } from '../../lib/helpers';
import type { PendingTransaction } from '@/app/types';
import Badge from '../../components/Badge';
import Pagination from '../../components/Pagination';

interface PaginatedResponse {
  transactions?: PendingTransaction[];
  total?: number;
  page?: number;
  limit?: number;
  totalPages?: number;
}

const ITEMS_PER_PAGE = 10;

const fetchPendingTransactions = async (page: number): Promise<PaginatedResponse> => {
  const response = await axios.get<PaginatedResponse>(`${config.handlerUrl}/pending-transactions`, {
    params: { page, limit: ITEMS_PER_PAGE },
  });
  return response.data;
};

function flattenTransactions(data: PaginatedResponse | undefined): PendingTransaction[] {
  const transactions: PendingTransaction[] = [];
  try {
    if (Array.isArray(data?.transactions)) {
      transactions.push(...data.transactions);
    }
  } catch (err) {
    console.error('Error processing transactions:', err);
  }
  transactions.sort((a, b) => b.createdAt - a.createdAt);
  return transactions;
}

interface PendingListProps {
  initialData: PaginatedResponse;
  currentPage: number;
}

export default function PendingList({ initialData, currentPage }: PendingListProps): JSX.Element {
  const router = useRouter();
  const [isRefreshing, setIsRefreshing] = useState(false);
  const { data, isError, error, refetch, isFetching } = useQuery({
    queryKey: ['pending-transactions', currentPage],
    queryFn: () => fetchPendingTransactions(currentPage),
    initialData,
    refetchInterval: 5000,
  });

  const handleRefresh = async (): Promise<void> => {
    setIsRefreshing(true);
    await refetch();
    setTimeout(() => setIsRefreshing(false), 500);
  };

  if (isError) {
    return (
      <div className="p-4 sm:p-8 max-w-6xl mx-auto">
        <h1 className="text-xl sm:text-2xl font-bold mb-4 text-[#ffa729]">Pending Transactions</h1>
        <div className="bg-red-900/50 border border-red-500 text-red-200 px-4 py-3 rounded-xl">
          <p className="font-bold">Error:</p>
          <p className="text-sm">{error instanceof Error ? error.message : 'Failed to load pending transactions'}</p>
        </div>
      </div>
    );
  }

  const transactions = flattenTransactions(data);
  const totalPages = data?.totalPages ?? Math.max(1, Math.ceil((data?.total ?? 0) / ITEMS_PER_PAGE));

  const statusVariant = (status: string): 'warning' | 'error' | 'success' => {
    if (status === 'dropped') return 'error';
    if (status === 'mined') return 'success';
    return 'warning';
  };

  return (
    <div className="p-4 sm:p-8 max-w-6xl mx-auto">
      <div className="flex items-center justify-between mb-4">
        <h1 className="text-xl sm:text-2xl font-bold text-[#ffa729]">Pending Transactions</h1>
        <button
          onClick={handleRefresh}
          disabled={isRefreshing || isFetching}
          className="flex items-center gap-2 px-4 py-2 text-sm rounded-lg bg-[#1e1e1e] text-gray-300 border border-[#2a2a2a]
                     hover:border-[#ffa729] disabled:opacity-50 transition-colors"
        >
          <svg
            className={`w-4 h-4 ${isRefreshing || isFetching ? 'animate-spin' : ''}`}
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
          </svg>
          {isRefreshing || isFetching ? 'Checking...' : 'Refresh'}
        </button>
      </div>

      {transactions.length === 0 ? (
        <div className="rounded-xl bg-[#1e1e1e] border border-[#2a2a2a] px-4 py-12 text-center">
          <p className="text-gray-400">No pending transactions in the mempool.</p>
        </div>
      ) : (
        <>
          <div className="rounded-xl bg-[#1e1e1e] border border-[#2a2a2a] overflow-hidden mb-6">
            <div className="overflow-x-auto">
              <table className="w-full text-sm">
                <thead>
                  <tr className="border-b border-[#2a2a2a]">
                    <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Hash</th>
                    <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden sm:table-cell">From</th>
                    <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden sm:table-cell">To</th>
                    <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Value</th>
                    <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Status</th>
                    <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden md:table-cell">Time</th>
                  </tr>
                </thead>
                <tbody>
                  {transactions.map((tx) => {
                    const [formattedValue] = formatAmount(tx.value);
                    return (
                      <tr
                        key={tx.hash}
                        className="border-b border-[#2a2a2a] last:border-b-0 hover:bg-[#252525] transition-colors"
                      >
                        <td className="px-4 py-3">
                          <Link
                            href={`/pending/tx/${tx.hash}`}
                            className="text-[#ffa729] hover:text-[#ffb954] hover:underline font-mono text-xs"
                            title={tx.hash}
                          >
                            {truncateHash(tx.hash, 10, 6)}
                          </Link>
                        </td>
                        <td className="px-4 py-3 hidden sm:table-cell">
                          <span className="text-gray-400 font-mono text-xs" title={tx.from}>
                            {truncateHash(tx.from, 8, 6)}
                          </span>
                        </td>
                        <td className="px-4 py-3 hidden sm:table-cell">
                          <span className="text-gray-400 font-mono text-xs" title={tx.to}>
                            {tx.to ? truncateHash(tx.to, 8, 6) : 'Contract Create'}
                          </span>
                        </td>
                        <td className="px-4 py-3 text-gray-300 tabular-nums whitespace-nowrap">
                          {formattedValue}
                          <span className="text-gray-500 text-xs ml-1">QRL</span>
                        </td>
                        <td className="px-4 py-3">
                          <Badge variant={statusVariant(tx.status)} dot>
                            {tx.status}
                          </Badge>
                        </td>
                        <td className="px-4 py-3 text-gray-400 tabular-nums hidden md:table-cell">
                          {tx.createdAt ? timeAgo(tx.createdAt) : '-'}
                        </td>
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
          </div>

          {totalPages > 1 && (
            <Pagination
              currentPage={currentPage}
              totalPages={totalPages}
              onPrevious={() => router.push(`/pending/${Math.max(currentPage - 1, 1)}`)}
              onNext={() => router.push(`/pending/${Math.min(currentPage + 1, totalPages)}`)}
            />
          )}
        </>
      )}
    </div>
  );
}
