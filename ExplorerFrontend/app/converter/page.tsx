import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';
import ConverterClient from './converter-client';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'Quanta to Shor Converter | QRL Explorer',
  description:
    'Convert Quanta to Shor quickly and accurately using our conversion tool. Get real-time conversion rates and insights on the QRL network.',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: 'https://zondscan.com/quanta-to-shor',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'Quanta to Shor Converter | QRL Explorer',
    description:
      'Convert Quanta to Shor quickly and accurately using our conversion tool. Get real-time conversion rates and insights on the QRL network.',
    url: 'https://zondscan.com/quanta-to-shor',
    siteName: 'ZondScan',
    type: 'website',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'Quanta to Shor Converter | QRL Explorer',
    description:
      'Convert Quanta to Shor quickly and accurately using our conversion tool. Get real-time conversion rates and insights on the QRL network.',
  },
};

export default function QuantaToShorPage(): JSX.Element {
  return <ConverterClient />;
}
