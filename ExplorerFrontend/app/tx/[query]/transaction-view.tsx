'use client';

import React, { useState, useEffect, Suspense } from 'react';
import { useSearchParams } from 'next/navigation';
import Link from 'next/link';
import axios from 'axios';
import { useQuery } from '@tanstack/react-query';
import { type TransactionDetails, getConfirmations, getTransactionStatus } from '@/app/types';
import { formatAmount, formatTokenAmount, decodeEventLog, decodeTokenTransferInput, decodeContractCall } from '../../lib/helpers';
import config from '../../../config';
import CopyButton from '../../components/CopyButton';
import Breadcrumbs from '../../components/Breadcrumbs';
import DetailRow from '../../components/DetailRow';
import Badge from '../../components/Badge';

// Once a tx has this many confirmations we stop polling /latestblock for
// it; further refinement is just visual noise (most chain UIs treat
// >12-25 as effectively final). Keeps the polling cost bounded for
// long-lived tabs.
const TERMINAL_CONFIRMATIONS = 24;

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

// QRC badge label for a token standard. Keeps the contract-created header
// and per-row transfer header in lock-step, both branch on the same
// syncer-supplied tokenStandard value (mirrors backendAPI ContractInfo +
// TokenTransfer). Anything we don't recognise falls back to QRC-20 so the
// UI stays usable on legacy un-tagged rows.
function qrcBadgeText(standard?: string): string {
  if (standard === 'ERC-721') return 'QRC-721';
  if (standard === 'ERC-1155') return 'QRC-1155';
  return 'QRC-20';
}

