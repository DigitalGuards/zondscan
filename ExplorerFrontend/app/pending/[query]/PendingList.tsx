"use client";

import axios from 'axios';
import React, { useState } from 'react';
import config from '../../../config';
import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import { formatAmount } from '../../lib/helpers';
import type { PendingTransaction } from '@/app/types';

interface PaginatedResponse {
  // New format fields
  jsonrpc?: string;
  id?: number;
  result?: {
    pending: {
      [address: string]: {
        [nonce: string]: PendingTransaction;
      };
    };
    queued: Record<string, unknown>;
  };
  // Old format fields
  transactions?: PendingTransaction[];
  total?: number;
  page?: number;
  limit?: number;
  totalPages?: number;
}

interface TransactionCardProps {
  transaction: PendingTransaction;
}

const TransactionCard: React.FC<TransactionCardProps> = ({ transaction }) => {
  // Use UTC to avoid hydration mismatch
  const formatDateUTC = (timestamp: number): string => {
    const d = new Date(timestamp * 1000);
    const day = d.getUTCDate().toString().padStart(2, '0');
    const month = (d.getUTCMonth() + 1).toString().padStart(2, '0');
    const year = d.getUTCFullYear();
    const hours = d.getUTCHours().toString().padStart(2, '0');
    const minutes = d.getUTCMinutes().toString().padStart(2, '0');
    const seconds = d.getUTCSeconds().toString().padStart(2, '0');
    return `${day}/${month}/${year}, ${hours}:${minutes}:${seconds}`;
  };
  const date = transaction.createdAt ? formatDateUTC(transaction.createdAt) : 'Pending';

  return (
    <div className="bg-gradient-to-r from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] rounded-xl p-6 shadow-lg hover:border-[#ffa729] transition-colors">
      <div className="flex flex-col gap-2">
        <div className="flex items-center justify-between">
          <Link href={`/pending/tx/${transaction.hash}`} className="text-[#ffa729] hover:text-[#ffb952] font-mono">
            {transaction.hash}
          </Link>
          <span className={`px-2 py-1 rounded text-sm ${
            transaction.status === 'pending' ? 'bg-yellow-500/20 text-yellow-500' :
            transaction.status === 'dropped' ? 'bg-red-500/20 text-red-500' :
            'bg-green-500/20 text-green-500'
          }`}>
            {transaction.status}
          </span>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <p className="text-gray-400 text-sm">From</p>
            <p className="text-white font-mono truncate">{transaction.from}</p>
          </div>
          <div>
            <p className="text-gray-400 text-sm">To</p>
            <p className="text-white font-mono truncate">{transaction.to}</p>
          </div>
          <div>
            <p className="text-gray-400 text-sm">Value</p>
            <p className="text-white">{formatAmount(transaction.value)[0]} QRL</p>
          </div>
          <div>
            <p className="text-gray-400 text-sm">Gas Price</p>
            <p className="text-white">{formatAmount(transaction.gasPrice)[0]} Shor</p>
          </div>
          <div>
            <p className="text-gray-400 text-sm">Time</p>
            <p className="text-white">{date}</p>
          </div>
        </div>
      </div>
    </div>
  );
};

const ITEMS_PER_PAGE = 10;

interface PendingListProps {
  initialData: PaginatedResponse;
  currentPage: number;
}

const fetchPendingTransactions = async (page: number): Promise<PaginatedResponse> => {
  console.log('Fetching pending transactions for page:', page);
  const response = await axios.get<PaginatedResponse>(`${config.handlerUrl}/pending-transactions`, {
    params: {
      page,
      limit: ITEMS_PER_PAGE
    }
  });
  
  console.log('Received response:', response.data);
  return response.data;
};

