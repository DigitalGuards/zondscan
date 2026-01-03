import { notFound } from 'next/navigation';
import type { Metadata } from 'next';
import BlocksClient from './blocks-client';
import type { BlocksResponse } from './types';
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

  try {
    const data = await getBlocks(pageNumber);

    return (
      <main>
        <BlocksClient 
          initialData={data}
          initialPage={pageNumber} 
        />
      </main>
    );
  } catch (error) {
    return (
      <div role="alert" className="p-8">
        <h1 className="text-2xl font-bold mb-6 text-[#ffa729]">Error</h1>
        <p className="text-gray-300">Failed to load blocks. Please try again later.</p>
        {process.env.NODE_ENV === 'development' && (
          <pre className="mt-2 text-sm text-red-500">
            {error instanceof Error ? error.message : 'Unknown error'}
          </pre>
        )}
      </div>
    );
  }
}
