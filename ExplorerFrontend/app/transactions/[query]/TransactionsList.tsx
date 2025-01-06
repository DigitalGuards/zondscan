'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import TransactionCard from './TransactionCard';
import SearchBar from '../../components/SearchBar';
import type { TransactionsListProps } from './types';

export default function TransactionsList({ 
  initialData, 
  currentPage 
}: TransactionsListProps): JSX.Element {
  const router = useRouter();
  const [transactions] = useState(initialData.txs);
  const [totalPages] = useState(Math.max(1, Math.round(initialData.total / 15)));

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
    <div className="space-y-4">
      <div className="max-w-3xl mx-auto mb-8">
        <SearchBar />
      </div>

      {transactions.length === 0 ? (
        <div className="text-center py-8">
          <p className="text-gray-300">No transactions found</p>
        </div>
      ) : (
        <>
          <div className="mb-8">
            {transactions.map(transaction => (
              <TransactionCard 
                key={transaction.TxHash} 
                transaction={transaction} 
              />
            ))}
          </div>
          
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
        </>
      )}
    </div>
  );
}
