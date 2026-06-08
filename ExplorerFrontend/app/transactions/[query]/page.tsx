import { Suspense } from 'react';
import { notFound } from 'next/navigation';
import type { Metadata } from 'next';
import TransactionsClient from './transactions-client';
import type { TransactionsResponse } from '@/app/types';
import config from '../../../config';
import { sharedMetadata } from '../../lib/seo/metaData';

async function getTransactions(page: string): Promise<TransactionsResponse> {
  try {
    const pageNum = parseInt(page, 10) || 1;

    // Cache the response server-side for 10 s. Block time on QRL Zond is
    // O(1 minute), so 10 s is well within a "freshness" SLA for a listing
    // page and lets ISR absorb concurrent traffic instead of hammering
    // the backend with one fetch per pageview. Drop the _t cache-buster
    // and the no-store directive that were forcing zero caching.
    const response = await fetch(`${config.handlerUrl}/txs?page=${pageNum}&limit=10`, {
      method: 'GET',
      headers: {
        Accept: 'application/json',
      },
      next: {
        revalidate: 10,
      },
    });

    if (!response.ok) {
      if (response.status === 404) {
        notFound();
      }
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const data = await response.json();
    return data;
  } catch (error) {
    console.error('Error fetching transactions:', error);
    throw error;
  }
}

function LoadingUI(): JSX.Element {
  return (
    <div className="flex items-center justify-center min-h-screen">
      <div className="text-lg">Loading transactions...</div>
    </div>
  );
}

interface PageProps {
  params: Promise<{ query: string }>;
}

export async function generateMetadata({ params }: { params: Promise<{ query: string }> }): Promise<Metadata> {
  const resolvedParams = await params;
  const pageNumber = resolvedParams.query || '1';
  const canonicalUrl = `https://zondscan.com/transactions`;
  
  return {
    ...sharedMetadata,
    title: `Transactions - Page ${pageNumber} | ZondScan`,
    description: `View all transactions on the QRL blockchain network. Page ${pageNumber} of the transaction list showing latest transfers, smart contract interactions, and more.`,
    alternates: {
      ...sharedMetadata.alternates,
      canonical: canonicalUrl,
    },
    openGraph: {
      ...sharedMetadata.openGraph,
      title: `Transactions - Page ${pageNumber} | ZondScan`,
      description: `View all transactions on the QRL blockchain network. Page ${pageNumber} of the transaction list showing latest transfers, smart contract interactions, and more.`,
      url: `https://zondscan.com/transactions/${pageNumber}`,
      siteName: 'ZondScan',
      type: 'website',
    },
    twitter: {
      ...sharedMetadata.twitter,
      title: `Transactions - Page ${pageNumber} | ZondScan`,
      description: `View all transactions on the QRL blockchain network. Page ${pageNumber} of the transaction list showing latest transfers, smart contract interactions, and more.`,
    },
  };
}

export default async function Page({ params }: PageProps): Promise<JSX.Element> {
  const resolvedParams = await params;
  const pageNumber = resolvedParams.query || '1';
  const data = await getTransactions(pageNumber);

  return (
    <main>
      <h1 className="sr-only">Transactions - Page {pageNumber}</h1>
      <Suspense fallback={<LoadingUI />}>
        <TransactionsClient
          initialData={data}
          pageNumber={pageNumber}
        />
      </Suspense>
    </main>
  );
}
