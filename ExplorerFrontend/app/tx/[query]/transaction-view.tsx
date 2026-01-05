'use client';

import React, { useState, useEffect, Suspense } from 'react';
import { useSearchParams } from 'next/navigation';
import Link from 'next/link';
import type { TransactionDetails } from './types';
import { getConfirmations, getTransactionStatus } from './types';
import { formatAmount } from '../../lib/helpers';
import CopyHashButton from '../../components/CopyHashButton';
import CopyAddressButton from '../../components/CopyAddressButton';

// Back link component that uses useSearchParams
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
      Back to Latest Transactions
    </Link>
  );
}

const formatTimestamp = (timestamp: number): string => {
  if (!timestamp) return 'Unknown';
  const date = new Date(timestamp * 1000);
  if (date.getUTCFullYear() === 1970) return 'Pending';

  // Use UTC to avoid hydration mismatch
  const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
  const month = months[date.getUTCMonth()];
  const day = date.getUTCDate();
  const year = date.getUTCFullYear();
  const hours = date.getUTCHours().toString().padStart(2, '0');
  const minutes = date.getUTCMinutes().toString().padStart(2, '0');
  const seconds = date.getUTCSeconds().toString().padStart(2, '0');
  return `${month} ${day}, ${year}, ${hours}:${minutes}:${seconds} UTC`;
};

