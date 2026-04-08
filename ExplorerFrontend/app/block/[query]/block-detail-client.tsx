"use client";

import axios from 'axios';
import React, { useState, useEffect, Suspense } from 'react';
import { useSearchParams } from 'next/navigation';
import config from '../../../config';
import Link from 'next/link';
import { formatAmount, truncateHash, timeAgo } from '../../lib/helpers';
import Breadcrumbs from '../../components/Breadcrumbs';
import DetailRow from '../../components/DetailRow';
import CopyButton from '../../components/CopyButton';
import EmptyState from '../../components/EmptyState';

function BackToBlocksLink(): JSX.Element | null {
  const searchParams = useSearchParams();
  const fromBlocks = searchParams.get('from') === 'blocks';
  const returnPage = searchParams.get('page') || '1';

  if (!fromBlocks) return null;

  return (
    <Link
      href={`/blocks/${returnPage}`}
      className="inline-flex items-center text-gray-400 hover:text-[#ffa729] mb-4 md:mb-6"
    >
      <svg className="w-4 h-4 mr-2" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
      </svg>
      Back to Blocks
    </Link>
  );
}

type Block = {
  baseFeePerGas: string;
  gasLimit: string;
  gasUsed: string;
  hash: string;
  number: string;
  parentHash: string;
  receiptsRoot: string;
  stateRoot: string;
  timestamp: string;
  transactions: Array<{
    hash: string;
    from: string;
    to: string;
    value: string;
  }>;
  transactionsRoot: string;
  extraData: string;
  logsBloom: string;
  miner: string;
  size: string;
  prevRandao: string;
  withdrawals: any[];
  withdrawalsRoot: string;
};

interface BlockDetailClientProps {
  blockNumber: string;
}

const formatHexValue = (hex: string | null | undefined): string => {
  if (!hex) return '0';
  const num = typeof hex === 'string' && hex.startsWith('0x')
    ? parseInt(hex, 16)
    : parseInt(hex);
  if (isNaN(num)) return '0';
  return num.toString().replace(/\B(?=(\d{3})+(?!\d))/g, ',');
};

const formatTimestampUTC = (timestamp: string | null | undefined): string => {
  if (!timestamp) return 'N/A';
  const ts = typeof timestamp === 'string' && timestamp.startsWith('0x')
    ? parseInt(timestamp, 16)
    : parseInt(timestamp);
  if (isNaN(ts)) return 'N/A';
  const date = new Date(ts * 1000);
  const month = (date.getUTCMonth() + 1).toString().padStart(2, '0');
  const day = date.getUTCDate().toString().padStart(2, '0');
  const year = date.getUTCFullYear();
  const hours = date.getUTCHours().toString().padStart(2, '0');
  const minutes = date.getUTCMinutes().toString().padStart(2, '0');
  const seconds = date.getUTCSeconds().toString().padStart(2, '0');
  return `${month}/${day}/${year}, ${hours}:${minutes}:${seconds} UTC`;
};

