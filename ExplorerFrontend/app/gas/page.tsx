import { Suspense } from 'react';
import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';
import GasClient from './gas-client';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'Gas Tracker | ZondScan',
  description: 'Network gas metrics, average gas price, gas usage, mempool size, and recent block gas history on the QRL 2.0 network',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: 'https://zondscan.com/gas',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'Gas Tracker | ZondScan',
    description: 'Network gas metrics for the QRL 2.0 network',
    url: 'https://zondscan.com/gas',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'Gas Tracker | ZondScan',
    description: 'Network gas metrics for the QRL 2.0 network',
  },
};

export default function GasPage(): JSX.Element {
  return (
    <main>
      <h1 className="sr-only">QRL 2.0 Network Gas Metrics</h1>
      <Suspense fallback={<div className="p-4 text-center">Loading gas metrics...</div>}>
        <GasClient />
      </Suspense>
    </main>
  );
}
