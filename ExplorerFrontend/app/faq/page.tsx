import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';
import FAQClient from './faq-client';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'FAQ | ZondScan',
  description: 'Find answers to frequently asked questions about QRL, blockchain, smart contracts, and more.',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: 'https://zondscan.com/faq',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'FAQ | ZondScan',
    description: 'Find answers to frequently asked questions about QRL, blockchain, smart contracts, and more.',
    url: 'https://zondscan.com/faq',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'FAQ | ZondScan',
    description: 'Find answers to frequently asked questions about QRL, blockchain, smart contracts, and more.',
  },
};

export default function FAQPage(): JSX.Element {
  return (
    <main aria-labelledby="faq-heading">
      <h1 id="faq-heading" className="sr-only">Zondscan FAQ</h1>
      <FAQClient />
    </main>
  );
}
