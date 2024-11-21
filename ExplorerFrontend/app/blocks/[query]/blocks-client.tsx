"use client";

import React from 'react';
import axios from 'axios';
import config from '../../../config';
import { useQuery } from '@tanstack/react-query';
import { Block, BlocksResponse } from './types';
import { useRouter } from 'next/navigation';

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

  return (
    <a 
      href={`/block/${blockData.number}`}
      className='relative overflow-hidden rounded-xl 
                bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                border border-[#3d3d3d] shadow-xl
                hover:border-[#ffa729] transition-all duration-300
                group mb-4 block'
    >
      <div className="p-6 flex flex-col md:flex-row items-center">
        {/* Left Section - Icon and Status */}
        <div className="flex items-center flex-col md:ml-4 mb-4 md:mb-0 md:w-48">
          <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-8 h-8 text-[#ffa729] group-hover:scale-110 transition-transform duration-300">
            <path strokeLinecap="round" strokeLinejoin="round" d="m21 7.5-9-5.25L3 7.5m18 0-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9" />
          </svg>
          <p className="text-gray-300 mt-2">Confirmed</p>
          <p className="text-gray-400 text-sm">{formatDate(blockData.timestamp)}</p>
        </div>

        {/* Right Section - Block Info */}
        <div className="flex-1 md:ml-8">
          <div className="space-y-2">
            <h2 className="text-lg font-semibold text-[#ffa729] group-hover:scale-105 transition-transform duration-300">
              Block #{blockData.number}
            </h2>
            <p className="text-gray-400">
              Hash: <span className="text-gray-300 font-mono">{blockData.hash}</span>
            </p>
            {blockData.transactions && (
              <p className="text-gray-400">
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
  const response = await axios.get<BlocksResponse>(`${config.handlerUrl}/blocks?page=${page}`);
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
    staleTime: 30000, // Consider data fresh for 30 seconds
    gcTime: 5 * 60 * 1000, // Keep unused data in cache for 5 minutes
    retry: 2,
    initialData // Use server-fetched data as initial data
  });

  const totalPages = data ? Math.round(data.total / 15) : 10;

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
      <div className="p-8">
        <h1 className="text-2xl font-bold mb-6 text-[#ffa729]">Blocks</h1>
        <div className="space-y-4">
          {[...Array(5)].map((_, i) => (
            <div 
              key={i}
              className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] p-6 animate-pulse"
            >
              <div className="flex flex-col md:flex-row items-center">
                <div className="w-48 flex flex-col items-center">
                  <div className="w-8 h-8 bg-gray-700 rounded-lg mb-2"></div>
                  <div className="h-4 w-20 bg-gray-700 rounded"></div>
                </div>
                <div className="flex-1 md:ml-8 space-y-2">
                  <div className="h-6 w-32 bg-gray-700 rounded"></div>
                  <div className="h-4 w-full bg-gray-700 rounded"></div>
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
      <div className="p-8">
        <h1 className="text-2xl font-bold mb-6 text-[#ffa729]">Blocks</h1>
        <div className="bg-red-900/50 border border-red-500 text-red-200 px-6 py-4 rounded-xl">
          <p className="font-bold">Error:</p>
          <p>{error instanceof Error ? error.message : 'Failed to load blocks'}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-8">
      <h1 className="text-2xl font-bold mb-6 text-[#ffa729]">Blocks</h1>
      <div className="mb-8">
        {data?.blocks.map((blockData) => (
          <BlockCard key={blockData.number} blockData={blockData} />
        ))}
      </div>
      <div className="flex justify-center items-center gap-4 text-gray-300">
        <button 
          onClick={goToPreviousPage} 
          disabled={currentPage === 1} 
          className="px-4 py-2 rounded-lg bg-[#2d2d2d] text-gray-300 border border-[#3d3d3d]
                   hover:border-[#ffa729] disabled:opacity-50 disabled:hover:border-[#3d3d3d]
                   transition-colors"
        >
          Previous
        </button>
        <span>Page {currentPage} of {totalPages}</span>
        <button 
          onClick={goToNextPage} 
          disabled={currentPage === totalPages} 
          className="px-4 py-2 rounded-lg bg-[#2d2d2d] text-gray-300 border border-[#3d3d3d]
                   hover:border-[#ffa729] disabled:opacity-50 disabled:hover:border-[#3d3d3d]
                   transition-colors"
        >
          Next
        </button>
      </div>
    </div>
  );
}
