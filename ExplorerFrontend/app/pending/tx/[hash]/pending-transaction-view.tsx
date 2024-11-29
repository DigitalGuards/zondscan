'use client';

import React from 'react';
import { PendingTransaction } from '../types';
import { formatAmount } from '../../../lib/helpers';

interface PendingTransactionViewProps {
  pendingTx: PendingTransaction;
}

export default function PendingTransactionView({ pendingTx }: PendingTransactionViewProps): JSX.Element {
  // Format values using our helper function
  const [value, valueUnit] = formatAmount(pendingTx.value);
  const [gasPrice, gasPriceUnit] = formatAmount(pendingTx.gasPrice);
  const [maxFeePerGas, maxFeeUnit] = pendingTx.maxFeePerGas ? formatAmount(pendingTx.maxFeePerGas) : ['0', 'QRL'];
  const [maxPriorityFeePerGas, maxPriorityFeeUnit] = pendingTx.maxPriorityFeePerGas ? formatAmount(pendingTx.maxPriorityFeePerGas) : ['0', 'QRL'];

  return (
    <div className="container mx-auto px-4">
      <h1 className="text-2xl font-bold mb-6 text-[#ffa729]">Pending Transaction Details</h1>
      
      <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] rounded-xl p-6 mb-8 shadow-xl">
        <div className="space-y-4">
          <div className="flex flex-col space-y-1">
            <span className="text-gray-400">Transaction Hash</span>
            <span className="text-gray-200 break-all font-mono">{pendingTx.hash}</span>
          </div>

          <div className="flex flex-col space-y-1">
            <span className="text-gray-400">Status</span>
            <div className="bg-yellow-500/20 text-yellow-500 px-3 py-1 rounded-lg text-sm inline-block">
              Pending
            </div>
          </div>

          <div className="flex flex-col space-y-1">
            <span className="text-gray-400">From</span>
            <span className="text-gray-200 break-all font-mono">{pendingTx.from}</span>
          </div>

          {pendingTx.to && (
            <div className="flex flex-col space-y-1">
              <span className="text-gray-400">To</span>
              <span className="text-gray-200 break-all font-mono">{pendingTx.to}</span>
            </div>
          )}

          <div className="flex flex-col space-y-1">
            <span className="text-gray-400">Value</span>
            <span className="text-gray-200">
              {value} {valueUnit}
            </span>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div className="flex flex-col space-y-1">
              <span className="text-gray-400">Gas Limit</span>
              <span className="text-gray-200">{parseInt(pendingTx.gas, 16).toLocaleString()}</span>
            </div>

            <div className="flex flex-col space-y-1">
              <span className="text-gray-400">Gas Price</span>
              <span className="text-gray-200">
                {gasPrice} {gasPriceUnit}
              </span>
            </div>

            {pendingTx.maxFeePerGas && (
              <div className="flex flex-col space-y-1">
                <span className="text-gray-400">Max Fee Per Gas</span>
                <span className="text-gray-200">
                  {maxFeePerGas} {maxFeeUnit}
                </span>
              </div>
            )}

            {pendingTx.maxPriorityFeePerGas && (
              <div className="flex flex-col space-y-1">
                <span className="text-gray-400">Max Priority Fee Per Gas</span>
                <span className="text-gray-200">
                  {maxPriorityFeePerGas} {maxPriorityFeeUnit}
                </span>
              </div>
            )}
          </div>

          <div className="flex flex-col space-y-1">
            <span className="text-gray-400">Nonce</span>
            <span className="text-gray-200">{parseInt(pendingTx.nonce, 16)}</span>
          </div>

          {pendingTx.input && pendingTx.input !== '0x' && (
            <div className="flex flex-col space-y-1">
              <span className="text-gray-400">Input Data</span>
              <span className="text-gray-200 break-all font-mono">{pendingTx.input}</span>
            </div>
          )}
        </div>
      </div>

      <div className="bg-yellow-900/20 border border-yellow-500/50 rounded-xl p-6 shadow-lg">
        <h2 className="text-yellow-500 font-semibold mb-2">About Pending Transactions</h2>
        <p className="text-gray-300">
          This transaction is currently in the mempool waiting to be included in a block. 
          The details shown here may change when the transaction is mined, and there is no 
          guarantee that this transaction will be included in a block.
        </p>
        <p className="text-gray-300 mt-2">
          This page will automatically refresh every 10 seconds to check for updates.
        </p>
      </div>
      
      {/* Add auto-refresh meta tag */}
      <meta httpEquiv="refresh" content="10" />
    </div>
  );
}
