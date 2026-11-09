import { SITE_NAME, siteUrl } from '../lib/site';
import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';
import ConverterClient from './converter-client';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: `QRL Unit Converter: Quanta, Shor, Planck | ${SITE_NAME}`,
  description:
    `Convert between QRL units with exact precision: 1 Quanta = 10^9 Shor = 10^18 Planck. Free unit conversion tool on the ${SITE_NAME} explorer.`,
  alternates: {
    ...sharedMetadata.alternates,
    canonical: siteUrl('/converter'),
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: `QRL Unit Converter: Quanta, Shor, Planck | ${SITE_NAME}`,
    description:
      `Convert between QRL units with exact precision: 1 Quanta = 10^9 Shor = 10^18 Planck. Free unit conversion tool on the ${SITE_NAME} explorer.`,
    url: siteUrl('/converter'),
    siteName: SITE_NAME,
    type: 'website',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: `QRL Unit Converter: Quanta, Shor, Planck | ${SITE_NAME}`,
    description:
      `Convert between QRL units with exact precision: 1 Quanta = 10^9 Shor = 10^18 Planck. Free unit conversion tool on the ${SITE_NAME} explorer.`,
  },
};

export default function UnitConverterPage(): JSX.Element {
  return (
    <main aria-labelledby="converter-heading">
      <h1 id="converter-heading" className="sr-only">QRL Unit Converter</h1>
      <ConverterClient />
    </main>
  );
}
