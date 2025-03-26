'use client';

import React, { useEffect } from 'react';
import TransactionsList from './TransactionsList';
import { Transaction } from './types';
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

export default function TransactionsClient({ initialData, pageNumber }: TransactionsClientProps) {
  const [data, setData] = React.useState<TransactionsResponse>(initialData);
  const [error, setError] = React.useState<string | null>(null);
  const [isLoading, setIsLoading] = React.useState<boolean>(false);

  // Re-fetch data when page changes
  useEffect(() => {
    console.log(`Client received data with ${initialData.txs?.length || 0} transactions. Total: ${initialData.total || 0}`);
    setData(initialData);
  }, [initialData, pageNumber]);

  // Optional: Function to manually refetch data
  const refetchData = async () => {
    try {
      setIsLoading(true);
      const pageNum = parseInt(pageNumber, 10) || 1;
      const timestamp = Date.now();
      const response = await fetch(`${config.handlerUrl}/txs?page=${pageNum}&_t=${timestamp}`, {
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
      console.log(`Refetched ${newData.txs?.length || 0} transactions. Total: ${newData.total || 0}`);
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
      <div role="alert" className="p-4">
        <h1 className="text-xl font-bold mb-2">Error</h1>
        <p>{error}</p>
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
        <span className="ml-2">Refreshing transactions...</span>
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
