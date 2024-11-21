import React from 'react';
import TransactionsList from './TransactionsList';
import { Transaction } from './types';

export const dynamic = 'force-dynamic';

interface TransactionsResponse {
  txs: Transaction[];
  total: number;
}

async function getTransactions(page: string): Promise<TransactionsResponse> {
  const handlerUrl = process.env.NEXT_PUBLIC_HANDLER_URL || 'http://localhost:8080';
  const response = await fetch(`${handlerUrl}/txs?page=${page}`, {
    next: { revalidate: 10 }
  });
  
  if (!response.ok) {
    throw new Error('Failed to fetch transactions');
  }

  return response.json();
}

export default async function Page({ params }: { params: { query: string } }) {
  const data = await getTransactions(params.query);
  
  return <TransactionsList initialData={data} currentPage={parseInt(params.query)} />;
}
