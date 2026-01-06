'use client';

import { useState, useEffect } from 'react';
import { useRouter } from 'next/navigation';
import TransactionCard from './TransactionCard';
import SearchBar from '../../components/SearchBar';
import type { TransactionsListProps } from '@/app/types';

export default function TransactionsList({ 
  initialData, 
  currentPage 
}: TransactionsListProps): JSX.Element {
  const router = useRouter();
  const [transactions, setTransactions] = useState(initialData.txs);
  const ITEMS_PER_PAGE = 5;
  const [totalPages] = useState(Math.max(1, Math.ceil(initialData.total / ITEMS_PER_PAGE)));

  useEffect(() => {
    console.log(`Total transactions: ${initialData.total}, Items per page: ${ITEMS_PER_PAGE}, Total pages: ${Math.ceil(initialData.total / ITEMS_PER_PAGE)}`);
  }, [initialData.total]);

  useEffect(() => {
    setTransactions(initialData.txs);
  }, [initialData]);

  const navigateToPage = (page: number): void => {
    router.push(`/transactions/${page}`);
  };

  const goToNextPage = (): void => {
    const nextPage = Math.min(currentPage + 1, totalPages);
    navigateToPage(nextPage);
  };

  const goToPreviousPage = (): void => {
    const prevPage = Math.max(currentPage - 1, 1);
    navigateToPage(prevPage);
  };

  return (
    <div className="space-y-4 px-4 sm:px-6 lg:px-8 mt-4 sm:mt-6">
      <div className="max-w-[900px] mx-auto mb-6 sm:mb-8">
        <SearchBar />
      </div>
      
      <div className="max-w-[900px] mx-auto mb-4">
        <h2 className="text-lg font-medium text-[#ffa729]">Latest Transactions</h2>
      </div>

      {transactions.length === 0 ? (
        <div className="text-center py-8">
          <p className="text-gray-300">No transactions found</p>
        </div>
      ) : (
        <div className="max-w-[900px] mx-auto mb-8">
          {transactions.map(transaction => (
            <TransactionCard
              key={transaction.TxHash}
              transaction={transaction}
              currentPage={currentPage}
            />
          ))}
        </div>
      )}
      
      <div className="flex justify-center items-center gap-4 text-gray-300">
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
