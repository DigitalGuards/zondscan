import React from 'react';
import { Metadata } from 'next';
import PendingList from './PendingList';
import axios from 'axios';
import config from '../../../config';
import { PendingTransaction } from '../tx/types';
import { sharedMetadata } from '../../lib/seo/metaData';

interface PaginatedResponse {
  transactions: PendingTransaction[];
  total: number;
  page: number;
  limit: number;
  totalPages: number;
}

async function fetchInitialData(page: number): Promise<PaginatedResponse> {
  try {
    const response = await axios.get<PaginatedResponse>(`${config.handlerUrl}/pending-transactions`, {
      params: {
        page,
        limit: 10
      }
    });
    return response.data;
  } catch (error) {
    console.error('Error fetching initial pending transactions:', error);
    // Return empty initial state that matches the PaginatedResponse interface
    return {
      transactions: [],
      total: 0,
      page: page,
      limit: 10,
      totalPages: 1
    };
  }
}

export async function generateMetadata({ params }: { params: Promise<{ query: string }> }): Promise<Metadata> {
  const resolvedParams = await params;
  const pageNumber = resolvedParams.query || '1';
  const canonicalUrl = `https://zondscan.com/pending`;
  
  return {
    ...sharedMetadata,
    title: `Pending Transactions - Page ${pageNumber} | ZondScan`,
    description:
      `View all pending transactions waiting to be included in the next block on the Zond blockchain network. Page ${pageNumber} of the mempool transactions list.`,
    alternates: {
      ...sharedMetadata.alternates,
      canonical: canonicalUrl,
    },
    openGraph: {
      ...sharedMetadata.openGraph,
      title: `Pending Transactions - Page ${pageNumber} | ZondScan`,
      description:
        `View all pending transactions waiting to be included in the next block on the Zond blockchain network. Page ${pageNumber} of the mempool transactions list.`,
      url: canonicalUrl,
      siteName: 'ZondScan',
      type: 'website',
    },
    twitter: {
      ...sharedMetadata.twitter,
      title: `Pending Transactions - Page ${pageNumber} | ZondScan`,
      description:
        `View all pending transactions waiting to be included in the next block on the Zond blockchain network. Page ${pageNumber} of the mempool transactions list.`,
    },
  };
}

interface PageProps {
  params: Promise<{ query: string }>;
}

export default async function PendingPage({ params }: PageProps) {
  const { query } = await params;
  const currentPage = parseInt(query, 10) || 1;
  const initialData = await fetchInitialData(currentPage);

  return (
    <main>
      <h1 className="sr-only">Pending Transactions - Page {currentPage}</h1>
      <PendingList 
        initialData={initialData}
        currentPage={currentPage}
      />
    </main>
  );
}
