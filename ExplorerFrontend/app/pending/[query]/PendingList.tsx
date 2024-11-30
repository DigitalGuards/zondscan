"use client";

import React, { useEffect } from 'react';
import axios from 'axios';
import config from '../../../config';
import { useQuery } from '@tanstack/react-query';
import Link from 'next/link';
import { PendingTransaction, PendingTransactionsResponse } from '../tx/types';
import { formatAmount } from '../../lib/helpers';

interface PendingTransactionDisplay {
  hash: string;
  from: string;
  to: string | null;
  value: string;
  gasPrice: string;
  timestamp: number;
}

const transformPendingData = (data: PendingTransactionsResponse): PendingTransactionDisplay[] => {
  const transformedTxs: PendingTransactionDisplay[] = [];
  
  try {
    if (data?.pending) {
      Object.entries(data.pending).forEach(([address, nonceTxs]) => {
        if (typeof nonceTxs === 'object') {
          Object.entries(nonceTxs).forEach(([nonce, tx]) => {
            if (tx && typeof tx === 'object' && 'hash' in tx) {
              try {
                const transaction: PendingTransactionDisplay = {
                  hash: tx.hash,
                  from: tx.from,
                  to: tx.to || null,
                  value: tx.value,
                  gasPrice: tx.gasPrice,
                  timestamp: Math.floor(Date.now() / 1000)
                };
                transformedTxs.push(transaction);
              } catch (error) {
                console.error('Error transforming transaction:', error);
              }
            }
          });
        }
      });
    }
  } catch (error) {
    console.error('Error in transformPendingData:', error);
  }
  
  return transformedTxs;
};

const fetchPendingTransactions = async () => {
  const response = await axios.get(`${config.handlerUrl}/pending-transactions`);
  const transformedTxs = transformPendingData(response.data);
  
  return {
    txs: transformedTxs,
    total: transformedTxs.length
  };
};

interface PendingListProps {
  initialData: {
    txs: PendingTransactionDisplay[];
    total: number;
  };
  currentPage: number;
}

function TransactionCard({ transaction }: { transaction: PendingTransactionDisplay }) {
  const date = new Date(transaction.timestamp * 1000).toLocaleString('en-GB', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit'
  });

  const [formattedValue, valueUnit] = formatAmount(transaction.value);
  const [formattedGasPrice, gasPriceUnit] = formatAmount(transaction.gasPrice);

  return (
    <Link 
      href={`/pending/tx/${transaction.hash}`}
      className='relative overflow-hidden rounded-2xl 
                bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                border border-[#3d3d3d] shadow-xl
                hover:border-[#ffa729] transition-all duration-300
                group mb-4 block'
    >
      <div className="flex items-center p-6">
        <div className="flex flex-col items-center w-48">
          <div className="mb-2">
            <div className="bg-yellow-500/20 text-yellow-500 px-3 py-1 rounded-lg text-sm">
              Pending
            </div>
          </div>
          <div className="text-center">
            <p className="text-lg font-semibold text-[#ffa729] mb-1">Transfer</p>
            <p className="text-sm text-gray-400">{date}</p>
          </div>
        </div>

        <div className="flex-1 px-8 border-l border-r border-[#3d3d3d]">
          <div className="mb-2">
            <p className="text-sm font-medium text-gray-400 mb-1">Transaction Hash</p>
            <p className="text-gray-300 hover:text-[#ffa729] transition-colors break-all font-mono">
              {transaction.hash}
            </p>
          </div>
          <div className="mt-4">
            <p className="text-sm font-medium text-gray-400 mb-1">From</p>
            <p className="text-gray-300 font-mono truncate">
              {transaction.from}
            </p>
            {transaction.to && (
              <div className="mt-2">
                <p className="text-sm font-medium text-gray-400 mb-1">To</p>
                <p className="text-gray-300 font-mono truncate">
                  {transaction.to}
                </p>
              </div>
            )}
          </div>
        </div>

        <div className="w-48 text-right">
          <p className="text-sm font-medium text-gray-400 mb-2">Amount</p>
          <p className="text-2xl font-semibold text-[#ffa729]">
            {formattedValue}
            <span className="text-sm text-gray-400 ml-2">{valueUnit}</span>
          </p>
          <p className="text-sm text-gray-400 mt-2">
            Gas Price: {formattedGasPrice} {gasPriceUnit}
          </p>
        </div>
      </div>
    </Link>
  );
}

export default function PendingList({ initialData, currentPage }: PendingListProps) {
  const { data, isError, error, refetch } = useQuery({
    queryKey: ['pending-transactions'],
    queryFn: fetchPendingTransactions,
    refetchInterval: 5000,
    retry: 2,
    refetchOnMount: true,
    initialData
  });

  useEffect(() => {
    refetch();
  }, [refetch]);

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

  return (
    <div className="px-4 lg:px-6">
      <h1 className="text-2xl font-bold mb-6 text-[#ffa729]">Mempool</h1>
      {!data || data.txs.length === 0 ? (
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
              <span className="text-[#ffa729] font-semibold">{data.txs.length}</span> unconfirmed {data.txs.length === 1 ? 'transaction' : 'transactions'} waiting to be included in a block. Updates every 5 seconds.
            </p>
          </div>
          {data.txs.map(transaction => (
            <TransactionCard 
              key={transaction.hash} 
              transaction={transaction}
            />
          ))}
        </div>
      )}
    </div>
  );
}