const AddressDisplay = ({ address, isMobile }: { address: string, isMobile: boolean }): JSX.Element => {
  const displayAddress = isMobile ? `${address.slice(0, 8)}...${address.slice(-6)}` : address;
  
  return (
    <div className="flex flex-col gap-2">
      <a 
        href={`/address/${address}`}
        className="text-gray-300 hover:text-[#ffa729] break-all font-mono 
                  transition-colors duration-300 group relative inline-block"
      >
        {displayAddress}
        <div className="absolute -inset-2 rounded-lg bg-[#3d3d3d] opacity-0 
                      group-hover:opacity-10 transition-opacity duration-300" />
      </a>
      <CopyAddressButton address={address} />
    </div>
  );
};

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

  // Calculate confirmations and get status
  const confirmations = getConfirmations(transaction.blockNumber, transaction.latestBlock);
  const status = getTransactionStatus(confirmations);

  // Format confirmation text
  const confirmationText = confirmations === null 
    ? 'Pending' 
    : `${confirmations} Confirmation${confirmations === 1 ? '' : 's'}`;

  // Format transaction value using the helper
  const [formattedValue, unit] = formatAmount(transaction.value);

  // Calculate paid fees from gas values
  const calculatePaidFees = (): string => {
    // Use PaidFees if available
    if (typeof transaction.PaidFees === 'number') {
      return transaction.PaidFees.toFixed(18);
    }
    
    // Fallback to manual calculation only if PaidFees is not available
    if (!transaction.gasUsed || !transaction.gasPrice) return '0';
    
    try {
      const gasUsed = BigInt(transaction.gasUsed);
      const gasPrice = BigInt(transaction.gasPrice);
      const paidFees = gasUsed * gasPrice;
      
      // Convert to QRL (divide by 10^18)
      const paidFeesQRL = Number(paidFees) / 1e18;
      return paidFeesQRL.toFixed(18); // Show full precision
    } catch (error) {
      console.error('Error calculating paid fees:', error);
      return '0';
    }
  };

  const paidFees = calculatePaidFees();

  return (
    <div className="py-4 md:py-8">
      {/* Back Button - only shown when coming from Latest Transactions */}
      <Suspense fallback={null}>
        <BackToTransactionsLink />
      </Suspense>

      <div className="relative overflow-hidden rounded-xl md:rounded-2xl
                    bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                    border border-[#3d3d3d] shadow-lg md:shadow-xl">
        <div className="p-4 md:p-8">
          {/* Header */}
          <div className="flex items-center justify-between mb-4 md:mb-8 pb-4 md:pb-6 border-b border-gray-700">
            <div className="flex items-center">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6 md:w-8 md:h-8 text-[#ffa729] mr-2 md:mr-3">
                <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
              </svg>
              <h1 className="text-lg md:text-2xl font-bold text-[#ffa729]">Transaction Details</h1>
            </div>
            <div className={`px-4 py-2 rounded-xl ${status.color} bg-opacity-20 border border-opacity-20 
                           flex items-center ${status.color.replace('bg-', 'border-')}`}>
              <div className={`w-2 h-2 rounded-full ${status.color} mr-2`}></div>
              <span className="text-sm font-medium">{status.text}</span>
            </div>
          </div>
          
          {/* Content Grid */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4 md:gap-8">
            {/* Left Column */}
            <div className="space-y-4 md:space-y-6">
              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">Transaction Hash</h2>
                <p className="text-gray-300 break-all font-mono mb-2">
                  {isMobile ? `${transaction.hash.slice(0, 10)}...${transaction.hash.slice(-8)}` : transaction.hash}
                </p>
                <CopyHashButton hash={transaction.hash} />
              </div>

              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">Block</h2>
                {transaction.blockNumber ? (
                  <div>
                    <a 
                      href={`/block/${transaction.blockNumber}`}
                      className="text-gray-300 hover:text-[#ffa729] transition-colors duration-300"
                    >
                      #{transaction.blockNumber}
                    </a>
                    <p className="text-sm text-gray-400 mt-1">{confirmationText}</p>
                  </div>
                ) : (
                  <p className="text-gray-300">Pending</p>
                )}
              </div>

              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">Timestamp</h2>
                <p className="text-gray-300">
                  {formatTimestamp(transaction.timestamp)}
                </p>
              </div>
            </div>

            {/* Right Column */}
            <div className="space-y-4 md:space-y-6">
              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">From</h2>
                <AddressDisplay address={transaction.from} isMobile={isMobile} />
              </div>

              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">To</h2>
                <AddressDisplay address={transaction.to} isMobile={isMobile} />
              </div>

              {/* Contract Creation Info */}
              {transaction.contractCreated && (
                <div className="col-span-1 md:col-span-2 pt-4 border-t border-gray-700">
                  <div className="bg-green-900/20 border border-green-500/30 rounded-xl p-4">
                    <div className="flex items-center gap-2 mb-3">
                      <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5 text-green-400">
                        <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                      </svg>
                      <h3 className="text-green-400 font-semibold">Contract Created</h3>
                      {transaction.contractCreated.isToken && (
                        <span className="px-2 py-0.5 rounded bg-[#ffa729]/20 text-[#ffa729] text-xs font-medium">
                          QRC-20 Token
                        </span>
                      )}
                    </div>
                    <div className="space-y-2">
                      <div>
                        <span className="text-sm text-gray-400">Contract Address: </span>
                        <a
                          href={`/address/${transaction.contractCreated.address}`}
                          className="text-[#ffa729] hover:text-[#ffb84d] font-mono text-sm transition-colors"
                        >
                          {isMobile
                            ? `${transaction.contractCreated.address.slice(0, 10)}...${transaction.contractCreated.address.slice(-8)}`
                            : transaction.contractCreated.address
                          }
                        </a>
                      </div>
                      {transaction.contractCreated.isToken && transaction.contractCreated.name && (
                        <div>
                          <span className="text-sm text-gray-400">Token: </span>
                          <span className="text-white font-medium">
                            {transaction.contractCreated.name} ({transaction.contractCreated.symbol})
                          </span>
                        </div>
                      )}
                    </div>
                  </div>
                </div>
              )}

              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">Value</h2>
                <p className="text-xl md:text-2xl font-semibold text-[#ffa729]">
                  {formattedValue}
                  <span className="text-sm text-gray-400 ml-2">{unit}</span>
                </p>
              </div>

              {(transaction.gasUsed || transaction.gasPrice) && (
                <div className="space-y-4 pt-4 border-t border-gray-700">
                  <div>
                    <h2 className="text-sm font-semibold text-gray-400 mb-2">Transaction Fee</h2>
                    <p className="text-xl md:text-2xl font-semibold text-[#ffa729]">
                      {paidFees}
                      <span className="text-sm text-gray-400 ml-2">QRL</span>
                    </p>
                  </div>
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
