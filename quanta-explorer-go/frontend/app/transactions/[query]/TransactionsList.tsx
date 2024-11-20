"use client";

import React, { useState } from 'react';
import Link from 'next/link';
import { useRouter } from 'next/navigation';
import { decodeBase64ToHexadecimal, toFixed } from "../../lib/helpers";
import { Transaction, TransactionsListProps } from './types';

const formatDate = (timestamp: number) => {
  const date = new Date(timestamp * 1000);
  const year = date.getFullYear();
  const month = String(date.getMonth() + 1).padStart(2, '0');
  const day = String(date.getDate()).padStart(2, '0');
  const hours = String(date.getHours()).padStart(2, '0');
  const minutes = String(date.getMinutes()).padStart(2, '0');
  const seconds = String(date.getSeconds()).padStart(2, '0');
  
  return `${year}-${month}-${day} ${hours}:${minutes}:${seconds}`;
};

const ReceiveIcon = () => (
  <svg width="73px" height="73px" viewBox="0 0 73 73" version="1.1" xmlns="http://www.w3.org/2000/svg">
    <title>WallettTansactionSendReceive</title>
    <g id="Page-1" stroke="none" strokeWidth="1" fill="none" fillRule="evenodd">
      <g id="WallettTansactionSendReceive" transform="translate(0.000000, 0.000000)">
        <polygon id="Path" fill="#6EBFFF" fillRule="nonzero" points="31 13 41 13 41 17 31 17"></polygon>
        <polygon id="Path" fill="#6EBFFF" fillRule="nonzero" points="57 33 61 33 61 43 57 43"></polygon>
        <polygon id="Path" fill="#6EBFFF" fillRule="nonzero" points="57.3576159 13 51 13 51 16.6184211 57.3576159 16.6184211 57.3576159 23 61 23 61 13"></polygon>
        <polygon id="Path" fill="#6EBFFF" fillRule="nonzero" points="31 59 41 59 41 63 31 63"></polygon>
        <polygon id="Path" fill="#6EBFFF" fillRule="nonzero" points="57.3576159 52 57.3576159 58.3576159 51 58.3576159 51 62 61 62 61 52"></polygon>
        <polygon id="Path" fill="#6EBFFF" fillRule="nonzero" points="11 33 15 33 15 43 11 43"></polygon>
        <polygon id="Path" fill="#6EBFFF" fillRule="nonzero" points="20.9342105 13 11 13 11 23 14.6184211 23 14.6184211 16.5761589 21 16.5761589 21 13"></polygon>
        <polygon id="Path" fill="#6EBFFF" fillRule="nonzero" points="14.5761589 58.4238411 14.5761589 52 11 52 11 62 21 62 21 58.3576159 14.5761589 58.3576159"></polygon>
        <polygon id="Path" fill="#6EBFFF" fillRule="nonzero" points="41.7037037 37.1035422 41.7037037 26 30.2962963 26 30.2962963 37.1035422 25 37.1035422 36 51 47 37.1035422"></polygon>
        <rect id="Rectangle" x="5.68434189e-14" y="0" width="73" height="73"></rect>
      </g>
    </g>
  </svg>
);

