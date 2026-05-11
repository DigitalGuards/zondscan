import { Suspense } from 'react';
import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';
import GasClient from './gas-client';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'Gas | QRL Explorer',
  description: 'Network gas metrics — average gas price, gas usage, mempool size, and recent block gas history on the QRL Zond network',
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'Gas | QRL Explorer',
    description: 'Network gas metrics for the QRL Zond network',
    url: 'https://zondscan.com/gas',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'Gas | QRL Explorer',
    description: 'Network gas metrics for the QRL Zond network',
  },
};

export default function GasPage(): JSX.Element {
  return (
    <main>
      <h1 className="sr-only">QRL Zond Network Gas Metrics</h1>
      <Suspense fallback={<div className="p-4 text-center">Loading gas metrics...</div>}>
        <GasClient />
      </Suspense>
    </main>
  );
}
