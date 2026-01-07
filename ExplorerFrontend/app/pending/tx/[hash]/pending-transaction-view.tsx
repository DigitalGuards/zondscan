'use client';

import React, { useMemo } from 'react';
import type { PendingTransaction } from '@/app/types';
import { formatAmount, decodeTokenTransferInput, formatTokenAmount } from '../../../lib/helpers';

interface PendingTransactionViewProps {
  pendingTx: PendingTransaction;
}

export default function PendingTransactionView({ pendingTx }: PendingTransactionViewProps): JSX.Element {
  const [formattedValue, unit] = formatAmount(pendingTx.value);
  const [formattedGasPrice] = formatAmount(pendingTx.gasPrice);

  // Decode token transfer from input data
  const decodedTransfer = useMemo(() => {
    return decodeTokenTransferInput(pendingTx.input);
  }, [pendingTx.input]);

  // Check if this is a token transfer (has decoded transfer data)
  const isTokenTransfer = decodedTransfer !== null;

  return (
    <div className="container mx-auto px-4">
      <div className="bg-[#1f1f1f] rounded-xl p-6 shadow-lg mt-6 border border-[#3d3d3d]">
        <div className="flex items-center justify-between mb-6">
          <div className="flex items-center gap-3">
            <h2 className="text-2xl font-bold text-white">Pending Transaction</h2>
            {isTokenTransfer && (
              <span className="px-2 py-0.5 rounded bg-[#ffa729]/20 text-[#ffa729] text-xs font-medium">
                Token Transfer
              </span>
            )}
          </div>
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
              <a
                href={`/address/${pendingTx.from}`}
                className="font-mono text-white hover:text-[#ffa729] break-all transition-colors"
              >
                {pendingTx.from}
              </a>
            </div>
            <div>
              <h3 className="text-gray-400 mb-1">To {isTokenTransfer && <span className="text-xs text-gray-500">(Contract)</span>}</h3>
              <a
                href={`/address/${pendingTx.to}`}
                className="font-mono text-white hover:text-[#ffa729] break-all transition-colors"
              >
                {pendingTx.to || 'Contract Creation'}
              </a>
            </div>
          </div>

          {/* Token Transfer Details */}
          {isTokenTransfer && decodedTransfer && (
            <div className="bg-[#ffa729]/10 border border-[#ffa729]/30 rounded-xl p-4">
              <div className="flex items-center gap-2 mb-3">
                <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5 text-[#ffa729]">
                  <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
                </svg>
                <h3 className="text-[#ffa729] font-semibold">Token Transfer (Pending)</h3>
                <span className="px-2 py-0.5 rounded bg-[#ffa729]/20 text-[#ffa729] text-xs font-medium">
                  QRC-20
                </span>
              </div>
              <div className="space-y-3">
                <div className="flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-2">
                  <span className="text-sm text-gray-400 min-w-[80px]">Method:</span>
                  <span className="text-white font-mono text-sm">
                    {decodedTransfer.methodName === 'transferFrom'
                      ? `${decodedTransfer.methodName}(address, address, uint256)`
                      : `${decodedTransfer.methodName}(address, uint256)`}
                  </span>
                </div>
                <div className="flex flex-col sm:flex-row sm:items-center gap-1 sm:gap-2">
                  <span className="text-sm text-gray-400 min-w-[80px]">Amount:</span>
                  <span className="text-white font-semibold">
                    {formatTokenAmount(decodedTransfer.amount, 18)}
                    <span className="text-gray-400 ml-2 text-sm">(raw: {decodedTransfer.amount})</span>
                  </span>
                </div>
                <div className="flex flex-col sm:flex-row sm:items-start gap-1 sm:gap-2">
                  <span className="text-sm text-gray-400 min-w-[80px]">Recipient:</span>
                  <a
                    href={`/address/${decodedTransfer.to}`}
                    className="text-[#ffa729] hover:text-[#ffb84d] font-mono text-sm transition-colors break-all"
                  >
                    {decodedTransfer.to}
                  </a>
                </div>
              </div>
              <p className="text-xs text-gray-500 mt-3">
                Note: Token name and symbol will be available once the transaction is confirmed.
              </p>
            </div>
          )}

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
              <h3 className="text-gray-400 mb-1">Input Data {isTokenTransfer && <span className="text-xs text-gray-500">(decoded above)</span>}</h3>
              <div className="bg-[#2d2d2d] p-4 rounded-lg">
                <p className="font-mono text-white break-all text-sm">{pendingTx.input}</p>
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
