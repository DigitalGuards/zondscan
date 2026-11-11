import { pageTitle, canonicalUrl } from '../lib/site';
import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';
import OrderBookClient from './orderbook-client';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: pageTitle('QRL Order Book'),
  description: 'Live MEXC QRL/USDT spot order book, recent trades and fund-flow analysis.',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: canonicalUrl('/orderbook'),
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: pageTitle('QRL Order Book'),
    description: 'MEXC QRL/USDT market depth, recent trades and fund flow.',
    url: canonicalUrl('/orderbook'),
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: pageTitle('QRL Order Book'),
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
