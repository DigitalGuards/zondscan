import React from 'react';
import PendingList from './PendingList';

interface PageProps {
  params: Promise<{ query: string }>;
}

export default async function PendingPage({ params }: PageProps) {
  // Initialize with empty data since we'll fetch it client-side
  const initialData = {
    txs: [],
    total: 0
  };

  const { query } = await params;

  return (
    <main>
      <h1 className="sr-only">Pending Transactions - Page {query}</h1>
      <PendingList 
        initialData={initialData}
        currentPage={parseInt(query, 10)}
      />
    </main>
  );
}
