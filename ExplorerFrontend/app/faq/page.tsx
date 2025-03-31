import { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';
import FAQClient from './faq-client';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'FAQ | QRL Explorer',
  description: 'Find answers to frequently asked questions about QRL, blockchain, smart contracts, and more.',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: 'https://zondscan.com/faq',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'FAQ | QRL Explorer',
    description: 'Find answers to frequently asked questions about QRL, blockchain, smart contracts, and more.',
    url: 'https://zondscan.com/faq',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'FAQ | QRL Explorer',
    description: 'Find answers to frequently asked questions about QRL, blockchain, smart contracts, and more.',
  },
};

export default function FAQPage() {
  return <FAQClient />;
}