const SendIcon = () => (
  <svg width="73px" height="73px" viewBox="0 0 73 73" version="1.1" xmlns="http://www.w3.org/2000/svg">
    <title>WallettTansactionSendReceive Copy 2</title>
    <g id="Page-1" stroke="none" strokeWidth="1" fill="none" fillRule="evenodd">
      <g id="WallettTansactionSendReceive-Copy-2" transform="translate(36.500000, 36.500000) rotate(180.000000) translate(-36.500000, -36.500000) ">
        <polygon id="Path" fill="#FFB954" fillRule="nonzero" points="31 13 41 13 41 17 31 17"></polygon>
        <polygon id="Path" fill="#FFB954" fillRule="nonzero" points="57 33 61 33 61 43 57 43"></polygon>
        <polygon id="Path" fill="#FFB954" fillRule="nonzero" points="57.3576159 13 51 13 51 16.6184211 57.3576159 16.6184211 57.3576159 23 61 23 61 13"></polygon>
        <polygon id="Path" fill="#FFB954" fillRule="nonzero" points="31 59 41 59 41 63 31 63"></polygon>
        <polygon id="Path" fill="#FFB954" fillRule="nonzero" points="57.3576159 52 57.3576159 58.3576159 51 58.3576159 51 62 61 62 61 52"></polygon>
        <polygon id="Path" fill="#FFB954" fillRule="nonzero" points="11 33 15 33 15 43 11 43"></polygon>
        <polygon id="Path" fill="#FFB954" fillRule="nonzero" points="20.9342105 13 11 13 11 23 14.6184211 23 14.6184211 16.5761589 21 16.5761589 21 13"></polygon>
        <polygon id="Path" fill="#FFB954" fillRule="nonzero" points="14.5761589 58.4238411 14.5761589 52 11 52 11 62 21 62 21 58.3576159 14.5761589 58.3576159"></polygon>
        <polygon id="Path" fill="#FFA729" fillRule="nonzero" points="41.7037037 37.1035422 41.7037037 26 30.2962963 26 30.2962963 37.1035422 25 37.1035422 36 51 47 37.1035422"></polygon>
        <rect id="Rectangle" x="5.68434189e-14" y="0" width="73" height="73"></rect>
      </g>
    </g>
  </svg>
);

interface TransactionCardProps {
  transaction: Transaction;
}

const TransactionCard = ({ transaction }: TransactionCardProps) => {
  const isSending = transaction.InOut === 0; 
  const date = formatDate(transaction.TimeStamp);

  return (
    <div className='flex-container border bg-gray-100 border p-4 rounded-lg mb-2 hover:bg-gray-200 hover:shadow-lg transition duration-75 ease-in-out'>
      <div className="flex flex-item-left items-center flex-col ml-4">
        {isSending ? <SendIcon /> : <ReceiveIcon />}
        <p className="text-lg font-semibold">Transfer</p>
        <p>Confirmed</p>
        <p>{date}</p>
      </div>
      <div className="flex-item-center flex-1">
        <p className="text-lg font-semibold">Hash</p>
        <p className="text-sm"><Link href={`/tx/${"0x" + decodeBase64ToHexadecimal(transaction.TxHash)}`}>{"0x" + decodeBase64ToHexadecimal(transaction.TxHash)}</Link></p>
      </div>
      <div className="flex-item-right flex-1">
        <p className="text-lg font-semibold">Quanta</p>
        <p className="text-lg font-medium">{toFixed(transaction.Amount)}</p>
      </div>
    </div>
  );
};

export default function TransactionsList({ initialData, currentPage }: TransactionsListProps) {
  const router = useRouter();
  const [transactions] = useState(initialData.txs);
  const [totalPages] = useState(Math.max(1, Math.round(initialData.total / 15)));

  const navigateToPage = (page: number) => {
    window.location.href = `/transactions/${page}`;
  };

  const goToNextPage = () => {
    const nextPage = Math.min(currentPage + 1, totalPages);
    navigateToPage(nextPage);
  };

  const goToPreviousPage = () => {
    const prevPage = Math.max(currentPage - 1, 1);
    navigateToPage(prevPage);
  };

  return (
    <div className="container mx-auto p-4">
      {transactions.length === 0 ? (
        <div className="text-center py-4">
          <p>No transactions found</p>
        </div>
      ) : (
        <>
          <div className="mb-4">
            {transactions.map(transaction => (
              <TransactionCard 
                key={transaction.TxHash} 
                transaction={transaction} 
              />
            ))}
          </div>
          <div className="flex justify-center items-center">
            <button 
              onClick={goToPreviousPage} 
              disabled={currentPage === 1} 
              className={`px-4 py-2 rounded mr-2 md:px-6 md:py-3 ${currentPage === 1 ? 'bg-gray-400' : 'bg-blue-500 text-white'}`}
            >
              Previous
            </button>
            <span>Page {currentPage} of {totalPages}</span>
            <button 
              onClick={goToNextPage} 
              disabled={currentPage === totalPages} 
              className={`px-4 py-2 rounded ml-2 ${currentPage === totalPages ? 'bg-gray-400' : 'bg-blue-500 text-white'}`}
            >
              Next
            </button>
          </div>
        </>
      )}
    </div>
  );
}
