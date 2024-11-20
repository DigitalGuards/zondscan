"use client";

import React, { useState, useEffect } from 'react';
import axios from 'axios';
import config from '../../../config';
import { useRouter } from 'next/navigation';
import { Block, BlocksResponse } from './types';

interface BlockCardProps {
  blockData: Block;
}

const BlockCard: React.FC<BlockCardProps> = ({ blockData }) => {
  const date = new Date(blockData.timestamp * 1000).toLocaleString();

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
          <p className="text-gray-400 text-sm">{date}</p>
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

export default function BlocksList({ params }: { params: { query: string } }) {
  const router = useRouter();
  const [blocksData, setBlocksData] = useState<Block[]>([]);
  const [totalPages, setTotalPages] = useState(10);
  const [currentPage, setCurrentPage] = useState(parseInt(params.query));
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  
  useEffect(() => {
    const fetchBlocks = async () => {
      try {
        setLoading(true);
        const response = await axios.get<BlocksResponse>(config.handlerUrl + `/blocks?page=${currentPage}`);
        setBlocksData(response.data.blocks);
        setTotalPages(Math.round(response.data.total / 15));
        setError(null);
      } catch (err) {
        console.error('Error fetching blocks:', err);
        setError('Failed to load blocks');
      } finally {
        setLoading(false);
      }
    };

    fetchBlocks();
  }, [currentPage]);

  const goToNextPage = () => {
    const nextPage = Math.min(currentPage + 1, totalPages);
    window.location.href = `/blocks/${nextPage}`;
  };

  const goToPreviousPage = () => {
    const prevPage = Math.max(currentPage - 1, 1);
    window.location.href = `/blocks/${prevPage}`;
  };

  if (loading) {
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

  if (error) {
    return (
      <div className="p-8">
        <h1 className="text-2xl font-bold mb-6 text-[#ffa729]">Blocks</h1>
        <div className="bg-red-900/50 border border-red-500 text-red-200 px-6 py-4 rounded-xl">
          <p className="font-bold">Error:</p>
          <p>{error}</p>
        </div>
      </div>
    );
  }

  return (
    <div className="p-8">
      <h1 className="text-2xl font-bold mb-6 text-[#ffa729]">Blocks</h1>
      <div className="mb-8">
        {blocksData.map((blockData) => (
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
