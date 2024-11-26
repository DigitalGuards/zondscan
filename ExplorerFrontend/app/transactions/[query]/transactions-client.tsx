'use client';

import React from 'react';
import TransactionsList from './TransactionsList';
import { Transaction } from './types';

interface TransactionsResponse {
  txs: Transaction[];
  total: number;
}

interface TransactionsClientProps {
  initialData: TransactionsResponse;
  pageNumber: string;
}

export default function TransactionsClient({ initialData, pageNumber }: TransactionsClientProps) {
  const [data, setData] = React.useState<TransactionsResponse>(initialData);
  const [error, setError] = React.useState<string | null>(null);

  React.useEffect(() => {
    setData(initialData);
  }, [initialData, pageNumber]);

  if (error) {
    return (
      <div role="alert" className="p-4">
        <h1 className="text-xl font-bold mb-2">Error</h1>
        <p>{error}</p>
      </div>
    );
  }

  return (
    <TransactionsList 
      initialData={data} 
      currentPage={parseInt(pageNumber)} 
    />
  );
}
