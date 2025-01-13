'use client';

import React from 'react';
import { PendingTransaction } from '../types';
import { formatAmount } from '../../../lib/helpers';

interface PendingTransactionViewProps {
  pendingTx: PendingTransaction;
}

export default function PendingTransactionView({ pendingTx }: PendingTransactionViewProps): JSX.Element {
  const [formattedValue, unit] = formatAmount(pendingTx.value);
  const [formattedGasPrice] = formatAmount(pendingTx.gasPrice);

  return (
    <div className="container mx-auto px-4">
      <div className="bg-[#1f1f1f] rounded-xl p-6 shadow-lg mt-6 border border-[#3d3d3d]">
        <div className="flex items-center justify-between mb-6">
          <h2 className="text-2xl font-bold text-white">Pending Transaction</h2>
          <span className="px-3 py-1 rounded-lg bg-yellow-500/20 text-yellow-500">
            Pending
          </span>
        </div>

        <div className="space-y-4">
          <div>
            <h3 className="text-gray-400 mb-1">Transaction Hash</h3>
            <p className="font-mono text-white break-all">{pendingTx.hash}</p>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <h3 className="text-gray-400 mb-1">From</h3>
              <p className="font-mono text-white break-all">{pendingTx.from}</p>
            </div>
            <div>
              <h3 className="text-gray-400 mb-1">To</h3>
              <p className="font-mono text-white break-all">{pendingTx.to || 'Contract Creation'}</p>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <h3 className="text-gray-400 mb-1">Value</h3>
              <p className="text-white">
                {formattedValue} {unit}
              </p>
            </div>
            <div>
              <h3 className="text-gray-400 mb-1">Gas Price</h3>
              <p className="text-white">{formattedGasPrice} Gwei</p>
            </div>
          </div>

          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            <div>
              <h3 className="text-gray-400 mb-1">Gas Limit</h3>
              <p className="text-white">{pendingTx.gas}</p>
            </div>
            <div>
              <h3 className="text-gray-400 mb-1">Nonce</h3>
              <p className="text-white">{pendingTx.nonce}</p>
            </div>
          </div>

          {pendingTx.input && pendingTx.input !== '0x' && (
            <div>
              <h3 className="text-gray-400 mb-1">Input Data</h3>
              <div className="bg-[#2d2d2d] p-4 rounded-lg">
                <p className="font-mono text-white break-all">{pendingTx.input}</p>
              </div>
            </div>
          )}
        </div>

        <div className="mt-6 pt-6 border-t border-[#3d3d3d]">
          <p className="text-gray-400 text-sm">
            This transaction is currently pending in the mempool. The page will automatically 
            refresh when the transaction is mined or if its status changes.
          </p>
        </div>
      </div>
    </div>
  );
}
