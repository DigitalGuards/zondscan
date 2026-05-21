'use client';

import React, { useState, useEffect, Suspense } from 'react';
import { useSearchParams } from 'next/navigation';
import Link from 'next/link';
import { type TransactionDetails, getConfirmations, getTransactionStatus } from '@/app/types';
import { formatAmount, formatTokenAmount } from '../../lib/helpers';
import CopyButton from '../../components/CopyButton';
import Breadcrumbs from '../../components/Breadcrumbs';
import DetailRow from '../../components/DetailRow';
import Badge from '../../components/Badge';

function BackToTransactionsLink(): JSX.Element | null {
  const searchParams = useSearchParams();
  const fromTransactions = searchParams.get('from') === 'transactions';
  const returnPage = searchParams.get('page') || '1';

  if (!fromTransactions) return null;

  return (
    <Link
      href={`/transactions/${returnPage}`}
      className="inline-flex items-center text-gray-400 hover:text-[#ffa729] mb-4 md:mb-6"
    >
      <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
      </svg>
      Back to Transactions
    </Link>
  );
}

const formatTimestamp = (timestamp: number): string => {
  if (!timestamp) return 'Unknown';
  const date = new Date(timestamp * 1000);
  if (date.getUTCFullYear() === 1970) return 'Pending';

  const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
  const month = months[date.getUTCMonth()];
  const day = date.getUTCDate();
  const year = date.getUTCFullYear();
  const hours = date.getUTCHours().toString().padStart(2, '0');
  const minutes = date.getUTCMinutes().toString().padStart(2, '0');
  const seconds = date.getUTCSeconds().toString().padStart(2, '0');
  return `${month} ${day}, ${year}, ${hours}:${minutes}:${seconds} UTC`;
};

const isZeroAddress = (addr: string): boolean =>
  addr === 'Q0' || addr === 'Q' + '0'.repeat(40);

interface TransactionViewProps {
  transaction: TransactionDetails;
}

