import React from 'react';
import PendingList from './PendingList';

interface PageProps {
  params: { query: string };
}

export default function PendingPage({ params }: PageProps) {
  // Initialize with empty data since we'll fetch it client-side
  const initialData = {
    txs: [],
    total: 0
  };

  return (
    <main>
      <h1 className="sr-only">Pending Transactions - Page {params.query}</h1>
      <PendingList 
        initialData={initialData}
        currentPage={parseInt(params.query, 10)}
      />
    </main>
  );
}
