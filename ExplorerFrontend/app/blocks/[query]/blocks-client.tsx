'use client';

import React from 'react';
import Link from 'next/link';
import axios from 'axios';
import config from '../../../config';
import { useQuery } from '@tanstack/react-query';
import type { BlocksResponse } from '@/app/types';
import { useRouter } from 'next/navigation';
import { timeAgo, truncateHash } from '../../lib/helpers';
import SearchBar from '../../components/SearchBar';
import Pagination from '../../components/Pagination';

const ITEMS_PER_PAGE = 10;

const fetchBlocks = async (page: string): Promise<BlocksResponse> => {
  const response = await axios.get<BlocksResponse>(`${config.handlerUrl}/blocks?page=${page}&limit=${ITEMS_PER_PAGE}`);
  return response.data;
};

const formatBlockNumber = (number: string | number): number => {
  if (typeof number === 'string' && number.startsWith('0x')) {
    return parseInt(number, 16);
  }
  return typeof number === 'number' ? number : parseInt(number);
};

interface BlocksClientProps {
  initialData: BlocksResponse;
  initialPage: string;
}

export default function BlocksClient({ initialData, initialPage }: BlocksClientProps): JSX.Element {
  const router = useRouter();
  const currentPage = parseInt(initialPage);

  // Page-1 is the chain head; refresh on roughly block time so new blocks
  // appear without a manual reload. Later pages are historical and stay
  // static so users mid-scroll on /blocks/57 don't get the rug pulled.
  const isHead = initialPage === '1';
  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['blocks', initialPage],
    queryFn: () => fetchBlocks(initialPage),
    staleTime: isHead ? 0 : 60000,
    gcTime: 5 * 60 * 1000,
    retry: 2,
    initialData,
    refetchInterval: isHead ? 15000 : false,
    refetchIntervalInBackground: false,
  });

  const totalPages = data ? Math.min(Math.ceil(data.total / ITEMS_PER_PAGE), 300) : 300;

  const goToNextPage = (): void => {
    router.push(`/blocks/${Math.min(currentPage + 1, totalPages)}`);
  };

  const goToPreviousPage = (): void => {
    router.push(`/blocks/${Math.max(currentPage - 1, 1)}`);
  };

  return (
    <div className="p-4 sm:p-8 max-w-6xl mx-auto">
      <h1 className="text-xl sm:text-2xl font-bold mb-4 text-[#ffa729]">Blocks</h1>

      <div className="mb-6">
        <SearchBar />
      </div>

      <div className="rounded-xl bg-[#1e1e1e] border border-[#2a2a2a] overflow-hidden mb-6">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-[#2a2a2a]">
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Block</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden sm:table-cell">Hash</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Txns</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden lg:table-cell">Activity</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Time</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden md:table-cell">Gas Used</th>
              </tr>
            </thead>
            <tbody>
              {isLoading ? (
                Array.from({ length: ITEMS_PER_PAGE }).map((_, i) => (
                  <tr key={i} className="border-b border-[#2a2a2a] last:border-b-0">
                    <td className="px-4 py-3"><div className="h-4 w-16 bg-[#2a2a2a] rounded animate-pulse" /></td>
                    <td className="px-4 py-3 hidden sm:table-cell"><div className="h-4 w-24 bg-[#2a2a2a] rounded animate-pulse" /></td>
                    <td className="px-4 py-3"><div className="h-4 w-8 bg-[#2a2a2a] rounded animate-pulse" /></td>
                    <td className="px-4 py-3 hidden lg:table-cell"><div className="h-4 w-20 bg-[#2a2a2a] rounded animate-pulse" /></td>
                    <td className="px-4 py-3"><div className="h-4 w-16 bg-[#2a2a2a] rounded animate-pulse" /></td>
                    <td className="px-4 py-3 hidden md:table-cell"><div className="h-4 w-16 bg-[#2a2a2a] rounded animate-pulse" /></td>
                  </tr>
                ))
              ) : isError ? (
                <tr>
                  <td colSpan={6} className="px-4 py-12 text-center text-red-400">
                    {error instanceof Error ? error.message : 'Failed to load blocks'}
                  </td>
                </tr>
              ) : !data?.blocks?.length ? (
                <tr>
                  <td colSpan={6} className="px-4 py-12 text-center text-gray-500">
                    No blocks found.
                  </td>
                </tr>
              ) : (
                data.blocks.map((block) => {
                  const blockNum = formatBlockNumber(block.number);
                  // blockActivity is keyed by block.number string verbatim
                  // (typically "0x..."). Look up by the original key so the
                  // join survives without re-encoding.
                  const activity = data.blockActivity?.[String(block.number)];
                  return (
                    <tr
                      key={block.number}
                      className="border-b border-[#2a2a2a] last:border-b-0 hover:bg-[#252525] transition-colors"
                    >
                      <td className="px-4 py-3">
                        <Link
                          href={`/block/${blockNum}?from=blocks&page=${currentPage}`}
                          className="text-[#ffa729] hover:text-[#ffb954] hover:underline font-medium tabular-nums"
                        >
                          {blockNum.toLocaleString()}
                        </Link>
                      </td>
                      <td className="px-4 py-3 hidden sm:table-cell">
                        <span className="text-gray-400 font-mono text-xs" title={block.hash}>
                          {truncateHash(block.hash, 10, 6)}
                        </span>
                      </td>
                      <td className="px-4 py-3 text-gray-300 tabular-nums">
                        {block.transactions?.length ?? 0}
                      </td>
                      <td className="px-4 py-3 hidden lg:table-cell whitespace-nowrap">
                        {activity ? (
                          <span className="text-xs font-mono flex items-center gap-2">
                            {activity.tokenTransfers > 0 && (
                              <span className="text-[#ffa729]" title="Token / NFT transfers across this block's txs">
                                {activity.tokenTransfers} token{activity.tokenTransfers === 1 ? '' : 's'}
                              </span>
                            )}
                            {activity.internalCalls > 0 && (
                              <span className="text-gray-400" title="Internal contract calls across this block's txs">
                                {activity.internalCalls} call{activity.internalCalls === 1 ? '' : 's'}
                              </span>
                            )}
                          </span>
                        ) : (
                          <span className="text-gray-600 text-xs">-</span>
                        )}
                      </td>
                      <td className="px-4 py-3 text-gray-400 tabular-nums">
                        {timeAgo(block.timestamp)}
                      </td>
                      <td className="px-4 py-3 text-gray-400 tabular-nums hidden md:table-cell">
                        {block.gasUsed ? parseInt(block.gasUsed, 16).toLocaleString() : '0'}
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </div>

      <Pagination
        currentPage={currentPage}
        totalPages={totalPages}
        onPrevious={goToPreviousPage}
        onNext={goToNextPage}
      />
    </div>
  );
}