export default function TransactionView({ transaction }: TransactionViewProps): JSX.Element {
  const [isMobile, setIsMobile] = useState(false);

  useEffect(() => {
    const checkScreenSize = (): void => {
      setIsMobile(window.innerWidth < 768);
    };
    checkScreenSize();
    window.addEventListener('resize', checkScreenSize);
    return () => window.removeEventListener('resize', checkScreenSize);
  }, []);

  const confirmations = getConfirmations(transaction.blockNumber, transaction.latestBlock);
  const status = getTransactionStatus(confirmations);
  const confirmationText = confirmations === null
    ? 'Pending'
    : `${confirmations} Confirmation${confirmations === 1 ? '' : 's'}`;

  const [formattedValue, unit] = formatAmount(transaction.value);

  const calculatePaidFees = (): string => {
    if (typeof transaction.PaidFees === 'number') {
      return transaction.PaidFees.toFixed(18);
    }
    if (!transaction.gasUsed || !transaction.gasPrice) return '0';
    try {
      const gasUsed = BigInt(transaction.gasUsed);
      const gasPrice = BigInt(transaction.gasPrice);
      const paidFees = gasUsed * gasPrice;
      return (Number(paidFees) / 1e18).toFixed(18);
    } catch {
      return '0';
    }
  };

  const paidFees = calculatePaidFees();
  const badgeVariant = status.color === 'bg-green-500' ? 'success' as const
    : status.color === 'bg-blue-500' ? 'info' as const
    : 'warning' as const;

  const displayAddr = (addr: string): string =>
    isMobile ? `${addr.slice(0, 10)}...${addr.slice(-8)}` : addr;

  return (
    <div className="detail-content">
      <Breadcrumbs items={[
        { label: 'Transactions', href: '/transactions/1' },
        { label: `${transaction.hash.slice(0, 10)}...${transaction.hash.slice(-6)}` },
      ]} />

      <Suspense>
        <BackToTransactionsLink />
      </Suspense>

      {/* Main Details Card */}
      <div className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-xl overflow-hidden mb-6">
        {/* Header */}
        <div className="flex items-center justify-between p-4 sm:p-6 border-b border-[#3d3d3d]">
          <div className="flex items-center gap-3">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6 text-[#ffa729]">
              <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
            </svg>
            <h1 className="text-xl sm:text-2xl font-bold text-[#ffa729]">Transaction Details</h1>
          </div>
          <Badge variant={badgeVariant} size="md" dot>{status.text}</Badge>
        </div>

        {/* Content */}
        <div className="p-4 sm:p-6">
          <DetailRow label="Transaction Hash" mono>
            <div className="flex items-start gap-2">
              <span>{isMobile ? displayAddr(transaction.hash) : transaction.hash}</span>
              <CopyButton value={transaction.hash} label="Copy hash" size="sm" />
            </div>
          </DetailRow>
          <DetailRow label="Status">
            <Badge variant={badgeVariant} dot>{status.text}</Badge>
            <span className="text-gray-500 text-xs ml-2">{confirmationText}</span>
          </DetailRow>
          <DetailRow label="Block">
            {transaction.blockNumber ? (
              <Link
                href={`/block/${transaction.blockNumber}`}
                className="text-[#ffa729] hover:text-[#ffb954] transition-colors"
              >
                #{transaction.blockNumber}
              </Link>
            ) : (
              <span className="text-gray-400">Pending</span>
            )}
          </DetailRow>
          <DetailRow label="Timestamp">{formatTimestamp(transaction.timestamp)}</DetailRow>
          <DetailRow label="From" mono>
            <div className="flex items-start gap-2">
              <Link
                href={`/address/${transaction.from}`}
                className="text-gray-200 hover:text-[#ffa729] transition-colors break-all"
              >
                {displayAddr(transaction.from)}
              </Link>
              <CopyButton value={transaction.from} label="Copy address" size="sm" />
            </div>
          </DetailRow>
          <DetailRow label="To" mono>
            <div className="flex items-start gap-2">
              <Link
                href={`/address/${transaction.to}`}
                className="text-gray-200 hover:text-[#ffa729] transition-colors break-all"
              >
                {displayAddr(transaction.to)}
              </Link>
              <CopyButton value={transaction.to} label="Copy address" size="sm" />
            </div>
          </DetailRow>
          <DetailRow label="Value">
            <span className="font-semibold text-[#ffa729]">{formattedValue}</span>
            <span className="text-gray-500 ml-1">{unit}</span>
          </DetailRow>
          {(transaction.gasUsed || transaction.gasPrice) && (
            <DetailRow label="Transaction Fee">
              {paidFees}
              <span className="text-gray-500 ml-1">QRL</span>
            </DetailRow>
          )}
        </div>
      </div>

      {/* Contract Creation Section */}
      {transaction.contractCreated && (
        <div className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-xl overflow-hidden mb-6">
          <div className="px-4 sm:px-6 py-4 border-b border-[#3d3d3d]">
            <div className="flex items-center gap-2">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5 text-green-400">
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <h2 className="text-[15px] font-semibold text-green-400">Contract Created</h2>
              {transaction.contractCreated.isToken && <Badge variant="brand">QRC-20 Token</Badge>}
            </div>
          </div>
          <div className="p-4 sm:p-6">
            <DetailRow label="Contract Address" mono>
              <Link
                href={`/address/${transaction.contractCreated.address}`}
                className="text-[#ffa729] hover:text-[#ffb84d] transition-colors break-all"
              >
                {displayAddr(transaction.contractCreated.address)}
              </Link>
            </DetailRow>
            {transaction.contractCreated.isToken && transaction.contractCreated.name && (
              <DetailRow label="Token">
                <span className="font-medium text-white">
                  {transaction.contractCreated.name} ({transaction.contractCreated.symbol})
                </span>
              </DetailRow>
            )}
          </div>
        </div>
      )}

      {/* Token Transfer Section */}
      {transaction.tokenTransfer && (
        <div className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-xl overflow-hidden mb-6">
          <div className="px-4 sm:px-6 py-4 border-b border-[#3d3d3d]">
            <div className="flex items-center gap-2">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5 text-[#ffa729]">
                <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
              </svg>
              <h2 className="text-[15px] font-semibold text-[#ffa729]">Token Transfer</h2>
              <Badge variant="brand">QRC-20</Badge>
            </div>
          </div>
          <div className="p-4 sm:p-6">
            <DetailRow label="Token">
              <Link
                href={`/address/${transaction.tokenTransfer.contractAddress}`}
                className="text-[#ffa729] hover:text-[#ffb84d] font-medium transition-colors"
              >
                {transaction.tokenTransfer.tokenName} ({transaction.tokenTransfer.tokenSymbol})
              </Link>
            </DetailRow>
            <DetailRow label="Amount">
              <span className="font-semibold text-white">
                {formatTokenAmount(transaction.tokenTransfer.amount, transaction.tokenTransfer.tokenDecimals)}
              </span>
              <span className="text-[#ffa729] ml-2 text-sm">{transaction.tokenTransfer.tokenSymbol}</span>
            </DetailRow>
            <DetailRow label="From" mono>
              {isZeroAddress(transaction.tokenTransfer.from) ? (
                <div className="flex items-center gap-2">
                  <Badge variant="success">Mint</Badge>
                  <span className="text-sm text-gray-400">
                    via{' '}
                    <Link
                      href={`/address/${transaction.tokenTransfer.contractAddress}`}
                      className="text-[#ffa729] hover:text-[#ffb84d] transition-colors"
                    >
                      {transaction.tokenTransfer.tokenName} Contract
                    </Link>
                  </span>
                </div>
              ) : (
                <Link
                  href={`/address/${transaction.tokenTransfer.from}`}
                  className="text-gray-200 hover:text-[#ffa729] transition-colors break-all"
                >
                  {displayAddr(transaction.tokenTransfer.from)}
                </Link>
              )}
            </DetailRow>
            <DetailRow label="To" mono>
              {isZeroAddress(transaction.tokenTransfer.to) ? (
                <Badge variant="error">Burn</Badge>
              ) : (
                <Link
                  href={`/address/${transaction.tokenTransfer.to}`}
                  className="text-gray-200 hover:text-[#ffa729] transition-colors break-all"
                >
                  {displayAddr(transaction.tokenTransfer.to)}
                </Link>
              )}
            </DetailRow>
          </div>
        </div>
      )}
    </div>
  );
}
