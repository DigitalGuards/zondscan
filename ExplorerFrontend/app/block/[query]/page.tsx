import { Metadata } from 'next';
import BlockDetailClient from './block-detail-client';
import { sharedMetadata } from '../../lib/seo/metaData';

export async function generateMetadata({ params }: { params: Promise<{ query: string }> }): Promise<Metadata> {
  const resolvedParams = await params;
  const blockNumber = resolvedParams.query;
  const canonicalUrl = `https://zondscan.com/block`;
  
  return {
    ...sharedMetadata,
    title: `Block #${blockNumber} | ZondScan`,
    description: `View detailed information for Zond blockchain block #${blockNumber}. See block hash, timestamp, transactions, gas used, and more.`,
    alternates: {
      ...sharedMetadata.alternates,
      canonical: canonicalUrl,
    },
    openGraph: {
      ...sharedMetadata.openGraph,
      title: `Block #${blockNumber} | ZondScan`,
      description: `View detailed information for Zond blockchain block #${blockNumber}. See block hash, timestamp, transactions, gas used, and more.`,
      url: `https://zondscan.com/block/${blockNumber}`,
      siteName: 'ZondScan',
      type: 'website',
    },
    twitter: {
      ...sharedMetadata.twitter,
      title: `Block #${blockNumber} | ZondScan`,
      description: `View detailed information for Zond blockchain block #${blockNumber}. See block hash, timestamp, transactions, gas used, and more.`,
    },
  };
}

interface PageProps {
  params: Promise<{ query: string }>;
}

export default async function Page({ params }: PageProps) {
  const resolvedParams = await params;
  const blockNumber = resolvedParams.query;

  return <BlockDetailClient blockNumber={blockNumber} />;
}