import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { sharedMetadata } from '../lib/seo/metaData';
import SlashingsClient from './slashings-client';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'Slashings | ZondScan',
  description: 'Slashing events and validator set changes on the QRL network.',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: 'https://zondscan.com/slashings',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'Slashings | ZondScan',
    url: 'https://zondscan.com/slashings',
  },
};

export default async function SlashingsPage({
  searchParams,
}: {
  searchParams: Promise<{ validator?: string | string[] }>;
}) {
  const { validator } = await searchParams;
  if (
    validator !== undefined &&
    (typeof validator !== 'string' ||
      !/^\d+$/.test(validator) ||
      BigInt(validator) > BigInt('18446744073709551615'))
  )
    notFound();
  const validatorIndex = validator === undefined ? undefined : BigInt(validator).toString();
  return (
    <main>
      <SlashingsClient key={validatorIndex ?? 'network'} validatorIndex={validatorIndex} />
    </main>
  );
}
