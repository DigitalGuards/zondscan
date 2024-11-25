'use client';

import { useState } from 'react';
import { useRouter } from 'next/navigation';
import TransactionCard from './TransactionCard';
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
    <div className="container mx-auto p-4">
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
              className={`px-6 py-3 rounded-xl
                         ${currentPage === 1 
                           ? 'bg-gray-700 text-gray-500 cursor-not-allowed' 
                           : 'bg-[#2d2d2d] hover:bg-[#3d3d3d] text-[#ffa729] hover:text-[#ffb954]'} 
                         transition-all duration-300`}
              aria-label="Previous page"
            >
              Previous
            </button>
            
            <span className="px-4" aria-live="polite">
              Page {currentPage} of {totalPages}
            </span>
            
            <button 
              onClick={goToNextPage} 
              disabled={currentPage === totalPages} 
              className={`px-6 py-3 rounded-xl
                         ${currentPage === totalPages 
                           ? 'bg-gray-700 text-gray-500 cursor-not-allowed' 
                           : 'bg-[#2d2d2d] hover:bg-[#3d3d3d] text-[#ffa729] hover:text-[#ffb954]'}
                         transition-all duration-300`}
              aria-label="Next page"
            >
              Next
            </button>
          </div>
        </>
      )}
    </div>
  );
}
