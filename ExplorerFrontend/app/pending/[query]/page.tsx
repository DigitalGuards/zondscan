import React from 'react';
import { Metadata } from 'next';
import PendingList from './PendingList';

export async function generateMetadata({ params }: { params: Promise<{ query: string }> }): Promise<Metadata> {
  const resolvedParams = await params;
  const pageNumber = resolvedParams.query || '1';
  
  return {
    title: `Pending Transactions - Page ${pageNumber} | ZondScan`,
    description: `View all pending transactions waiting to be included in the next block on the Zond blockchain network. Page ${pageNumber} of the mempool transactions list.`,
    openGraph: {
      title: `Pending Transactions - Page ${pageNumber} | ZondScan`,
      description: `View all pending transactions waiting to be included in the next block on the Zond blockchain network. Page ${pageNumber} of the mempool transactions list.`,
      url: `https://zondscan.com/pending/${pageNumber}`,
      siteName: 'ZondScan',
      type: 'website',
    },
  };
}

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
