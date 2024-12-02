import { Metadata } from 'next';
import BlockDetailClient from './block-detail-client';

export async function generateMetadata({ params }: { params: Promise<{ query: string }> }): Promise<Metadata> {
  const resolvedParams = await params;
  const blockNumber = resolvedParams.query;
  
  return {
    title: `Block #${blockNumber} | ZondScan`,
    description: `View detailed information for Zond blockchain block #${blockNumber}. See block hash, timestamp, transactions, gas used, and more.`,
    openGraph: {
      title: `Block #${blockNumber} | ZondScan`,
      description: `View detailed information for Zond blockchain block #${blockNumber}. See block hash, timestamp, transactions, gas used, and more.`,
      url: `https://zondscan.com/block/${blockNumber}`,
      siteName: 'ZondScan',
      type: 'website',
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