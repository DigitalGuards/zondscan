import { notFound } from 'next/navigation';
import type { Metadata } from 'next';
import BlocksClient from './blocks-client';
import type { BlocksResponse } from '@/app/types';
import config from '../../../config';
import { sharedMetadata } from '../../lib/seo/metaData';

export const dynamic = 'force-dynamic';

async function getBlocks(page: string): Promise<BlocksResponse | null> {
  let response: Response;
  try {
    response = await fetch(`${config.handlerUrl}/blocks?page=${page}&limit=10`, {
      method: 'GET',
      headers: {
        'Accept': 'application/json'
      }
    });
  } catch (error) {
    // Network / fetch failure. Return null so the page renders a friendly
    // error panel instead of crashing to the error boundary.
    console.error('Error fetching blocks:', error);
    return null;
  }

  // notFound() throws a NEXT_NOT_FOUND sentinel that the framework must
  // receive uncaught, so keep it outside the fetch try/catch above.
  if (response.status === 404) {
    notFound();
  }

  if (!response.ok) {
    console.error(`Error fetching blocks: HTTP status ${response.status}`);
    return null;
  }

  try {
    return await response.json();
  } catch (error) {
    console.error('Error parsing blocks response:', error);
    return null;
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
    description: `View the most recently synced blocks on the QRL blockchain network. Page ${pageNumber} of the blocks list.`,
    alternates: {
      ...sharedMetadata.alternates,
      canonical: canonicalUrl,
    },
    openGraph: {
      ...sharedMetadata.openGraph,
      title: `Latest Synced Blocks - Page ${pageNumber} | ZondScan`,
      description: `View the most recently synced blocks on the QRL blockchain network. Page ${pageNumber} of the blocks list.`,
      url: `https://zondscan.com/blocks/${pageNumber}`,
      siteName: 'ZondScan',
      type: 'website',
    },
    twitter: {
      ...sharedMetadata.twitter,
      title: `Latest Synced Blocks - Page ${pageNumber} | ZondScan`,
      description: `View the most recently synced blocks on the QRL blockchain network. Page ${pageNumber} of the blocks list.`,
    },
  };
}

export default async function BlocksPage({ params }: PageProps): Promise<JSX.Element> {
  const resolvedParams = await params;
  const pageNumber = resolvedParams.query || '1';
  const data = await getBlocks(pageNumber);

  if (!data) {
    return (
      <main>
        <div className="container mx-auto px-4">
          <div className="bg-red-900/20 border border-red-500/50 rounded-xl p-6 shadow-lg mt-6">
            <h2 className="text-red-500 font-semibold mb-2">Failed to Load Blocks</h2>
            <p className="text-text-secondary">
              The blocks list could not be loaded right now. This is usually a
              temporary issue reaching the backend. Please try again in a moment.
            </p>
          </div>
        </div>
      </main>
    );
  }

  return (
    <main>
      <BlocksClient
        initialData={data}
        initialPage={pageNumber}
      />
    </main>
  );
}
