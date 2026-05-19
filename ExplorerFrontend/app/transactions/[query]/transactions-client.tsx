'use client';

import React from 'react';
import TransactionsList from './TransactionsList';
import type { Transaction } from '@/app/types';
import config from '../../../config';

interface TransactionsResponse {
  txs: Transaction[];
  total: number;
  latestBlock?: number;
}

interface TransactionsClientProps {
  initialData: TransactionsResponse;
  pageNumber: string;
}

export default function TransactionsClient({ initialData, pageNumber }: TransactionsClientProps): JSX.Element {
  const [data, setData] = React.useState<TransactionsResponse>(initialData);
  const [error, setError] = React.useState<string | null>(null);
  const [isLoading, setIsLoading] = React.useState<boolean>(false);
  // Resync local `data` whenever the parent pushes a new initialData /
  // pageNumber. React's "adjusting state on prop change" pattern: detect
  // mid-render and update, avoiding set-state-in-effect.
  const [prevKey, setPrevKey] = React.useState<string>(`${pageNumber}|${initialData.total}`);
  const nextKey = `${pageNumber}|${initialData.total}`;
  if (prevKey !== nextKey) {
    setPrevKey(nextKey);
    setData(initialData);
  }

  const refetchData = async (): Promise<void> => {
    try {
      setIsLoading(true);
      const pageNum = parseInt(pageNumber, 10) || 1;
      const timestamp = Date.now();
      const response = await fetch(`${config.handlerUrl}/txs?page=${pageNum}&limit=10&_t=${timestamp}`, {
        method: 'GET',
        headers: {
          'Accept': 'application/json',
          'Cache-Control': 'no-cache'
        },
        cache: 'no-store'
      });

      if (!response.ok) {
        throw new Error(`HTTP error! status: ${response.status}`);
      }

      const newData = await response.json();
      setData(newData);
      setError(null);
    } catch (err) {
      console.error('Error refetching transactions:', err);
      setError('Failed to update transactions. Please try again.');
    } finally {
      setIsLoading(false);
    }
  };

  if (error) {
    return (
      <div className="p-4 sm:p-8 max-w-6xl mx-auto">
        <h1 className="text-xl sm:text-2xl font-bold mb-4 text-[#ffa729]">Transactions</h1>
        <div className="bg-red-900/50 border border-red-500 text-red-200 px-4 py-3 rounded-xl">
          <p className="font-bold">Error:</p>
          <p className="text-sm">{error}</p>
        </div>
        <button
          onClick={refetchData}
          className="mt-4 px-4 py-2 bg-[#ffa729] text-black rounded-lg hover:bg-[#ffb85c] transition-colors"
        >
          Try Again
        </button>
      </div>
    );
  }

  if (isLoading) {
    return (
      <div className="flex justify-center items-center p-8">
        <div className="animate-spin rounded-full h-8 w-8 border-t-2 border-b-2 border-[#ffa729]"></div>
        <span className="ml-2 text-gray-400">Refreshing transactions...</span>
      </div>
    );
  }

  return (
    <TransactionsList
      initialData={data}
      currentPage={parseInt(pageNumber)}
    />
  );
}
