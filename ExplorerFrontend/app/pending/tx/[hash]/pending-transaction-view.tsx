'use client';

import React, { useMemo } from 'react';
import Link from 'next/link';
import type { PendingTransaction } from '@/app/types';
import { formatAmount, formatGasPrice, decodeTokenTransferInput, formatTokenAmount } from '../../../lib/helpers';
import Badge from '../../../components/Badge';
import Breadcrumbs from '../../../components/Breadcrumbs';
import DetailRow from '../../../components/DetailRow';
import CopyButton from '../../../components/CopyButton';

interface PendingTransactionViewProps {
  pendingTx: PendingTransaction;
}

export default function PendingTransactionView({ pendingTx }: PendingTransactionViewProps): JSX.Element {
  const [formattedValue, unit] = formatAmount(pendingTx.value);
  const formattedGasPrice = formatGasPrice(pendingTx.gasPrice);

  const decodedTransfer = useMemo(() => {
    return decodeTokenTransferInput(pendingTx.input);
  }, [pendingTx.input]);

  const isTokenTransfer = decodedTransfer !== null;

  return (
    <div className="p-4 sm:p-8 max-w-6xl mx-auto">
      <Breadcrumbs items={[
        { label: 'Pending', href: '/pending/1' },
        { label: `${pendingTx.hash.slice(0, 10)}...${pendingTx.hash.slice(-6)}` },
      ]} />

      {/* Main Details Card */}
      <div className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-xl overflow-hidden mb-6">
        {/* Header */}
        <div className="flex items-center justify-between p-4 sm:p-6 border-b border-[#3d3d3d]">
          <div className="flex items-center gap-3">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6 text-[#ffa729]">
              <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
            </svg>
            <h1 className="text-xl sm:text-2xl font-bold text-[#ffa729]">Pending Transaction</h1>
            {isTokenTransfer && <Badge variant="brand">Token Transfer</Badge>}
          </div>
          <Badge variant="warning" size="md" dot>Pending</Badge>
        </div>

        {/* Content */}
        <div className="p-4 sm:p-6">
          <DetailRow label="Transaction Hash" mono>
            <div className="flex items-start gap-2">
              <span>{pendingTx.hash}</span>
              <CopyButton value={pendingTx.hash} label="Copy hash" size="sm" />
            </div>
          </DetailRow>
          <DetailRow label="Status">
            <Badge variant="warning" dot>pending</Badge>
          </DetailRow>
          <DetailRow label="From" mono>
            <Link
              href={`/address/${pendingTx.from}`}
              className="text-gray-200 hover:text-[#ffa729] transition-colors break-all"
            >
              {pendingTx.from}
            </Link>
          </DetailRow>
          <DetailRow label="To" mono>
            {pendingTx.to ? (
              <Link
                href={`/address/${pendingTx.to}`}
                className="text-gray-200 hover:text-[#ffa729] transition-colors break-all"
              >
                {pendingTx.to}
              </Link>
            ) : (
              <span className="text-gray-400">Contract Creation</span>
            )}
            {isTokenTransfer && <span className="text-xs text-gray-500 ml-2">(Contract)</span>}
          </DetailRow>
          <DetailRow label="Value">
            <span className="font-semibold text-[#ffa729]">{formattedValue}</span>
            <span className="text-gray-500 ml-1">{unit}</span>
          </DetailRow>
          <DetailRow label="Gas Price">{formattedGasPrice} Gwei</DetailRow>
          <DetailRow label="Gas Limit">{pendingTx.gas}</DetailRow>
          <DetailRow label="Nonce">{pendingTx.nonce}</DetailRow>
        </div>
      </div>

      {/* Token Transfer Section */}
      {isTokenTransfer && decodedTransfer && (
        <div className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-xl overflow-hidden mb-6">
          <div className="px-4 sm:px-6 py-4 border-b border-[#3d3d3d]">
            <div className="flex items-center gap-2">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5 text-[#ffa729]">
                <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
              </svg>
              <h2 className="text-[15px] font-semibold text-[#ffa729]">Token Transfer (Pending)</h2>
              <Badge variant="brand">QRC-20</Badge>
            </div>
          </div>
          <div className="p-4 sm:p-6">
            <DetailRow label="Method">
              <span className="font-mono text-sm">
                {decodedTransfer.methodName === 'transferFrom'
                  ? `${decodedTransfer.methodName}(address, address, uint256)`
                  : `${decodedTransfer.methodName}(address, uint256)`}
              </span>
            </DetailRow>
            <DetailRow label="Amount">
              <span className="font-semibold">{formatTokenAmount(decodedTransfer.amount, 18)}</span>
              <span className="text-gray-500 ml-2 text-xs">(raw: {decodedTransfer.amount})</span>
            </DetailRow>
            <DetailRow label="Recipient" mono>
              <Link
                href={`/address/${decodedTransfer.to}`}
                className="text-[#ffa729] hover:text-[#ffb84d] transition-colors break-all"
              >
                {decodedTransfer.to}
              </Link>
            </DetailRow>
            <p className="text-xs text-gray-500 mt-3">
              Token name and symbol will be available once the transaction is confirmed.
            </p>
          </div>
        </div>
      )}

      {/* Input Data Section */}
      {pendingTx.input && pendingTx.input !== '0x' && (
        <div className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-xl overflow-hidden">
          <div className="px-4 sm:px-6 py-4 border-b border-[#3d3d3d]">
            <h2 className="text-[15px] font-semibold text-[#ffa729]">
              Input Data
              {isTokenTransfer && <span className="text-xs text-gray-500 ml-2">(decoded above)</span>}
            </h2>
          </div>
          <div className="p-4 sm:p-6">
            <p className="font-mono text-gray-300 break-all text-xs leading-relaxed">{pendingTx.input}</p>
          </div>
        </div>
      )}
    </div>
  );
}
