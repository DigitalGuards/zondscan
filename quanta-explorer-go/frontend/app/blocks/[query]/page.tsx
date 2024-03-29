"use client";

import React, { useState, useEffect } from 'react';
import axios from 'axios';
import { decodeBase64ToHexadecimal } from "../../lib/helpers"
import config from '../../../config';
import BlockchainIcon from '../../public/blockchain-icon.svg'
import Link from 'next/link';
import { useRouter } from 'next/navigation';

const TransactionCard = ({ blockData }) => {
  const date = new Date(blockData.timestamp * 1000).toLocaleString();

  return (
    <div className='flex-container border bg-gray-100 border p-4 rounded-lg mb-2 hover:bg-gray-200 hover:shadow-lg transition duration-75 ease-in-out'>
      <div className="flex flex-item-left items-center flex-col ml-4">
        <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor" className="w-6 h-6">
          <path strokeLinecap="round" strokeLinejoin="round" d="m21 7.5-9-5.25L3 7.5m18 0-9 5.25m9-5.25v9l-9 5.25M3 7.5l9 5.25M3 7.5v9l9 5.25m0-9v9" />
        </svg>
        <p>Confirmed</p>
        {date}
      </div>
      <div className="flex-item-center flex-1">
        <p className="text-lg font-semibold">Block Number</p>
        <p className="text-sm"><Link href={`/block/${blockData.number}`}>{blockData.number}</Link></p>
      </div>
    </div>
  );
};

export default function TransactionsList({ params }: { params: { query: string } }) {
  const router = useRouter()
  const [blocksData, setBlocksData] = useState([]);
  const [totalPages, setTotalPages] = useState(10);
  const [currentPage, setCurrentPage] = useState(parseInt(params.query));

  console.log(params.query);
  
  useEffect(() => {
    axios.get(config.handlerUrl + `/blocks?page=${currentPage}`)
      .then(response => {
        setBlocksData(response.data.blocks);
        setTotalPages(Math.round(response.data.total / 15))
      })
      .catch(error => console.error('Error fetching transactions:', error));
  }, [currentPage]);

  const goToNextPage = () => {
    const nextPage = Math.min(parseInt(params.query) + 1, totalPages);
    setCurrentPage(nextPage);
    router.push(`/blocks/${nextPage}`);
  };

  const goToPreviousPage = () => {
    const prevPage = Math.max(parseInt(params.query) - 1, 1);
    setCurrentPage(prevPage);
    router.push(`/blocks/${prevPage}`);
  };

  return (
    <div className="container mx-auto p-4">
      <div className="mb-4">
        {blocksData.map(blockData => (
          <TransactionCard key={blockData} blockData={blockData} />
        ))}
      </div>
      <div className="flex justify-center items-center">
        <button onClick={goToPreviousPage} disabled={currentPage === 1} className="bg-blue-500 text-white px-4 py-2 rounded mr-2 md:px-6 md:py-3">Previous</button>
        <span>Page {currentPage} of {totalPages}</span>
        <button onClick={goToNextPage} disabled={currentPage === totalPages} className="bg-blue-500 text-white px-4 py-2 rounded ml-2">Next</button>
      </div>
    </div>
  );
}
