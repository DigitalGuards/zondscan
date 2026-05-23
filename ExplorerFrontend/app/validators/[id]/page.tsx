import type { Metadata } from 'next';
import { sharedMetadata } from '../../lib/seo/metaData';
import ValidatorDetailClient from './validator-detail-client';

interface PageProps {
  params: Promise<{ id: string }>;
}

// Per-validator metadata. Brings /validators/[id] in line with the other
// detail-page shells (/tx, /block, /address, /pending/tx): canonical URL,
// OG + Twitter title/description sourced from sharedMetadata so favicon
// and site name stay consistent across the explorer.
export async function generateMetadata({ params }: PageProps): Promise<Metadata> {
  const { id } = await params;
  const canonicalUrl = `https://zondscan.com/validators/${id}`;
  const title = `Validator #${id} | ZondScan`;
  const description = `Stake, status, age, and activation history for QRL Zond validator #${id}.`;

  return {
    ...sharedMetadata,
    title,
    description,
    alternates: {
      ...sharedMetadata.alternates,
      canonical: canonicalUrl,
    },
    openGraph: {
      ...sharedMetadata.openGraph,
      title,
      description,
      url: canonicalUrl,
      siteName: 'ZondScan',
      type: 'website',
    },
    twitter: {
      ...sharedMetadata.twitter,
      title,
      description,
    },
  };
}

export default async function ValidatorDetailPage({ params }: PageProps): Promise<JSX.Element> {
  const { id } = await params;
  return (
    <main aria-labelledby="validator-detail-heading">
      <h1 id="validator-detail-heading" className="sr-only">QRL Zond Validator #{id}</h1>
      <ValidatorDetailClient id={id} />
    </main>
  );
}