export default function BlockDetailClient({ blockNumber }: BlockDetailClientProps): JSX.Element {
  const [blockData, setBlockData] = useState<Block | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const blockNum = parseInt(blockNumber);

  useEffect(() => {
    const fetchBlock = async (): Promise<void> => {
      try {
        setLoading(true);
        const response = await axios.get(`${config.handlerUrl}/block/${blockNumber}`);
        const block = response.data?.block?.result;
        if (!block) throw new Error('Invalid block data received');

        setBlockData({
          baseFeePerGas: block.baseFeePerGas || '0x0',
          gasLimit: block.gasLimit || '0x0',
          gasUsed: block.gasUsed || '0x0',
          hash: block.hash || '',
          number: block.number || '0x0',
          parentHash: block.parentHash || '',
          receiptsRoot: block.receiptsRoot || '',
          stateRoot: block.stateRoot || '',
          timestamp: block.timestamp || '0x0',
          transactions: block.transactions || [],
          transactionsRoot: block.transactionsRoot || '',
          extraData: block.extraData || '',
          logsBloom: block.logsBloom || '',
          miner: block.miner || '',
          size: block.size || '0x0',
          prevRandao: block.prevRandao || '',
          withdrawals: block.withdrawals || [],
          withdrawalsRoot: block.withdrawalsRoot || '',
        });
        setError(null);
      } catch (err) {
        console.error('Error fetching block:', err);
        setError('Failed to load block details');
      } finally {
        setLoading(false);
      }
    };

    if (blockNumber) fetchBlock();
  }, [blockNumber]);

  if (loading) {
    return (
      <div className="flex justify-center items-center min-h-[400px]">
        <div className="animate-spin rounded-full h-10 w-10 border-t-2 border-b-2 border-[#ffa729]"></div>
      </div>
    );
  }

  if (error || !blockData) {
    return (
      <div className="p-4 sm:p-8 max-w-6xl mx-auto">
        <div className="bg-red-900/50 border border-red-500 text-red-200 px-4 py-3 rounded-xl">
          <p className="font-bold">Error:</p>
          <p className="text-sm">{error || 'Block not found'}</p>
        </div>
      </div>
    );
  }

  const tsNum = typeof blockData.timestamp === 'string' && blockData.timestamp.startsWith('0x')
    ? parseInt(blockData.timestamp, 16)
    : parseInt(blockData.timestamp);

  return (
    <div className="p-4 sm:p-8 max-w-6xl mx-auto">
      <Breadcrumbs items={[
        { label: 'Blocks', href: '/blocks/1' },
        { label: `Block #${blockNumber}` },
      ]} />

      <Suspense>
        <BackToBlocksLink />
      </Suspense>

      {/* Block Details Card */}
      <div className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-xl overflow-hidden mb-6">
        {/* Header */}
        <div className="flex items-center justify-between p-4 sm:p-6 border-b border-[#3d3d3d]">
          <div className="flex items-center gap-3">
            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.5}>
              <path strokeLinecap="round" strokeLinejoin="round" d="m21 7.5-9-5.25L3 7.5m18 0-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9" />
            </svg>
            <h1 className="text-xl sm:text-2xl font-bold text-[#ffa729]">Block #{formatHexValue(blockData.number)}</h1>
          </div>
          <div className="flex items-center gap-2">
            {blockNum > 0 && (
              <Link
                href={`/block/${blockNum - 1}`}
                className="px-3 py-1.5 rounded-lg bg-[#1e1e1e] border border-[#3d3d3d] text-gray-300 text-sm hover:border-[#ffa729] transition-colors"
              >
                &larr;
              </Link>
            )}
            <Link
              href={`/block/${blockNum + 1}`}
              className="px-3 py-1.5 rounded-lg bg-[#1e1e1e] border border-[#3d3d3d] text-gray-300 text-sm hover:border-[#ffa729] transition-colors"
            >
              &rarr;
            </Link>
          </div>
        </div>

        {/* Content */}
        <div className="p-4 sm:p-6">
          <DetailRow label="Block Hash" mono>
            <div className="flex items-start gap-2">
              <span>{blockData.hash}</span>
              <CopyButton value={blockData.hash} label="Copy hash" size="sm" />
            </div>
          </DetailRow>
          <DetailRow label="Parent Hash" mono>
            <Link
              href={`/block/${blockNum - 1}`}
              className="text-gray-300 hover:text-[#ffa729] transition-colors break-all"
            >
              {blockData.parentHash}
            </Link>
          </DetailRow>
          <DetailRow label="Timestamp">
            {timeAgo(tsNum)}
            <span className="text-gray-500 ml-2">({formatTimestampUTC(blockData.timestamp)})</span>
          </DetailRow>
          <DetailRow label="Transactions">
            {blockData.transactions?.length ?? 0}
          </DetailRow>
          <DetailRow label="Gas Used">{formatHexValue(blockData.gasUsed)}</DetailRow>
          <DetailRow label="Gas Limit">{formatHexValue(blockData.gasLimit)}</DetailRow>
          <DetailRow label="Base Fee">{formatHexValue(blockData.baseFeePerGas)} Shor</DetailRow>
          {blockData.prevRandao && (
            <DetailRow label="Prev Randao" mono>{blockData.prevRandao}</DetailRow>
          )}
          <DetailRow label="State Root" mono>{blockData.stateRoot}</DetailRow>
          <DetailRow label="Receipts Root" mono>{blockData.receiptsRoot}</DetailRow>
          {blockData.extraData && blockData.extraData !== '0x' && (
            <DetailRow label="Extra Data" mono>{blockData.extraData}</DetailRow>
          )}
        </div>
      </div>

      {/* Transactions Table */}
      <div className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-xl overflow-hidden">
        <div className="px-4 sm:px-6 py-4 border-b border-[#3d3d3d]">
          <h2 className="text-[15px] font-semibold text-[#ffa729]">
            Transactions ({blockData.transactions?.length ?? 0})
          </h2>
        </div>

        {blockData.transactions && blockData.transactions.length > 0 ? (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-[#3d3d3d]/50">
                  <th className="text-left px-4 sm:px-6 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Hash</th>
                  <th className="text-left px-4 sm:px-6 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden sm:table-cell">From</th>
                  <th className="text-left px-4 sm:px-6 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden sm:table-cell">To</th>
                  <th className="text-left px-4 sm:px-6 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Value</th>
                </tr>
              </thead>
              <tbody>
                {blockData.transactions.map((tx) => {
                  const [amount, unit] = formatAmount(tx.value);
                  return (
                    <tr
                      key={tx.hash}
                      className="border-b border-[#3d3d3d]/30 last:border-b-0 hover:bg-[#353535] transition-colors"
                    >
                      <td className="px-4 sm:px-6 py-3">
                        <Link
                          href={`/tx/${tx.hash}`}
                          className="text-[#ffa729] hover:text-[#ffb954] hover:underline font-mono text-xs"
                          title={tx.hash}
                        >
                          {truncateHash(tx.hash, 10, 6)}
                        </Link>
                      </td>
                      <td className="px-4 sm:px-6 py-3 hidden sm:table-cell">
                        <Link
                          href={`/address/${tx.from}`}
                          className="text-gray-400 hover:text-[#ffa729] font-mono text-xs transition-colors"
                          title={tx.from}
                        >
                          {truncateHash(tx.from, 8, 6)}
                        </Link>
                      </td>
                      <td className="px-4 sm:px-6 py-3 hidden sm:table-cell">
                        <Link
                          href={`/address/${tx.to}`}
                          className="text-gray-400 hover:text-[#ffa729] font-mono text-xs transition-colors"
                          title={tx.to}
                        >
                          {truncateHash(tx.to, 8, 6)}
                        </Link>
                      </td>
                      <td className="px-4 sm:px-6 py-3 text-gray-300 tabular-nums whitespace-nowrap">
                        {amount}
                        <span className="text-gray-500 text-xs ml-1">{unit}</span>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        ) : (
          <EmptyState title="No transactions in this block" description="This block was produced without any transactions." />
        )}
      </div>
    </div>
  );
}
