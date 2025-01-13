"use client";

import axios from 'axios';
import React, { useEffect } from 'react';
import config from '../../../config';
import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import { formatAmount } from '../../lib/helpers';
import { PendingTransaction } from '../tx/types';

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
  const date = transaction.createdAt 
    ? new Date(transaction.createdAt * 1000).toLocaleString('en-GB', {
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

export default function PendingList({ initialData, currentPage }: PendingListProps) {
  const { data, isError, error, refetch } = useQuery({
    queryKey: ['pending-transactions', currentPage],
    queryFn: () => fetchPendingTransactions(currentPage),
    initialData,
    refetchInterval: 5000,
  });

  if (isError) {
    console.error('Error fetching pending transactions:', error);
    return (
      <div className="bg-red-900/20 border border-red-500/50 rounded-xl p-6 shadow-lg mt-6">
        <h2 className="text-red-500 font-semibold mb-2">Error Loading Transactions</h2>
        <p className="text-gray-300">Failed to load pending transactions. Please try again later.</p>
      </div>
    );
  }

  // Convert the nested structure to a flat array
  const transactions: PendingTransaction[] = [];
  try {
    // Handle both old and new response formats
    if (data?.result?.pending) {
      // New format
      Object.entries(data.result.pending).forEach(([address, nonceMap]) => {
        Object.entries(nonceMap).forEach(([nonce, tx]) => {
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
      <div className="bg-red-900/20 border border-red-500/50 rounded-xl p-6 shadow-lg mt-6">
        <h2 className="text-red-500 font-semibold mb-2">Error Processing Transactions</h2>
        <p className="text-gray-300">Failed to process transaction data. Please try again later.</p>
      </div>
    );
  }

  // Sort by createdAt descending
  transactions.sort((a, b) => (b.createdAt - a.createdAt));

  if (transactions.length === 0) {
    return (
      <div className="bg-[#1f1f1f] border border-[#3d3d3d] rounded-xl p-6 shadow-lg mt-6">
        <h2 className="text-gray-300 font-semibold mb-2">No Pending Transactions</h2>
        <p className="text-gray-400">There are currently no pending transactions in the mempool.</p>
        <button 
          onClick={() => refetch()} 
          className="mt-4 px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
        >
          Refresh Transactions
        </button>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {transactions.map((transaction) => (
        <TransactionCard key={transaction.hash} transaction={transaction} />
      ))}
      <div className="mt-4 text-center">
        <button 
          onClick={() => refetch()} 
          className="px-4 py-2 bg-blue-600 hover:bg-blue-700 text-white rounded-lg transition-colors"
        >
          Refresh Transactions
        </button>
      </div>
    </div>
  );
}
