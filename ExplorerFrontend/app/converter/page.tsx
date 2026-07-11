import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';
import ConverterClient from './converter-client';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'Quanta to Shor Converter | ZondScan',
  description:
    'Convert Quanta to Shor quickly and accurately using our conversion tool. Get real-time conversion rates and insights on the QRL network.',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: 'https://zondscan.com/converter',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'Quanta to Shor Converter | ZondScan',
    description:
      'Convert Quanta to Shor quickly and accurately using our conversion tool. Get real-time conversion rates and insights on the QRL network.',
    url: 'https://zondscan.com/converter',
    siteName: 'ZondScan',
    type: 'website',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'Quanta to Shor Converter | ZondScan',
    description:
      'Convert Quanta to Shor quickly and accurately using our conversion tool. Get real-time conversion rates and insights on the QRL network.',
  },
};

export default function QuantaToShorPage(): JSX.Element {
  return (
    <main aria-labelledby="converter-heading">
      <h1 id="converter-heading" className="sr-only">Quanta to Shor Converter</h1>
      <ConverterClient />
    </main>
  );
}
