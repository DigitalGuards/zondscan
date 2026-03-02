import { notFound } from 'next/navigation';
import type { Metadata } from 'next';
import BlocksClient from './blocks-client';
import type { BlocksResponse } from '@/app/types';
import config from '../../../config';
import { sharedMetadata } from '../../lib/seo/metaData';

export const dynamic = 'force-dynamic';

async function getBlocks(page: string): Promise<BlocksResponse> {
  try {
    const response = await fetch(`${config.handlerUrl}/blocks?page=${page}`, {
      method: 'GET',
      headers: {
        'Accept': 'application/json'
      }
    });

    if (!response.ok) {
      if (response.status === 404) {
        notFound();
      }
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    return response.json();
  } catch (error) {
    console.error('Error fetching blocks:', error);
    throw error;
  }
}

interface PageProps {
    params: Promise<{ query: string }>;
}

export async function generateMetadata({ params }: { params: Promise<{ query: string }> }): Promise<Metadata> {
  const resolvedParams = await params;
  const pageNumber = resolvedParams.query || '1';
  const canonicalUrl = `https://zondscan.com/blocks`;
  
  return {
    ...sharedMetadata,
    title: `Latest Synced Blocks - Page ${pageNumber} | ZondScan`,
    description: `View the most recently synced blocks on the Zond blockchain network. Page ${pageNumber} of the blocks list.`,
    alternates: {
      ...sharedMetadata.alternates,
      canonical: canonicalUrl,
    },
    openGraph: {
      ...sharedMetadata.openGraph,
      title: `Latest Synced Blocks - Page ${pageNumber} | ZondScan`,
      description: `View the most recently synced blocks on the Zond blockchain network. Page ${pageNumber} of the blocks list.`,
      url: `https://zondscan.com/blocks/${pageNumber}`,
      siteName: 'ZondScan',
      type: 'website',
    },
    twitter: {
      ...sharedMetadata.twitter,
      title: `Latest Synced Blocks - Page ${pageNumber} | ZondScan`,
      description: `View the most recently synced blocks on the Zond blockchain network. Page ${pageNumber} of the blocks list.`,
    },
  };
}

export default async function BlocksPage({ params }: PageProps): Promise<JSX.Element> {
  const resolvedParams = await params;
  const pageNumber = resolvedParams.query || '1';
  const data = await getBlocks(pageNumber);

  return (
    <main>
      <BlocksClient
        initialData={data}
        initialPage={pageNumber}
      />
    </main>
  );
}
