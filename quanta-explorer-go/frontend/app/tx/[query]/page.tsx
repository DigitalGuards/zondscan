"use client";

import React, { useEffect, useState } from 'react';
import { useParams } from 'next/navigation';
import { TransactionDetails, TransactionResponse } from './types';

const formatTimestamp = (timestamp: number): string => {
  if (!timestamp) return 'Unknown';
  const date = new Date(timestamp * 1000);
  if (date.getFullYear() === 1970) return 'Pending';
  
  return new Intl.DateTimeFormat('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    timeZoneName: 'short'
  }).format(date);
};

const getStatusColor = (status: string): string => {
  switch (status.toLowerCase()) {
    case 'success':
      return 'bg-green-500';
    case 'failed':
      return 'bg-red-500';
    case 'pending':
      return 'bg-yellow-500';
    default:
      return 'bg-blue-500';
  }
};

const AddressLink = ({ address }: { address: string }) => (
  <a 
    href={`/address/${address}`}
    className="text-gray-300 hover:text-[#ffa729] break-all font-mono 
              transition-colors duration-300 group relative"
  >
    {address}
    <div className="absolute -inset-2 rounded-lg bg-[#3d3d3d] opacity-0 
                  group-hover:opacity-10 transition-opacity duration-300" />
  </a>
);

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

  if (loading) return null;

  if (error) {
    return (
      <div className="py-8">
        <div className="bg-red-900/50 border border-red-500 text-red-200 px-6 py-4 rounded-xl" role="alert">
          <strong className="font-bold">Error: </strong>
          <span className="block sm:inline">{error}</span>
        </div>
      </div>
    );
  }

  if (!transaction) {
    return (
      <div className="py-8">
        <div className="bg-yellow-900/50 border border-yellow-500 text-yellow-200 px-6 py-4 rounded-xl" role="alert">
          <strong className="font-bold">Not Found: </strong>
          <span className="block sm:inline">Transaction not found</span>
        </div>
      </div>
    );
  }

  const statusColor = getStatusColor(transaction.status);

  return (
    <div className="py-8">
      <div className="relative overflow-hidden rounded-2xl 
                    bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                    border border-[#3d3d3d] shadow-xl">
        <div className="p-8">
          {/* Header */}
          <div className="flex items-center justify-between mb-8 pb-6 border-b border-gray-700">
            <div className="flex items-center">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-8 h-8 text-[#ffa729] mr-3">
                <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
              </svg>
              <h1 className="text-2xl font-bold text-[#ffa729]">Transaction Details</h1>
            </div>
            <div className={`px-4 py-2 rounded-xl ${statusColor} bg-opacity-20 border border-opacity-20 
                           flex items-center ${statusColor.replace('bg-', 'border-')}`}>
              <div className={`w-2 h-2 rounded-full ${statusColor} mr-2`}></div>
              <span className="text-sm font-medium">{transaction.status}</span>
            </div>
          </div>
          
          {/* Content Grid */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
            {/* Left Column */}
            <div className="space-y-6">
              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">Transaction Hash</h2>
                <p className="text-gray-300 break-all font-mono">{transaction.hash}</p>
              </div>

              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">Block</h2>
                <a 
                  href={`/block/${transaction.blockNumber}`}
                  className="text-gray-300 hover:text-[#ffa729] transition-colors duration-300"
                >
                  {transaction.blockNumber || 'Pending'}
                </a>
              </div>

              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">Timestamp</h2>
                <p className="text-gray-300">
                  {formatTimestamp(transaction.timestamp)}
                </p>
              </div>
            </div>

            {/* Right Column */}
            <div className="space-y-6">
              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">From</h2>
                <AddressLink address={transaction.from} />
              </div>

              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">To</h2>
                <AddressLink address={transaction.to} />
              </div>

              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">Value</h2>
                <p className="text-2xl font-semibold text-[#ffa729]">
                  {transaction.value}
                  <span className="text-sm text-gray-400 ml-2">QUANTA</span>
                </p>
              </div>

              {(transaction.gasUsed || transaction.gasPrice) && (
                <div className="space-y-4 pt-4 border-t border-gray-700">
                  {transaction.gasUsed && (
                    <div>
                      <h2 className="text-sm font-semibold text-gray-400 mb-2">Gas Used</h2>
                      <p className="text-gray-300">{transaction.gasUsed}</p>
                    </div>
                  )}

                  {transaction.gasPrice && (
                    <div>
                      <h2 className="text-sm font-semibold text-gray-400 mb-2">Gas Price</h2>
                      <p className="text-gray-300">{transaction.gasPrice}</p>
                    </div>
                  )}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
