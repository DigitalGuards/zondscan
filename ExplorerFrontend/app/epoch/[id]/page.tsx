import { SITE_NAME, SITE_ORIGIN } from '../../lib/site';
import type { Metadata } from 'next';
import EpochDetailClient from './epoch-detail-client';
import { sharedMetadata } from '../../lib/seo/metaData';

export async function generateMetadata({ params }: { params: Promise<{ id: string }> }): Promise<Metadata> {
  const resolvedParams = await params;
  const epochId = resolvedParams.id;

  return {
    ...sharedMetadata,
    title: `Epoch ${epochId} | ${SITE_NAME}`,
    description: `View detailed information for epoch ${epochId} on the QRL 2.0 beacon chain.`,
    alternates: {
      ...sharedMetadata.alternates,
      canonical: `${SITE_ORIGIN}/epoch/${epochId}`,
    },
    openGraph: {
      ...sharedMetadata.openGraph,
      title: `Epoch ${epochId} | ${SITE_NAME}`,
      description: `View detailed information for epoch ${epochId} on the QRL 2.0 beacon chain.`,
      url: `${SITE_ORIGIN}/epoch/${epochId}`,
      siteName: SITE_NAME,
      type: 'website',
    },
  };
}

interface PageProps {
  params: Promise<{ id: string }>;
}

export default async function Page({ params }: PageProps): Promise<JSX.Element> {
  const resolvedParams = await params;
  return <EpochDetailClient epochId={resolvedParams.id} />;
}
