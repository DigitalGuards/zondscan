"use client";

import React from 'react';
import axios from 'axios';
import config from '../../../config';
import { useQuery } from '@tanstack/react-query';
import { Block, BlocksResponse } from './types';
import { useRouter } from 'next/navigation';
import SearchBar from '../../components/SearchBar';

interface BlockCardProps {
  blockData: Block;
}

const BlockCard: React.FC<BlockCardProps> = ({ blockData }) => {
  // Format date in a consistent way that matches server-side rendering
  const formatDate = (timestamp: number) => {
    const date = new Date(timestamp * 1000);
    return new Intl.DateTimeFormat('en-US', {
      year: 'numeric',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      hour12: false
    }).format(date);
  };

  // Truncate hash to show first 6 and last 4 characters
  const truncateHash = (hash: string) => {
    if (!hash) return '';
    return `${hash.substring(0, 6)}...${hash.substring(hash.length - 4)}`;
  };

  return (
    <a 
      href={`/block/${blockData.number}`}
      className='relative overflow-hidden rounded-xl 
                bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                border border-[#3d3d3d] shadow-xl
                hover:border-[#ffa729] transition-all duration-300
                group mb-3 sm:mb-4 block'
    >
      <div className="p-4 sm:p-6">
        <div className="flex flex-col sm:flex-row items-start sm:items-center">
          {/* Left Section - Icon and Status */}
          <div className="flex flex-row sm:flex-col items-center mb-3 sm:mb-0 sm:w-48 w-full justify-between">
            <div className="flex items-center">
              <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6 sm:w-8 sm:h-8 text-[#ffa729] group-hover:scale-110 transition-transform duration-300">
                <path strokeLinecap="round" strokeLinejoin="round" d="m21 7.5-9-5.25L3 7.5m18 0-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9" />
              </svg>
              <span className="text-gray-300 ml-2 sm:hidden">Block #{blockData.number}</span>
            </div>
            <div className="flex flex-col items-end sm:items-center">
              <p className="text-gray-300 mt-0 sm:mt-2 text-sm sm:text-base">Confirmed</p>
              <p className="text-gray-400 text-xs sm:text-sm">{formatDate(blockData.timestamp)}</p>
            </div>
          </div>

          {/* Right Section - Block Info */}
          <div className="flex-1 sm:ml-8 space-y-2 w-full">
            <h2 className="text-lg font-semibold text-[#ffa729] group-hover:scale-105 transition-transform duration-300 hidden sm:block">
              Block #{blockData.number}
            </h2>
            <div className="flex items-center text-gray-400">
              <span className="sm:inline hidden">Hash: </span>
              <span className="text-gray-300 font-mono text-sm sm:text-base" title={blockData.hash}>
                {truncateHash(blockData.hash)}
              </span>
            </div>
            {blockData.transactions && (
              <p className="text-gray-400 text-sm sm:text-base">
                Transactions: <span className="text-gray-300">{blockData.transactions.length}</span>
              </p>
            )}
          </div>
        </div>
      </div>
    </a>
  );
};

const fetchBlocks = async (page: string) => {
  const response = await axios.get<BlocksResponse>(`${config.handlerUrl}/blocks?page=${page}&limit=5`);
  return response.data;
};

interface BlocksClientProps {
  initialData: BlocksResponse;
  initialPage: string;
}

export default function BlocksClient({ initialData, initialPage }: BlocksClientProps) {
  const router = useRouter();
  const currentPage = parseInt(initialPage);

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['blocks', initialPage],
    queryFn: () => fetchBlocks(initialPage),
    staleTime: 60000, // Consider data fresh for 60 seconds (increased from 30s)
    gcTime: 5 * 60 * 1000, // Keep unused data in cache for 5 minutes
    retry: 2,
    initialData // Use server-fetched data as initial data
  });

  // Limit total pages to 300 and calculate based on 5 blocks per page
  const totalPages = data ? Math.min(Math.round(data.total / 5), 300) : 300;

  const goToNextPage = () => {
    const nextPage = Math.min(currentPage + 1, totalPages);
    router.push(`/blocks/${nextPage}`);
  };

  const goToPreviousPage = () => {
    const prevPage = Math.max(currentPage - 1, 1);
    router.push(`/blocks/${prevPage}`);
  };

  if (isLoading) {
    return (
      <div className="p-4 sm:p-8">
        <h1 className="text-xl sm:text-2xl font-bold mb-4 sm:mb-6 text-[#ffa729]">Latest Synced Blocks</h1>
        <div className="space-y-3 sm:space-y-4">
          {[...Array(5)].map((_, i) => (
            <div 
              key={i}
              className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] p-4 sm:p-6 animate-pulse"
            >
              <div className="flex flex-col sm:flex-row items-start sm:items-center">
                <div className="w-full sm:w-48 flex flex-row sm:flex-col items-center justify-between sm:justify-center">
                  <div className="w-6 h-6 sm:w-8 sm:h-8 bg-gray-700 rounded-lg mb-0 sm:mb-2"></div>
                  <div className="h-4 w-20 bg-gray-700 rounded"></div>
                </div>
                <div className="flex-1 sm:ml-8 space-y-2 mt-3 sm:mt-0">
                  <div className="h-5 sm:h-6 w-32 bg-gray-700 rounded"></div>
                  <div className="h-4 w-32 bg-gray-700 rounded"></div>
                </div>
              </div>
            </div>
          ))}
        </div>
      </div>
    );
  }

  if (isError) {
    return (
      <div className="p-4 sm:p-8">
        <h1 className="text-xl sm:text-2xl font-bold mb-4 sm:mb-6 text-[#ffa729]">Latest Synced Blocks</h1>
        <div className="bg-red-900/50 border border-red-500 text-red-200 px-4 sm:px-6 py-3 sm:py-4 rounded-xl">
          <p className="font-bold">Error:</p>
          <p className="text-sm sm:text-base">{error instanceof Error ? error.message : 'Failed to load blocks'}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-4 sm:p-8">
      <h1 className="text-xl sm:text-2xl font-bold mb-4 sm:mb-6 text-[#ffa729]">Latest Synced Blocks</h1>
      
      <div className="max-w-[1200px] mx-auto mb-8">
        <SearchBar />
      </div>

      <div className="mb-6 sm:mb-8">
        {data?.blocks.map((blockData) => (
          <BlockCard key={blockData.number} blockData={blockData} />
        ))}
      </div>
      <div className="flex justify-center items-center gap-2 sm:gap-4 text-sm sm:text-base text-gray-300">
        <button 
          onClick={goToPreviousPage} 
          disabled={currentPage === 1} 
          className="px-3 sm:px-4 py-2 rounded-lg bg-[#2d2d2d] text-gray-300 border border-[#3d3d3d]
                   hover:border-[#ffa729] disabled:opacity-50 disabled:hover:border-[#3d3d3d]
                   transition-colors text-sm sm:text-base"
        >
          Previous
        </button>
        <span className="text-sm sm:text-base">Page {currentPage} of {totalPages}</span>
        <button 
          onClick={goToNextPage} 
          disabled={currentPage === totalPages} 
          className="px-3 sm:px-4 py-2 rounded-lg bg-[#2d2d2d] text-gray-300 border border-[#3d3d3d]
                   hover:border-[#ffa729] disabled:opacity-50 disabled:hover:border-[#3d3d3d]
                   transition-colors text-sm sm:text-base"
        >
          Next
        </button>
      </div>
    </div>
  );
}
