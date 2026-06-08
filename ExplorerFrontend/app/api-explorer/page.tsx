import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';
import ApiExplorerClient from './api-explorer-client';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'API Explorer | QRL Explorer',
  description: 'Explore the free Zondscan REST API. Access blockchain data, transactions, blocks, addresses, validators, and token information for the QRL 2.0 network.',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: 'https://zondscan.com/api-explorer',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'API Explorer | QRL Explorer',
    description: 'Explore the free Zondscan REST API. Access blockchain data, transactions, blocks, addresses, validators, and token information for the QRL 2.0 network.',
    url: 'https://zondscan.com/api-explorer',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'API Explorer | QRL Explorer',
    description: 'Explore the free Zondscan REST API. Access blockchain data, transactions, blocks, addresses, validators, and token information for the QRL 2.0 network.',
  },
};

export default function ApiExplorerPage(): JSX.Element {
  return (
    <main aria-labelledby="api-explorer-heading">
      <h1 id="api-explorer-heading" className="sr-only">Zondscan API Explorer</h1>
      <ApiExplorerClient />
    </main>
  );
}
