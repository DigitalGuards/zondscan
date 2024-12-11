"use client";

import axios from 'axios';
import React, { useEffect } from 'react';
import config from '../../../config';
import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import { formatAmount } from '../../lib/helpers';

interface PendingTransaction {
  hash: string;
  from: string;
  to: string;
  value: string;
  gasPrice: string;
  timestamp?: number;
}

interface PaginatedResponse {
  transactions: PendingTransaction[];
  total: number;
  page: number;
  limit: number;
  totalPages: number;
}

interface TransactionCardProps {
  transaction: PendingTransaction;
}

const TransactionCard: React.FC<TransactionCardProps> = ({ transaction }) => {
  const date = transaction.timestamp 
    ? new Date(transaction.timestamp * 1000).toLocaleString('en-GB', {
        day: '2-digit',
        month: '2-digit',
        year: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit'
      })
    : 'Pending';

  return (
    <div className="bg-gradient-to-r from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] rounded-xl p-4 mb-4 hover:border-[#ffa729] transition-colors">
      <div className="flex flex-col gap-2">
        <div className="flex items-center justify-between">
          <Link href={`/pending/tx/${transaction.hash}`} className="text-[#ffa729] hover:text-[#ffb952] font-mono">
            {transaction.hash}
          </Link>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div>
            <p className="text-gray-400">From</p>
            <Link href={`/address/${transaction.from}`} className="text-[#ffa729] hover:text-[#ffb952] font-mono break-all">
              {transaction.from}
            </Link>
          </div>
          <div>
            <p className="text-gray-400">To</p>
            <Link href={`/address/${transaction.to}`} className="text-[#ffa729] hover:text-[#ffb952] font-mono break-all">
              {transaction.to}
            </Link>
          </div>
        </div>
        <div className="flex justify-between items-center mt-2">
          <div>
            <span className="text-gray-400">Value: </span>
            <span className="text-white font-mono">{formatAmount(transaction.value)}</span>
          </div>
          <div>
            <span className="text-gray-400">Gas Price: </span>
            <span className="text-white font-mono">{formatAmount(transaction.gasPrice)}</span>
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
  const response = await axios.get<PaginatedResponse>(`${config.handlerUrl}/pending-transactions`, {
    params: {
      page,
      limit: ITEMS_PER_PAGE
    }
  });
  
  return response.data;
};

export default function PendingList({ initialData, currentPage }: PendingListProps) {
  const { data, isError, error, refetch } = useQuery({
    queryKey: ['pending-transactions', currentPage],
    queryFn: () => fetchPendingTransactions(currentPage),
    refetchInterval: 5000,
    retry: 2,
    refetchOnMount: false,
    initialData
  });

  useEffect(() => {
    if (currentPage !== data?.page) {
      refetch();
    }
  }, [refetch, currentPage, data?.page]);

  if (isError) {
    return (
      <div className="px-4 lg:px-6">
        <h1 className="text-2xl font-bold mb-6 text-[#ffa729]">Mempool</h1>
        <div className="bg-red-900/50 border border-red-500 text-red-200 px-6 py-4 rounded-xl">
          <p className="font-bold">Error:</p>
          <p>{error instanceof Error ? error.message : 'Failed to load pending transactions'}</p>
        </div>
      </div>
    );
  }

  const startIndex = (currentPage - 1) * ITEMS_PER_PAGE + 1;
  const endIndex = Math.min(currentPage * ITEMS_PER_PAGE, data?.total || 0);

  return (
    <div className="px-4 lg:px-6">
      <h1 className="text-2xl font-bold mb-6 text-[#ffa729]">Mempool</h1>
      {!data?.transactions?.length ? (
        <div className="bg-gradient-to-r from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] rounded-xl p-8 text-center">
          <div className="inline-block p-4 bg-yellow-500/20 rounded-full mb-4">
            <svg className="w-8 h-8 text-yellow-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"></path>
            </svg>
          </div>
          <h2 className="text-xl font-semibold text-[#ffa729] mb-2">No Pending Transactions</h2>
          <p className="text-gray-400 max-w-lg mx-auto">
            Showing unconfirmed transactions waiting to be included in a block. Updates every 5 seconds.
          </p>
        </div>
      ) : (
        <div className="mb-8">
          <div className="bg-gradient-to-r from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] rounded-xl p-4 mb-6">
            <p className="text-gray-300">
              <span className="text-[#ffa729] font-semibold">{data?.total}</span> unconfirmed {data?.total === 1 ? 'transaction' : 'transactions'} waiting to be included in a block. Updates every 5 seconds.
              {data?.totalPages > 1 && ` (Showing ${startIndex}-${endIndex} of ${data?.total})`}
            </p>
          </div>
          {data?.transactions?.map(transaction => (
            <TransactionCard 
              key={transaction.hash} 
              transaction={transaction}
            />
          ))}
          
          {/* Pagination */}
          {data?.totalPages > 1 && (
            <div className="flex justify-center gap-2 mt-6">
              {currentPage > 1 && (
                <Link
                  href={`/pending/${currentPage - 1}`}
                  className="px-4 py-2 rounded-lg bg-[#2d2d2d] hover:bg-[#3d3d3d] text-gray-300 transition-colors"
                >
                  Previous
                </Link>
              )}
              <span className="px-4 py-2 text-gray-400">
                Page {currentPage} of {data?.totalPages}
              </span>
              {currentPage < data?.totalPages && (
                <Link
                  href={`/pending/${currentPage + 1}`}
                  className="px-4 py-2 rounded-lg bg-[#2d2d2d] hover:bg-[#3d3d3d] text-gray-300 transition-colors"
                >
                  Next
                </Link>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  );
}
