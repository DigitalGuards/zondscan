import { SITE_NAME, siteUrl } from '../lib/site';
import { Suspense } from 'react';
import ValidatorsWrapper from './validators-client';
import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';


export const metadata: Metadata = {
  ...sharedMetadata,
  title: `Validators | ${SITE_NAME}`,
  description:
    'View active validators, their ages, uptime, and staking information on the QRL 2.0 network',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: siteUrl('/validators'),
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: `Validators | ${SITE_NAME}`,
    description:
      'View active validators, their ages, uptime, and staking information on the QRL 2.0 network',
    url: siteUrl('/validators'),
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: `Validators | ${SITE_NAME}`,
    description:
      'View active validators, their ages, uptime, and staking information on the QRL 2.0 network',
  },
};


export default async function ValidatorsPage(): Promise<JSX.Element> {
  return (
    <main>
      <h1 className="sr-only">QRL 2.0 Network Validators</h1>
      <Suspense fallback={<div className="p-4 text-center">Loading validators...</div>}>
        <ValidatorsWrapper />
      </Suspense>
    </main>
  );
}
