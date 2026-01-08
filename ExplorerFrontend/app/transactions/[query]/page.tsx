import { Suspense } from 'react';
import { notFound } from 'next/navigation';
import type { Metadata } from 'next';
import TransactionsClient from './transactions-client';
import type { TransactionsResponse } from '@/app/types';
import config from '../../../config';

async function getTransactions(page: string): Promise<TransactionsResponse> {
  try {
    console.log(`Fetching transactions for page ${page}`);
    
    const pageNum = parseInt(page, 10) || 1;
    
    const timestamp = Date.now();
    const response = await fetch(`${config.handlerUrl}/txs?page=${pageNum}&_t=${timestamp}`, {
      method: 'GET',
      headers: {
        'Accept': 'application/json',
        'Cache-Control': 'no-cache'
      },
      cache: 'no-store',
      next: {
        revalidate: 0
      }
    });

    if (!response.ok) {
      if (response.status === 404) {
        notFound();
      }
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    const data = await response.json();
    console.log(`Received ${data.txs?.length || 0} transactions. Total reported: ${data.total || 0}`);
    
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
    title: `Transactions - Page ${pageNumber} | ZondScan`,
    description: `View all transactions on the Zond blockchain network. Page ${pageNumber} of the transaction list showing latest transfers, smart contract interactions, and more.`,
    alternates: {
      canonical: canonicalUrl,
    },
    openGraph: {
      title: `Transactions - Page ${pageNumber} | ZondScan`,
      description: `View all transactions on the Zond blockchain network. Page ${pageNumber} of the transaction list showing latest transfers, smart contract interactions, and more.`,
      url: `https://zondscan.com/transactions/${pageNumber}`,
      siteName: 'ZondScan',
      type: 'website',
    },
  };
}

export default async function Page({ params }: PageProps): Promise<JSX.Element> {
  const resolvedParams = await params;
  const pageNumber = resolvedParams.query || '1';

  try {
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
  } catch (error) {
    return (
      <div role="alert" className="p-4">
        <h1 className="text-xl font-bold mb-2">Error</h1>
        <p>Failed to load transactions. Please try again later.</p>
        {process.env.NODE_ENV === 'development' && (
          <pre className="mt-2 text-sm text-red-500">
            {error instanceof Error ? error.message : 'Unknown error'}
          </pre>
        )}
      </div>
    );
  }
}