export default function PendingList({ initialData, currentPage }: PendingListProps): JSX.Element {
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [lastChecked, setLastChecked] = useState<Date | null>(null);
  const { data, isError, error, refetch, isFetching } = useQuery({
    queryKey: ['pending-transactions', currentPage],
    queryFn: () => fetchPendingTransactions(currentPage),
    initialData,
    refetchInterval: 5000,
  });

  const handleRefresh = async (): Promise<void> => {
    setIsRefreshing(true);
    await refetch();
    setLastChecked(new Date());
    setTimeout(() => setIsRefreshing(false), 500);
  };

  if (isError) {
    console.error('Error fetching pending transactions:', error);
    return (
      <div className="px-4 sm:px-6 lg:px-8">
        <div className="max-w-[1200px] mx-auto">
          <div className="bg-red-900/20 border border-red-500/50 rounded-xl p-6 shadow-lg">
            <h2 className="text-red-500 font-semibold mb-2">Error Loading Transactions</h2>
            <p className="text-gray-300">Failed to load pending transactions. Please try again later.</p>
          </div>
        </div>
      </div>
    );
  }

  // Convert the nested structure to a flat array
  const transactions: PendingTransaction[] = [];
  try {
    // Handle both old and new response formats
    if (data?.result?.pending) {
      // New format
      Object.entries(data.result.pending).forEach(([_address, nonceMap]) => {
        Object.entries(nonceMap).forEach(([_nonce, tx]) => {
          transactions.push(tx as PendingTransaction);
        });
      });
    } else if (Array.isArray(data?.transactions)) {
      // Old format
      transactions.push(...data.transactions);
    } else {
      console.warn('Unexpected response format:', data);
    }
  } catch (err) {
    console.error('Error processing transactions:', err);
    return (
      <div className="px-4 sm:px-6 lg:px-8">
        <div className="max-w-[1200px] mx-auto">
          <div className="bg-red-900/20 border border-red-500/50 rounded-xl p-6 shadow-lg">
            <h2 className="text-red-500 font-semibold mb-2">Error Processing Transactions</h2>
            <p className="text-gray-300">Failed to process transaction data. Please try again later.</p>
          </div>
        </div>
      </div>
    );
  }

  // Sort by createdAt descending
  transactions.sort((a, b) => (b.createdAt - a.createdAt));

  if (transactions.length === 0) {
    return (
      <div className="px-4 sm:px-6 lg:px-8">
        <div className="max-w-[1200px] mx-auto">
          <div className="bg-gradient-to-r from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] rounded-xl p-6 shadow-lg">
            <h2 className="text-[#ffa729] font-semibold text-lg mb-2">No Pending Transactions</h2>
            <p className="text-gray-300 mb-4">There are currently no pending transactions in the mempool.</p>
            {lastChecked && !isRefreshing && !isFetching && (
              <div className="mb-4 text-sm">
                <span className="text-green-400">✓ Confirmed empty at {lastChecked.toLocaleTimeString()}</span>
              </div>
            )}
            <button 
              onClick={handleRefresh}
              disabled={isRefreshing || isFetching}
              className={`px-6 py-2 bg-[#ffa729] hover:bg-[#ffb952] text-black font-medium rounded-lg transition-all
                         flex items-center gap-2 disabled:opacity-50 disabled:cursor-not-allowed
                         ${isRefreshing || isFetching ? 'animate-pulse' : ''}`}
            >
              {isRefreshing || isFetching ? (
                <>
                  <svg className="animate-spin h-4 w-4 text-black" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                    <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                    <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                  </svg>
                  Checking Mempool...
                </>
              ) : (
                <>
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                  </svg>
                  Check Again
                </>
              )}
            </button>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="px-4 sm:px-6 lg:px-8">
      <div className="max-w-[1200px] mx-auto space-y-4">
        {transactions.map((transaction) => (
          <TransactionCard key={transaction.hash} transaction={transaction} />
        ))}
        <div className="mt-4 text-center">
          <button 
            onClick={handleRefresh}
            disabled={isRefreshing || isFetching}
            className={`px-6 py-2 bg-[#ffa729] hover:bg-[#ffb952] text-black font-medium rounded-lg transition-all
                       flex items-center gap-2 mx-auto disabled:opacity-50 disabled:cursor-not-allowed
                       ${isRefreshing || isFetching ? 'animate-pulse' : ''}`}
          >
            {isRefreshing || isFetching ? (
              <>
                <svg className="animate-spin h-4 w-4 text-black" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                Checking Mempool...
              </>
            ) : (
              <>
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
                </svg>
                Check Again
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