// ERC-1155 amounts are integer counts (no decimals on chain) but still
// benefit from thousands separators on multi-digit values. Falls back to
// the raw string if BigInt rejects it.
function formatErc1155Amount(amount: string): string {
  try {
    return BigInt(amount).toLocaleString('en-US');
  } catch {
    return amount;
  }
}

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

  // Poll /latestblock so the confirmation count ticks up live; stops
  // once we cross the terminal-confirmations threshold to bound load
  // on long-lived tabs. /latestblock is cached server-side for 5s
  // (iter 15 audit), so concurrent visitors fan into one DB hit per
  // bucket.
  const ssrConfirmations = getConfirmations(transaction.blockNumber, transaction.latestBlock);
  const latestBlockQuery = useQuery<{ blockNumber: number }>({
    queryKey: ['latestblock'],
    queryFn: async () => {
      const r = await axios.get<{ blockNumber: number }>(`${config.handlerUrl}/latestblock`);
      return r.data;
    },
    enabled: ssrConfirmations !== null && ssrConfirmations < TERMINAL_CONFIRMATIONS,
    refetchInterval: (q) => {
      const latest = q.state.data?.blockNumber ?? transaction.latestBlock;
      const live = getConfirmations(transaction.blockNumber, latest);
      return live !== null && live < TERMINAL_CONFIRMATIONS ? 5000 : false;
    },
    refetchIntervalInBackground: false,
  });

  const effectiveLatest = Math.max(
    transaction.latestBlock ?? 0,
    latestBlockQuery.data?.blockNumber ?? 0,
  );
  const confirmations = getConfirmations(
    transaction.blockNumber,
    effectiveLatest > 0 ? effectiveLatest : transaction.latestBlock,
  );
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
    <main className="detail-content" aria-labelledby="tx-detail-heading">
      <Breadcrumbs items={[
        { label: 'Transactions', href: '/transactions/1' },
        { label: `${transaction.hash.slice(0, 10)}...${transaction.hash.slice(-6)}` },
      ]} />

      <Suspense>
        <BackToTransactionsLink />
      </Suspense>

      {/* Main Details Card */}
      <section
        aria-labelledby="tx-detail-heading"
        className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-xl overflow-hidden mb-6"
      >
        {/* Header */}
        <div className="flex items-center justify-between p-4 sm:p-6 border-b border-[#3d3d3d]">
          <div className="flex items-center gap-3">
            <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6 text-[#ffa729]" aria-hidden="true">
              <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
            </svg>
            <h1 id="tx-detail-heading" className="text-xl sm:text-2xl font-bold text-[#ffa729]">Transaction Details</h1>
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
      </section>

      {/* Contract Creation Section */}
      {transaction.contractCreated && (() => {
        const cc = transaction.contractCreated;
        const ccStandard = cc.tokenStandard;
        // ERC-1155 contracts often leave name()/symbol() unimplemented, the
        // standard requires neither. Fall back to a placeholder rather than
        // hiding the row, since the badge already tells the user it's a
        // token contract.
        const tokenLabel = cc.name
          ? `${cc.name}${cc.symbol && cc.symbol !== cc.name ? ` (${cc.symbol})` : ''}`
          : cc.symbol
            ? `(${cc.symbol})`
            : 'Unnamed Collection';
        return (
        <section aria-labelledby="contract-created-heading" className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-xl overflow-hidden mb-6">
          <div className="px-4 sm:px-6 py-4 border-b border-[#3d3d3d]">
            <div className="flex items-center gap-2">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5 text-green-400" aria-hidden="true">
                <path strokeLinecap="round" strokeLinejoin="round" d="M9 12.75L11.25 15 15 9.75M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
              </svg>
              <h2 id="contract-created-heading" className="text-[15px] font-semibold text-green-400">Contract Created</h2>
              {cc.isToken && (
                <Badge variant={ccStandard === 'ERC-721' || ccStandard === 'ERC-1155' ? 'warning' : 'brand'}>
                  {qrcBadgeText(ccStandard)} Token
                </Badge>
              )}
            </div>
          </div>
          <div className="p-4 sm:p-6">
            <DetailRow label="Contract Address" mono>
              <Link
                href={`/address/${cc.address}`}
                className="text-[#ffa729] hover:text-[#ffb84d] transition-colors break-all"
              >
                {displayAddr(cc.address)}
              </Link>
            </DetailRow>
            {cc.isToken && (
              <DetailRow label="Token">
                <span className="font-medium text-white">{tokenLabel}</span>
              </DetailRow>
            )}
          </div>
        </section>
        );
      })()}

      {/* Token Transfer Section(s). A single tx can emit several Transfer
          events (DEX swaps, ERC-1155 TransferBatch fan-out), each persisted
          as its own row, so we render one card per row. */}
      {transaction.tokenTransfers && transaction.tokenTransfers.length > 0 &&
        transaction.tokenTransfers.map((tt, idx) => {
          const standard = tt.tokenStandard;
          const headerLabel =
            standard === 'ERC-721' ? 'NFT Transfer'
            : standard === 'ERC-1155' ? 'Multi-Token Transfer'
            : 'Token Transfer';
          const isNFT = standard === 'ERC-721' || standard === 'ERC-1155';
          const rowKey = tt.logIndex || `${tt.contractAddress}-${idx}`;
          return (
        <section
          key={rowKey}
          aria-labelledby={`token-transfer-${rowKey}-heading`}
          className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-xl overflow-hidden mb-6"
        >
          <div className="px-4 sm:px-6 py-4 border-b border-[#3d3d3d]">
            <div className="flex items-center gap-2">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5 text-[#ffa729]" aria-hidden="true">
                <path strokeLinecap="round" strokeLinejoin="round" d="M7.5 21L3 16.5m0 0L7.5 12M3 16.5h13.5m0-13.5L21 7.5m0 0L16.5 12M21 7.5H7.5" />
              </svg>
              <h2 id={`token-transfer-${rowKey}-heading`} className="text-[15px] font-semibold text-[#ffa729]">{headerLabel}</h2>
              <Badge variant={isNFT ? 'warning' : 'brand'}>{qrcBadgeText(standard)}</Badge>
            </div>
          </div>
          <div className="p-4 sm:p-6">
            <DetailRow label={standard === 'ERC-721' ? 'Collection' : standard === 'ERC-1155' ? 'Collection' : 'Token'}>
              <Link
                href={`/address/${tt.contractAddress}`}
                className="text-[#ffa729] hover:text-[#ffb84d] font-medium transition-colors"
              >
                {tt.tokenName || tt.tokenSymbol || tt.contractAddress}
                {tt.tokenName && tt.tokenSymbol && tt.tokenSymbol !== tt.tokenName ? ` (${tt.tokenSymbol})` : ''}
              </Link>
            </DetailRow>
            {tt.tokenID && (
              <DetailRow label="Token ID" mono>
                <span className="font-semibold text-white">#{tt.tokenID}</span>
              </DetailRow>
            )}
            {standard !== 'ERC-721' && (
              <DetailRow label="Amount">
                <span className="font-semibold text-white">
                  {standard === 'ERC-1155'
                    ? formatErc1155Amount(tt.amount)
                    : formatTokenAmount(tt.amount, tt.tokenDecimals)}
                </span>
                {tt.tokenSymbol && (
                  <span className="text-[#ffa729] ml-2 text-sm">{tt.tokenSymbol}</span>
                )}
              </DetailRow>
            )}
            <DetailRow label="From" mono>
              {isZeroAddress(tt.from) ? (
                <div className="flex items-center gap-2">
                  <Badge variant="success">Mint</Badge>
                  <span className="text-sm text-gray-400">
                    via{' '}
                    <Link
                      href={`/address/${tt.contractAddress}`}
                      className="text-[#ffa729] hover:text-[#ffb84d] transition-colors"
                    >
                      {tt.tokenName || 'Contract'}
                    </Link>
                  </span>
                </div>
              ) : (
                <Link
                  href={`/address/${tt.from}`}
                  className="text-gray-200 hover:text-[#ffa729] transition-colors break-all"
                >
                  {displayAddr(tt.from)}
                </Link>
              )}
            </DetailRow>
            <DetailRow label="To" mono>
              {isZeroAddress(tt.to) ? (
                <Badge variant="error">Burn</Badge>
              ) : (
                <Link
                  href={`/address/${tt.to}`}
                  className="text-gray-200 hover:text-[#ffa729] transition-colors break-all"
                >
                  {displayAddr(tt.to)}
                </Link>
              )}
            </DetailRow>
          </div>
        </section>
          );
        })}

      {/* Event Logs Panel. Decodes the five well-known token signatures
          (Transfer / TransferSingle / TransferBatch / Approval /
          ApprovalForAll) inline; everything else falls back to topics +
          data so users can copy them into a decoder. */}
      {transaction.logs && transaction.logs.length > 0 && (
        <section aria-labelledby="event-logs-heading" className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-xl overflow-hidden mb-6">
          <div className="px-4 sm:px-6 py-4 border-b border-[#3d3d3d]">
            <div className="flex items-center gap-2">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5 text-[#ffa729]" aria-hidden="true">
                <path strokeLinecap="round" strokeLinejoin="round" d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5" />
              </svg>
              <h2 id="event-logs-heading" className="text-[15px] font-semibold text-[#ffa729]">Event Logs</h2>
              <Badge variant="neutral">{transaction.logs.length}</Badge>
            </div>
          </div>
          <div className="p-4 sm:p-6 space-y-4">
            {transaction.logs.map((logEntry) => {
              // Pass the per-log ABI (when the emitting contract is verified)
              // so the decoder can fall through from the five known
              // signatures to a full ABI-driven decode.
              const decoded = decodeEventLog(logEntry.topics, logEntry.data, logEntry.contract?.abi);
              const idxLabel = (() => { try { return parseInt(logEntry.logIndex || '0x0', 16).toString(); } catch { return logEntry.logIndex; } })();
              const decodedViaAbi = !!decoded && !decoded.standard;
              return (
                <div key={`${logEntry.address}-${logEntry.logIndex}`} className="rounded-lg bg-[#1a1a1a]/60 border border-[#3d3d3d] p-3 sm:p-4">
                  <div className="flex flex-wrap items-center gap-2 mb-2">
                    <Badge variant="neutral">#{idxLabel}</Badge>
                    {decoded ? (
                      <>
                        <Badge variant={decoded.standard === 'ERC-721' || decoded.standard === 'ERC-1155' ? 'warning' : decoded.standard ? 'brand' : 'info'}>
                          {decoded.name}
                        </Badge>
                        {decoded.standard && (
                          <span className="text-xs text-gray-500 font-mono">{decoded.standard}</span>
                        )}
                        {decodedViaAbi && logEntry.contract?.contractName && (
                          <span className="text-xs text-gray-500 font-mono">via {logEntry.contract.contractName}</span>
                        )}
                      </>
                    ) : (
                      <Badge variant="neutral">Raw</Badge>
                    )}
                    <Link
                      href={`/address/${logEntry.address}`}
                      className="text-[#ffa729] hover:text-[#ffb84d] transition-colors break-all text-xs font-mono"
                    >
                      {logEntry.address}
                    </Link>
                  </div>
                  {decoded ? (
                    <>
                      <p className="text-xs text-gray-500 font-mono mb-2">{decoded.signature}</p>
                      <div className="space-y-1">
                        {decoded.args.map((arg) => (
                          <div key={arg.label} className="text-xs flex flex-wrap items-start gap-2">
                            <span className="text-gray-400 font-mono min-w-[80px]">{arg.label}:</span>
                            {arg.type === 'address' && arg.value ? (
                              <Link
                                href={`/address/${arg.value}`}
                                className="text-gray-200 hover:text-[#ffa729] transition-colors break-all font-mono"
                              >
                                {arg.value}
                              </Link>
                            ) : arg.type === 'bool' ? (
                              <Badge variant={arg.value === 'true' ? 'success' : 'error'}>{arg.value}</Badge>
                            ) : arg.type === 'uint256' ? (
                              <span className="text-gray-200 font-mono break-all">
                                {(() => { try { return BigInt(arg.value || '0').toLocaleString('en-US'); } catch { return arg.value || ''; } })()}
                              </span>
                            ) : arg.type === 'uint256[]' ? (
                              <span className="text-gray-200 font-mono break-all">
                                {arg.values && arg.values.length > 0
                                  ? arg.values.map((v) => {
                                      try { return BigInt(v).toLocaleString('en-US'); } catch { return v; }
                                    }).join(', ')
                                  : '[]'}
                              </span>
                            ) : arg.type === 'string' ? (
                              <span className="text-gray-200 break-all">{arg.value}</span>
                            ) : (
                              // 'raw' or unknown; surface the slot so it's at
                              // least copy-pasteable into another decoder.
                              <span className="text-gray-200 font-mono break-all">{arg.value}</span>
                            )}
                          </div>
                        ))}
                      </div>
                    </>
                  ) : (
                    <div className="text-xs space-y-1">
                      <div>
                        <span className="text-gray-400 font-mono">topics:</span>
                        <div className="ml-2 mt-1 space-y-0.5">
                          {logEntry.topics.map((t, ti) => (
                            <div key={ti} className="text-gray-200 font-mono break-all">[{ti}] {t}</div>
                          ))}
                        </div>
                      </div>
                      {logEntry.data && logEntry.data !== '0x' && (
                        <div>
                          <span className="text-gray-400 font-mono">data:</span>
                          <p className="ml-2 mt-1 text-gray-200 font-mono break-all">{logEntry.data}</p>
                        </div>
                      )}
                      {/* The decoder fell through to raw because the emitting
                          contract isn't verified (or has no contractCode row
                          at all). Surface a CTA so users hit a path of
                          action instead of a wall of hex. */}
                      {(!logEntry.contract || !logEntry.contract.verified) && (
                        <p className="mt-2 text-[11px] text-gray-500">
                          <Link
                            href={`/verify-contract?address=${logEntry.address}`}
                            className="text-[#ffa729] hover:text-[#ffb84d] hover:underline"
                          >
                            Verify this contract
                          </Link>
                          {' '}to see decoded event names and labelled arguments here.
                        </p>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </section>
      )}

      {/* Internal Transactions panel. The syncer records each EVM
          sub-frame (CALL / DELEGATECALL / STATICCALL) the tx triggered
          under `internalTransactionByAddress` keyed by parent tx hash.
          Most simple txs have none; complex contract calls fan out. */}
      {transaction.internalTransactions && transaction.internalTransactions.length > 0 && (
        <section aria-labelledby="internal-tx-heading" className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-xl overflow-hidden mb-6">
          <div className="px-4 sm:px-6 py-4 border-b border-[#3d3d3d]">
            <div className="flex items-center gap-2">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-5 h-5 text-[#ffa729]" aria-hidden="true">
                <path strokeLinecap="round" strokeLinejoin="round" d="M8.25 6.75h12M8.25 12h12m-12 5.25h12M3.75 6.75h.007v.008H3.75V6.75zm.375 0a.375.375 0 11-.75 0 .375.375 0 01.75 0zM3.75 12h.007v.008H3.75V12zm.375 0a.375.375 0 11-.75 0 .375.375 0 01.75 0zM3.75 17.25h.007v.008H3.75v-.008zm.375 0a.375.375 0 11-.75 0 .375.375 0 01.75 0z" />
              </svg>
              <h2 id="internal-tx-heading" className="text-[15px] font-semibold text-[#ffa729]">Internal Transactions</h2>
              <Badge variant="neutral">{transaction.internalTransactions.length}</Badge>
            </div>
          </div>
          <div className="p-4 sm:p-6 space-y-3">
            {transaction.internalTransactions.map((itx, i) => {
              const depth = itx.traceAddress?.length || 0;
              const path = itx.traceAddress && itx.traceAddress.length > 0
                ? itx.traceAddress.join(',')
                : 'root';
              return (
                <div
                  key={`${path}-${i}`}
                  className="rounded-lg bg-[#1a1a1a]/60 border border-[#3d3d3d] p-3 sm:p-4"
                  style={{ marginLeft: depth > 0 ? `${Math.min(depth, 4) * 12}px` : 0 }}
                >
                  <div className="flex flex-wrap items-center gap-2 mb-2">
                    <Badge variant="neutral">{path}</Badge>
                    {itx.callType && (
                      <Badge variant={itx.callType === 'DELEGATECALL' || itx.callType === 'STATICCALL' ? 'info' : 'brand'}>
                        {itx.callType}
                      </Badge>
                    )}
                    {itx.value > 0 && (
                      <Badge variant="warning">{itx.value} QRL</Badge>
                    )}
                  </div>
                  <div className="space-y-1 text-xs">
                    {itx.from && (
                      <div className="flex flex-wrap items-start gap-2">
                        <span className="text-gray-400 font-mono min-w-[60px]">from:</span>
                        <Link href={`/address/${itx.from}`} className="text-gray-200 hover:text-[#ffa729] transition-colors break-all font-mono">
                          {itx.from}
                        </Link>
                      </div>
                    )}
                    {itx.to && (
                      <div className="flex flex-wrap items-start gap-2">
                        <span className="text-gray-400 font-mono min-w-[60px]">to:</span>
                        <Link href={`/address/${itx.to}`} className="text-[#ffa729] hover:text-[#ffb84d] transition-colors break-all font-mono">
                          {itx.to}
                        </Link>
                      </div>
                    )}
                    {itx.input && itx.input !== '0x' && itx.input !== '0x0' && (
                      <div className="flex flex-wrap items-start gap-2">
                        <span className="text-gray-400 font-mono min-w-[60px]">input:</span>
                        <span className="text-gray-300 font-mono break-all">
                          {itx.input.length > 18 ? `${itx.input.slice(0, 18)}…` : itx.input}
                        </span>
                      </div>
                    )}
                    {itx.gasUsed && itx.gasUsed !== '0x0' && (
                      <div className="flex flex-wrap items-start gap-2">
                        <span className="text-gray-400 font-mono min-w-[60px]">gasUsed:</span>
                        <span className="text-gray-300 font-mono">
                          {(() => { try { return BigInt(itx.gasUsed).toLocaleString('en-US'); } catch { return itx.gasUsed; } })()}
                        </span>
                      </div>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      )}

      {/* Input Data Card. Decode in three tiers:
          1. Known ERC token selectors via decodeTokenTransferInput (renders
             with the standard-tagged badge from the original UI).
          2. Target contract ABI via decodeContractCall (verified contracts
             only); renders method name + labelled args, same scheme as
             the Event Logs panel for ABI-decoded events.
          3. Raw calldata fallback so users can copy it into any decoder. */}
      {transaction.input && transaction.input !== '0x' && (() => {
        const decodedInput = decodeTokenTransferInput(transaction.input);
        const decodedCall = !decodedInput
          ? decodeContractCall(transaction.input, transaction.targetContract?.abi)
          : null;
        return (
          <section aria-labelledby="input-data-heading" className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-xl overflow-hidden mb-6">
            <div className="px-4 sm:px-6 py-4 border-b border-[#3d3d3d]">
              <div className="flex items-center gap-2">
                <h2 id="input-data-heading" className="text-[15px] font-semibold text-[#ffa729]">Input Data</h2>
                {decodedInput && (
                  <Badge variant={decodedInput.standard === 'ERC-20' ? 'brand' : 'warning'}>
                    {decodedInput.methodName}
                  </Badge>
                )}
                {decodedCall && (
                  <>
                    <Badge variant="info">{decodedCall.name}</Badge>
                    {transaction.targetContract?.contractName && (
                      <span className="text-xs text-gray-500 font-mono">via {transaction.targetContract.contractName}</span>
                    )}
                  </>
                )}
              </div>
            </div>
            <div className="p-4 sm:p-6">
              {decodedInput && (
                <p className="text-xs text-gray-500 font-mono mb-2">
                  {decodedInput.standard} · {decodedInput.methodName}
                </p>
              )}
              {decodedCall && (
                <>
                  <p className="text-xs text-gray-500 font-mono mb-2">{decodedCall.signature}</p>
                  <div className="space-y-1 mb-3">
                    {decodedCall.args.map((arg) => (
                      <div key={arg.label} className="text-xs flex flex-wrap items-start gap-2">
                        <span className="text-gray-400 font-mono min-w-[80px]">{arg.label}:</span>
                        {arg.type === 'address' && arg.value ? (
                          <Link
                            href={`/address/${arg.value}`}
                            className="text-gray-200 hover:text-[#ffa729] transition-colors break-all font-mono"
                          >
                            {arg.value}
                          </Link>
                        ) : arg.type === 'bool' ? (
                          <Badge variant={arg.value === 'true' ? 'success' : 'error'}>{arg.value}</Badge>
                        ) : arg.type === 'uint256' ? (
                          <span className="text-gray-200 font-mono break-all">
                            {(() => { try { return BigInt(arg.value || '0').toLocaleString('en-US'); } catch { return arg.value || ''; } })()}
                          </span>
                        ) : arg.type === 'uint256[]' ? (
                          <span className="text-gray-200 font-mono break-all">
                            {arg.values && arg.values.length > 0
                              ? arg.values.map((v) => { try { return BigInt(v).toLocaleString('en-US'); } catch { return v; } }).join(', ')
                              : '[]'}
                          </span>
                        ) : arg.type === 'string' ? (
                          <span className="text-gray-200 break-all">{arg.value}</span>
                        ) : (
                          <span className="text-gray-200 font-mono break-all">{arg.value}</span>
                        )}
                      </div>
                    ))}
                  </div>
                </>
              )}
              <p className="font-mono text-gray-300 break-all text-xs leading-relaxed">{transaction.input}</p>
              {/* Neither the well-known-token-selector decoder nor the
                  ABI decoder produced a hit. If the target has a
                  contractCode row but isn't verified, nudge users
                  toward verification so future hits to this tx show
                  a labelled method instead of raw hex. */}
              {!decodedInput && !decodedCall && transaction.to && transaction.targetContract && !transaction.targetContract.verified && (
                <p className="mt-2 text-[11px] text-gray-500">
                  <Link
                    href={`/verify-contract?address=${transaction.to}`}
                    className="text-[#ffa729] hover:text-[#ffb84d] hover:underline"
                  >
                    Verify this contract
                  </Link>
                  {' '}to see the method name + labelled arguments instead of raw calldata.
                </p>
              )}
            </div>
          </section>
        );
      })()}
    </main>
  );
}
