import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';
import ApiExplorerClient from './api-explorer-client';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'API Explorer | QRL Explorer',
  description: 'Explore the free Zondscan REST API. Access blockchain data, transactions, blocks, addresses, validators, and token information for the QRL Zond network.',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: '/api-explorer',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'API Explorer | QRL Explorer',
    description: 'Explore the free Zondscan REST API. Access blockchain data, transactions, blocks, addresses, validators, and token information for the QRL Zond network.',
    url: '/api-explorer',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'API Explorer | QRL Explorer',
    description: 'Explore the free Zondscan REST API. Access blockchain data, transactions, blocks, addresses, validators, and token information for the QRL Zond network.',
  },
};

export default function ApiExplorerPage(): JSX.Element {
  return <ApiExplorerClient />;
}
