"use client";

import axios from 'axios';
import React, { useState, useEffect } from 'react';
import config from '../../../config';
import Link from 'next/link';
import { formatAmount } from '../../lib/helpers';

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
  difficulty: string;
  extraData: string;
  logsBloom: string;
  miner: string;
  mixHash: string;
  nonce: string;
  sha3Uncles: string;
  size: string;
  totalDifficulty: string;
  uncles: string[];
  withdrawals: any[];
  withdrawalsRoot: string;
};

interface BlockDetailClientProps {
  blockNumber: string;
}

// Helper function to format hex values
const formatHexValue = (hex: string | null | undefined): string => {
  if (!hex) return '0';
  const num = typeof hex === 'string' && hex.startsWith('0x') ? 
    parseInt(hex, 16) : 
    parseInt(hex);
  return isNaN(num) ? '0' : num.toLocaleString();
};

// Helper function to format timestamp
const formatTimestamp = (timestamp: string | null | undefined): string => {
  if (!timestamp) return 'N/A';
  const timestampNum = typeof timestamp === 'string' && timestamp.startsWith('0x') ? 
    parseInt(timestamp, 16) : 
    parseInt(timestamp);
  if (isNaN(timestampNum)) return 'N/A';
  return new Date(timestampNum * 1000).toLocaleString();
};

export default function BlockDetailClient({ blockNumber }: BlockDetailClientProps) {
  const [blockData, setBlockData] = useState<Block | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    const fetchBlock = async () => {
      try {
        setLoading(true);
        const response = await axios.get(`${config.handlerUrl}/block/${blockNumber}`);
        
        // Extract the block data from the nested response
        const block = response.data?.block?.result;
        if (!block) {
          throw new Error('Invalid block data received');
        }

        // Map the API response to our Block type
        const mappedBlock: Block = {
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
          difficulty: block.difficulty || '0x0',
          extraData: block.extraData || '',
          logsBloom: block.logsBloom || '',
          miner: block.miner || '',
          mixHash: block.mixHash || '',
          nonce: block.nonce || '',
          sha3Uncles: block.sha3Uncles || '',
          size: block.size || '0x0',
          totalDifficulty: block.totalDifficulty || '0x0',
          uncles: block.uncles || [],
          withdrawals: block.withdrawals || [],
          withdrawalsRoot: block.withdrawalsRoot || ''
        };

        setBlockData(mappedBlock);
        setError(null);
      } catch (err) {
        console.error('Error fetching block:', err);
        setError('Failed to load block details');
      } finally {
        setLoading(false);
      }
    };

    if (blockNumber) {
      fetchBlock();
    }
  }, [blockNumber]);

  if (loading) {
    return (
      <div className="flex justify-center items-center min-h-screen">
        <div className="animate-spin rounded-full h-12 w-12 border-t-2 border-b-2 border-[#ffa729]"></div>
      </div>
    );
  }

  if (error) {
    return (
      <div className="p-8">
        <div className="bg-red-900/50 border border-red-500 text-red-200 px-6 py-4 rounded-xl">
          <p className="font-bold">Error:</p>
          <p>{error}</p>
        </div>
      </div>
    );
  }

  if (!blockData) {
    return (
      <div className="p-8">
        <div className="bg-yellow-900/50 border border-yellow-500 text-yellow-200 px-6 py-4 rounded-xl">
          <p>Block not found</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-8">
      <div className="relative overflow-hidden rounded-2xl 
                    bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                    border border-[#3d3d3d] shadow-xl">
        <div className="p-6">
          {/* Header */}
          <div className="flex items-center justify-between mb-8 pb-6 border-b border-gray-700">
            <div className="flex items-center">
              <svg xmlns="http://www.w3.org/2000/svg" className="h-8 w-8 text-[#ffa729] mr-3" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="m21 7.5-9-5.25L3 7.5m18 0-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9" />
              </svg>
              <div>
                <h1 className="text-2xl font-bold text-[#ffa729]">Block #{formatHexValue(blockData?.number)}</h1>
                <p className="text-gray-400 mt-1">
                  {formatTimestamp(blockData?.timestamp)}
                </p>
              </div>
            </div>
          </div>

          {/* Block Details */}
          <div className="space-y-6">
            {/* Basic Info */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">Hash</h2>
                <p className="text-gray-300 break-all font-mono">{blockData?.hash}</p>
              </div>
              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">Parent Hash</h2>
                <Link 
                  href={`/block/${parseInt(blockData?.number || '0', 16) - 1}`}
                  className="text-gray-300 hover:text-[#ffa729] break-all font-mono transition-colors"
                >
                  {blockData?.parentHash}
                </Link>
              </div>
            </div>

            {/* Gas Info */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">Gas Used</h2>
                <p className="text-gray-300">{formatHexValue(blockData?.gasUsed)}</p>
              </div>
              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">Gas Limit</h2>
                <p className="text-gray-300">{formatHexValue(blockData?.gasLimit)}</p>
              </div>
              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">Base Fee</h2>
                <p className="text-gray-300">{formatHexValue(blockData?.baseFeePerGas)}</p>
              </div>
            </div>

            {/* Transactions */}
            <div>
              <h2 className="text-lg font-semibold text-[#ffa729] mb-4">Transactions</h2>
              <div className="space-y-2">
                {blockData?.transactions && blockData.transactions.length > 0 ? (
                  blockData.transactions.map((tx, index) => (
                    <div 
                      key={tx.hash} 
                      className="p-4 rounded-lg bg-[#2d2d2d] border border-[#3d3d3d] hover:border-[#ffa729] transition-colors"
                    >
                      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                        <div>
                          <p className="text-sm text-gray-400">Hash</p>
                          <Link 
                            href={`/tx/${tx.hash}`}
                            className="text-gray-300 hover:text-[#ffa729] break-all font-mono transition-colors"
                          >
                            {tx.hash}
                          </Link>
                        </div>
                        <div>
                          <p className="text-sm text-gray-400">From</p>
                          <Link 
                            href={`/address/${tx.from}`}
                            className="text-gray-300 hover:text-[#ffa729] break-all font-mono transition-colors"
                          >
                            {tx.from}
                          </Link>
                        </div>
                        <div>
                          <p className="text-sm text-gray-400">To</p>
                          <Link 
                            href={`/address/${tx.to}`}
                            className="text-gray-300 hover:text-[#ffa729] break-all font-mono transition-colors"
                          >
                            {tx.to}
                          </Link>
                        </div>
                        <div>
                          <p className="text-sm text-gray-400">Value</p>
                          {(() => {
                            const [amount, unit] = formatAmount(tx.value);
                            return (
                              <p className="text-gray-300">{amount} {unit}</p>
                            );
                          })()}
                        </div>
                      </div>
                    </div>
                  ))
                ) : (
                  <p className="text-gray-400">No transactions in this block</p>
                )}
              </div>
            </div>

            {/* Additional Details */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">State Root</h2>
                <p className="text-gray-300 break-all font-mono">{blockData?.stateRoot}</p>
              </div>
              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">Receipts Root</h2>
                <p className="text-gray-300 break-all font-mono">{blockData?.receiptsRoot}</p>
              </div>
              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">Transactions Root</h2>
                <p className="text-gray-300 break-all font-mono">{blockData?.transactionsRoot}</p>
              </div>
              <div>
                <h2 className="text-sm font-semibold text-gray-400 mb-2">Extra Data</h2>
                <p className="text-gray-300 break-all font-mono">{blockData?.extraData}</p>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
