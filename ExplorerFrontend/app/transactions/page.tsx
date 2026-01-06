import React from 'react';
import TransactionsList from './[query]/TransactionsList';
import type { Transaction } from '@/app/types';
import config from '../../config';

export const dynamic = 'force-dynamic';

interface TransactionsResponse {
  txs: Transaction[];
  total: number;
}

interface PageProps {
  searchParams: Promise<{ page?: string }>;
}

async function getTransactions(page: string): Promise<TransactionsResponse> {
  const handlerUrl = config.handlerUrl;
  const response = await fetch(`${handlerUrl}/txs?page=${page}`, {
    next: { revalidate: 10 }
  });
  
  if (!response.ok) {
    throw new Error('Failed to fetch transactions');
  }

  return response.json();
}

export default async function Page({ searchParams }: PageProps): Promise<JSX.Element> {
  // Await searchParams
  const resolvedParams = await searchParams;
  
  // Get page from searchParams or default to '1'
  const page = resolvedParams.page || '1';
  
  // Fetch data
  const data = await getTransactions(page);
  
  // Parse page number
  const currentPage = parseInt(page);

  return (
    <TransactionsList 
      initialData={data} 
      currentPage={currentPage} 
    />
  );
}
