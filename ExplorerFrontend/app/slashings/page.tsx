import { SITE_NAME, siteUrl } from '../lib/site';
import type { Metadata } from 'next';
import { notFound } from 'next/navigation';
import { sharedMetadata } from '../lib/seo/metaData';
import SlashingsClient from './slashings-client';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: `Slashings | ${SITE_NAME}`,
  description: 'Slashing events and validator set changes on the QRL network.',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: siteUrl('/slashings'),
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: `Slashings | ${SITE_NAME}`,
    url: siteUrl('/slashings'),
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
