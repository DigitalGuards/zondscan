import { SITE_NAME, SITE_ORIGIN, pageTitle } from '../../lib/site';
import type { Metadata } from 'next';
import BlockDetailClient from './block-detail-client';
import { sharedMetadata } from '../../lib/seo/metaData';

export async function generateMetadata({ params }: { params: Promise<{ query: string }> }): Promise<Metadata> {
  const resolvedParams = await params;
  const blockNumber = resolvedParams.query;
  const canonicalUrl = `${SITE_ORIGIN}/block/${blockNumber}`;
  
  return {
    ...sharedMetadata,
    title: pageTitle(`Block #${blockNumber}`),
    description: `View detailed information for QRL blockchain block #${blockNumber}. See block hash, timestamp, transactions, gas used, and more.`,
    alternates: {
      ...sharedMetadata.alternates,
      canonical: canonicalUrl,
    },
    openGraph: {
      ...sharedMetadata.openGraph,
      title: pageTitle(`Block #${blockNumber}`),
      description: `View detailed information for QRL blockchain block #${blockNumber}. See block hash, timestamp, transactions, gas used, and more.`,
      url: `${SITE_ORIGIN}/block/${blockNumber}`,
      siteName: SITE_NAME,
      type: 'website',
    },
    twitter: {
      ...sharedMetadata.twitter,
      title: pageTitle(`Block #${blockNumber}`),
      description: `View detailed information for QRL blockchain block #${blockNumber}. See block hash, timestamp, transactions, gas used, and more.`,
    },
  };
}

interface PageProps {
  params: Promise<{ query: string }>;
}

export default async function Page({ params }: PageProps): Promise<JSX.Element> {
  const resolvedParams = await params;
  const blockNumber = resolvedParams.query;

  return <BlockDetailClient blockNumber={blockNumber} />;
}