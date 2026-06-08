import React from 'react';
import type { Metadata } from 'next';
import TransactionsList from './[query]/TransactionsList';
import type { Transaction } from '@/app/types';
import { sharedMetadata } from '../lib/seo/metaData';
import config from '../../config';

export const dynamic = 'force-dynamic';

// Static metadata for the global transactions list. Mirrors the per-detail
// page pattern (canonical / OG / Twitter spread from sharedMetadata) so
// social previews are consistent across the explorer.
export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'Transactions | ZondScan',
  description: 'Browse the latest transactions on the QRL 2.0 blockchain: hash, sender, recipient, value, and gas across the most recent network activity.',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: 'https://zondscan.com/transactions',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'Transactions | ZondScan',
    description: 'Browse the latest transactions on the QRL 2.0 blockchain: hash, sender, recipient, value, and gas across the most recent network activity.',
    url: 'https://zondscan.com/transactions',
    siteName: 'ZondScan',
    type: 'website',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'Transactions | ZondScan',
    description: 'Browse the latest transactions on the QRL 2.0 blockchain: hash, sender, recipient, value, and gas across the most recent network activity.',
  },
};

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
