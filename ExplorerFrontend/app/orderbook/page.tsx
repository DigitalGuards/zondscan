import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';
import OrderBookClient from './orderbook-client';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'QRL Order Book | ZondScan',
  description: 'Live MEXC QRL/USDT spot order book, recent trades and fund-flow analysis.',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: 'https://zondscan.com/orderbook',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'QRL Order Book | ZondScan',
    description: 'MEXC QRL/USDT market depth, recent trades and fund flow.',
    url: 'https://zondscan.com/orderbook',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'QRL Order Book | ZondScan',
    description: 'MEXC QRL/USDT market depth, recent trades and fund flow.',
  },
};

export default function OrderBookPage(): JSX.Element {
  return (
    <>
      <h1 className="sr-only">QRL USDT Order Book</h1>
      <OrderBookClient />
    </>
  );
}
