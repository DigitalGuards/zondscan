import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';
import OrderBookClient from './orderbook-client';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'QRL Order Book Arena | ZondScan',
  description:
    'Explore the live MEXC QRL/USDT spot order book as an animated market arena with exact depth and recent-trade data.',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: 'https://zondscan.com/orderbook',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'QRL Order Book Arena | ZondScan',
    description: 'A live, visual view of the MEXC QRL/USDT spot order book.',
    url: 'https://zondscan.com/orderbook',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'QRL Order Book Arena | ZondScan',
    description: 'A live, visual view of the MEXC QRL/USDT spot order book.',
  },
};

export default function OrderBookPage(): JSX.Element {
  return (
    <main>
      <h1 className="sr-only">QRL USDT Order Book Arena</h1>
      <OrderBookClient />
    </main>
  );
}

