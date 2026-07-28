import { notFound } from 'next/navigation';
import type { Metadata } from 'next';
import EpochsClient from './epochs-client';
import config from '../../../config';
import { sharedMetadata } from '../../lib/seo/metaData';

export const dynamic = 'force-dynamic';

interface EpochsData {
  epochs: any[];
  total: number;
  finalizedEpoch: string;
  justifiedEpoch: string;
}

async function getEpochs(page: string): Promise<EpochsData> {
  try {
    const response = await fetch(`${config.handlerUrl}/epochs?page=${page}&limit=15`, {
      method: 'GET',
      headers: { 'Accept': 'application/json' },
    });

    if (!response.ok) {
      if (response.status === 404) notFound();
      throw new Error(`HTTP error! status: ${response.status}`);
    }

    return response.json();
  } catch (error) {
    console.error('Error fetching epochs:', error);
    throw error;
  }
}

interface PageProps {
  params: Promise<{ query: string }>;
}

export async function generateMetadata({ params }: { params: Promise<{ query: string }> }): Promise<Metadata> {
  const resolvedParams = await params;
  const pageNumber = resolvedParams.query || '1';

  return {
    ...sharedMetadata,
    title: `Epochs - Page ${pageNumber} | ZondScan`,
    description: `View epochs on the QRL 2.0 beacon chain. Page ${pageNumber} of the epochs list.`,
    alternates: {
      ...sharedMetadata.alternates,
      // Self-referencing canonical: there is no /epochs index route (only
      // /epochs/<page>), so each paginated page canonicalizes to itself.
      canonical: `https://zondscan.com/epochs/${pageNumber}`,
    },
    openGraph: {
      ...sharedMetadata.openGraph,
      title: `Epochs - Page ${pageNumber} | ZondScan`,
      description: `View epochs on the QRL 2.0 beacon chain. Page ${pageNumber} of the epochs list.`,
      url: `https://zondscan.com/epochs/${pageNumber}`,
      siteName: 'ZondScan',
      type: 'website',
    },
    twitter: {
      ...sharedMetadata.twitter,
      title: `Epochs - Page ${pageNumber} | ZondScan`,
      description: `View epochs on the QRL 2.0 beacon chain. Page ${pageNumber} of the epochs list.`,
    },
  };
}

export default async function EpochsPage({ params }: PageProps): Promise<JSX.Element> {
  const resolvedParams = await params;
  const pageNumber = resolvedParams.query || '1';
  const data = await getEpochs(pageNumber);

  return (
    <main>
      <EpochsClient initialData={data} initialPage={pageNumber} />
    </main>
  );
}
