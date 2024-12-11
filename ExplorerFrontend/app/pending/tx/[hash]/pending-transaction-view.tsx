'use client';

import React from 'react';
import Link from 'next/link';
import { PendingTransaction } from '../types';
import { formatAmount } from '../../../lib/helpers';

interface PendingTransactionViewProps {
  pendingTx: PendingTransaction;
}

export default function PendingTransactionView({ pendingTx }: PendingTransactionViewProps): JSX.Element {
  return (
    <div className="container mx-auto px-4">
      <h1 className="text-2xl font-bold mb-6 text-[#ffa729]">Pending Transaction Details</h1>
      
      <div className="bg-gradient-to-r from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] rounded-xl p-6 mb-6">
        <div className="grid grid-cols-1 gap-6">
          <div>
            <h2 className="text-lg font-semibold text-[#ffa729] mb-2">Transaction Hash</h2>
            <p className="font-mono text-gray-300 break-all">{pendingTx.hash}</p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div>
              <h2 className="text-lg font-semibold text-[#ffa729] mb-2">From</h2>
              <Link href={`/address/${pendingTx.from}`} className="font-mono text-gray-300 hover:text-[#ffa729] break-all">
                {pendingTx.from}
              </Link>
            </div>
            <div>
              <h2 className="text-lg font-semibold text-[#ffa729] mb-2">To</h2>
              {pendingTx.to ? (
                <Link href={`/address/${pendingTx.to}`} className="font-mono text-gray-300 hover:text-[#ffa729] break-all">
                  {pendingTx.to}
                </Link>
              ) : (
                <span className="text-gray-500">Contract Creation</span>
              )}
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div>
              <h2 className="text-lg font-semibold text-[#ffa729] mb-2">Value</h2>
              <p className="font-mono text-gray-300">{formatAmount(pendingTx.value)}</p>
            </div>
            <div>
              <h2 className="text-lg font-semibold text-[#ffa729] mb-2">Gas Price</h2>
              <p className="font-mono text-gray-300">{formatAmount(pendingTx.gasPrice)}</p>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
            <div>
              <h2 className="text-lg font-semibold text-[#ffa729] mb-2">Gas Limit</h2>
              <p className="font-mono text-gray-300">{pendingTx.gas}</p>
            </div>
            <div>
              <h2 className="text-lg font-semibold text-[#ffa729] mb-2">Nonce</h2>
              <p className="font-mono text-gray-300">{pendingTx.nonce}</p>
            </div>
          </div>

          {(pendingTx.maxFeePerGas || pendingTx.maxPriorityFeePerGas) && (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {pendingTx.maxFeePerGas && (
                <div>
                  <h2 className="text-lg font-semibold text-[#ffa729] mb-2">Max Fee Per Gas</h2>
                  <p className="font-mono text-gray-300">{formatAmount(pendingTx.maxFeePerGas)}</p>
                </div>
              )}
              {pendingTx.maxPriorityFeePerGas && (
                <div>
                  <h2 className="text-lg font-semibold text-[#ffa729] mb-2">Max Priority Fee Per Gas</h2>
                  <p className="font-mono text-gray-300">{formatAmount(pendingTx.maxPriorityFeePerGas)}</p>
                </div>
              )}
            </div>
          )}

          <div>
            <h2 className="text-lg font-semibold text-[#ffa729] mb-2">Input Data</h2>
            <div className="bg-[#1a1a1a] rounded-lg p-4 overflow-x-auto">
              <pre className="font-mono text-gray-300 whitespace-pre-wrap break-all">
                {pendingTx.input === '0x' ? '(none)' : pendingTx.input}
              </pre>
            </div>
          </div>
        </div>
      </div>

      <div className="bg-yellow-500/20 border border-yellow-500/30 rounded-xl p-4 mb-6">
        <p className="text-yellow-200">
          <span className="font-semibold">Note:</span> This transaction is pending and waiting to be included in a block.
          The status updates every few seconds.
        </p>
      </div>
    </div>
  );
}
