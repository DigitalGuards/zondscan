import { Suspense } from 'react';
import ValidatorsWrapper from './validators-client';
import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';


export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'Validators | QRL Explorer',
  description:
    'View active validators, their ages, uptime, and staking information on the QRL network',
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'Validators | QRL Explorer',
    description:
      'View active validators, their ages, uptime, and staking information on the QRL network',
    url: 'https://zondscan.com/richlist',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'Validators | QRL Explorer',
    description:
      'View active validators, their ages, uptime, and staking information on the QRL network',
  },
};


export default async function ValidatorsPage(): Promise<JSX.Element> {
  return (
    <main>
      <h1 className="sr-only">QRL Zond Network Validators</h1>
      <Suspense fallback={<div className="p-4 text-center">Loading validators...</div>}>
        <ValidatorsWrapper />
      </Suspense>
    </main>
  );
}
