"use client";

import React, { useEffect, useState } from 'react';
import { useParams } from 'next/navigation';
import { TransactionDetails, TransactionResponse } from './types';

export default function TransactionPage() {
  const params = useParams();
  const [transaction, setTransaction] = useState<TransactionDetails | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchTransaction = async () => {
      if (!params?.query) {
        setError('Transaction hash is required');
        setLoading(false);
        return;
      }

      try {
        const response = await fetch(`/api/transaction/${params.query}`);
        if (!response.ok) {
          throw new Error('Failed to fetch transaction details');
        }
        const data = await response.json();
        if (data.error) {
          throw new Error(data.error);
        }
        // Transform the response data to match our TransactionDetails type
        const txData = data.response;
        setTransaction({
          hash: txData.hash || params.query,
          blockNumber: txData.blockNumber || '',
          from: txData.from || '',
          to: txData.to || '',
          value: txData.value || '0',
          timestamp: txData.timestamp || 0,
          status: txData.status || 'Unknown',
          gasUsed: txData.gasUsed,
          gasPrice: txData.gasPrice,
          nonce: txData.nonce
        });
      } catch (err) {
        setError(err instanceof Error ? err.message : 'An error occurred');
      } finally {
        setLoading(false);
      }
    };

    fetchTransaction();
  }, [params?.query]);

  if (loading) return null; // Using the loading.tsx file

  if (error) {
    return (
      <div className="p-4 max-w-3xl mx-auto">
        <div className="bg-red-900/50 border border-red-500 text-red-200 px-4 py-3 rounded relative" role="alert">
          <strong className="font-bold">Error: </strong>
          <span className="block sm:inline">{error}</span>
        </div>
      </div>
    );
  }

  if (!transaction) {
    return (
      <div className="p-4 max-w-3xl mx-auto">
        <div className="bg-yellow-900/50 border border-yellow-500 text-yellow-200 px-4 py-3 rounded relative" role="alert">
          <strong className="font-bold">Not Found: </strong>
          <span className="block sm:inline">Transaction not found</span>
        </div>
      </div>
    );
  }

  return (
    <div className="p-4 max-w-3xl mx-auto">
      <div className="bg-[#2d2d2d] shadow rounded-lg p-6">
        <div className="flex items-center mb-6">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6 text-[#ffa729] mr-2">
            <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
          </svg>
          <h1 className="text-2xl font-bold text-[#ffa729]">Transaction Details</h1>
        </div>
        
        <div className="space-y-4">
          <div className="border-b border-gray-700 pb-4">
            <h2 className="text-sm font-semibold text-[#ffa729] mb-1">Transaction Hash</h2>
            <p className="text-gray-300 break-all">{transaction.hash}</p>
          </div>

          <div className="border-b border-gray-700 pb-4">
            <h2 className="text-sm font-semibold text-[#ffa729] mb-1">Status</h2>
            <p className="text-gray-300">{transaction.status}</p>
          </div>

          <div className="border-b border-gray-700 pb-4">
            <h2 className="text-sm font-semibold text-[#ffa729] mb-1">Block</h2>
            <p className="text-gray-300">{transaction.blockNumber}</p>
          </div>

          <div className="border-b border-gray-700 pb-4">
            <h2 className="text-sm font-semibold text-[#ffa729] mb-1">From</h2>
            <p className="text-gray-300 break-all">{transaction.from}</p>
          </div>

          <div className="border-b border-gray-700 pb-4">
            <h2 className="text-sm font-semibold text-[#ffa729] mb-1">To</h2>
            <p className="text-gray-300 break-all">{transaction.to}</p>
          </div>

          <div className="border-b border-gray-700 pb-4">
            <h2 className="text-sm font-semibold text-[#ffa729] mb-1">Value</h2>
            <p className="text-gray-300">{transaction.value} QUANTA</p>
          </div>

          {transaction.gasUsed && (
            <div className="border-b border-gray-700 pb-4">
              <h2 className="text-sm font-semibold text-[#ffa729] mb-1">Gas Used</h2>
              <p className="text-gray-300">{transaction.gasUsed}</p>
            </div>
          )}

          {transaction.gasPrice && (
            <div className="border-b border-gray-700 pb-4">
              <h2 className="text-sm font-semibold text-[#ffa729] mb-1">Gas Price</h2>
              <p className="text-gray-300">{transaction.gasPrice}</p>
            </div>
          )}

          {transaction.timestamp && (
            <div>
              <h2 className="text-sm font-semibold text-[#ffa729] mb-1">Timestamp</h2>
              <p className="text-gray-300">
                {new Date(transaction.timestamp * 1000).toLocaleString()}
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
