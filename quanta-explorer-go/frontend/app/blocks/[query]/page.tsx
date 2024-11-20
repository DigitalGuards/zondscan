"use client";

import React, { useState, useEffect } from 'react';
import axios from 'axios';
import config from '../../../config';
import Image from 'next/image';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { Block, BlocksResponse } from './types';

interface BlockCardProps {
  blockData: Block;
}

const BlockCard: React.FC<BlockCardProps> = ({ blockData }) => {
  const date = new Date(blockData.timestamp * 1000).toLocaleString();

  return (
    <div className='flex flex-col md:flex-row border bg-[#2d2d2d] p-4 rounded-lg mb-2 hover:bg-[#3d3d3d] transition duration-75 ease-in-out'>
      <div className="flex items-center flex-col md:ml-4 mb-4 md:mb-0 md:w-48">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6 text-[#ffa729]">
          <path strokeLinecap="round" strokeLinejoin="round" d="m21 7.5-9-5.25L3 7.5m18 0-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9" />
        </svg>
        <p className="text-gray-300 mt-2">Confirmed</p>
        <p className="text-gray-400 text-sm">{date}</p>
      </div>
      <div className="flex-1 text-center md:text-left md:max-w-md">
        <p className="text-lg font-semibold text-[#ffa729]">Block Number</p>
        <Link href={`/block/${blockData.number}`} className="text-gray-300 hover:text-[#ffa729] transition-colors">
          {blockData.number}
        </Link>
      </div>
    </div>
  );
};

export default function BlocksList({ params }: { params: { query: string } }) {
  const router = useRouter();
  const [blocksData, setBlocksData] = useState<Block[]>([]);
  const [totalPages, setTotalPages] = useState(10);
  const [currentPage, setCurrentPage] = useState(parseInt(params.query));
  
  useEffect(() => {
    axios.get<BlocksResponse>(config.handlerUrl + `/blocks?page=${currentPage}`)
      .then(response => {
        setBlocksData(response.data.blocks);
        setTotalPages(Math.round(response.data.total / 15))
      })
      .catch(error => console.error('Error fetching blocks:', error));
  }, [currentPage]);

  const goToNextPage = () => {
    const nextPage = Math.min(currentPage + 1, totalPages);
    setCurrentPage(nextPage);
    router.push(`/blocks/${nextPage}`);
  };

  const goToPreviousPage = () => {
    const prevPage = Math.max(currentPage - 1, 1);
    setCurrentPage(prevPage);
    router.push(`/blocks/${prevPage}`);
  };

  return (
    <div className="p-4 max-w-3xl">
      <h1 className="text-2xl font-bold mb-6 text-[#ffa729]">Blocks</h1>
      <div className="mb-4">
        {blocksData.map((blockData) => (
          <BlockCard key={blockData.number} blockData={blockData} />
        ))}
      </div>
      <div className="flex justify-center items-center gap-4 text-gray-300">
        <button 
          onClick={goToPreviousPage} 
          disabled={currentPage === 1} 
          className="bg-[#2d2d2d] px-4 py-2 rounded hover:bg-[#3d3d3d] disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          Previous
        </button>
        <span>Page {currentPage} of {totalPages}</span>
        <button 
          onClick={goToNextPage} 
          disabled={currentPage === totalPages} 
          className="bg-[#2d2d2d] px-4 py-2 rounded hover:bg-[#3d3d3d] disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
        >
          Next
        </button>
      </div>
    </div>
  );
}
