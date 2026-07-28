import type { Metadata } from 'next';
import { Suspense } from 'react';
import VerifyContractClient from './verify-contract-client';
import { sharedMetadata } from '../lib/seo/metaData';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'Verify Contract | ZondScan',
  description: 'Verify and publish a Hyperion smart contract on the QRL 2.0 network.',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: 'https://zondscan.com/verify-contract',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'Verify Contract | ZondScan',
    description: 'Verify and publish a Hyperion smart contract on the QRL 2.0 network.',
    url: 'https://zondscan.com/verify-contract',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'Verify Contract | ZondScan',
    description: 'Verify and publish a Hyperion smart contract on the QRL 2.0 network.',
  },
};

export default function VerifyContractPage(): JSX.Element {
  return (
    <div className="mx-auto max-w-4xl px-3 md:px-4 py-4 md:py-8">
      <div className="card-simple p-4 md:p-6 lg:p-8 space-y-4 md:space-y-5">
        <div>
          <h1 className="text-lg md:section-title">Verify smart contract</h1>
          <p className="text-xs md:text-sm text-text-secondary mt-1">
            Submit your contract&apos;s source code. The backend re-compiles it with the pinned Hyperion build and byte-matches the result against the on-chain runtime code. On success the source, ABI, and compiler settings are published on the contract&apos;s page.
          </p>
        </div>
        {/* `useSearchParams()` requires a Suspense boundary during static
            export so the page can pre-render shell HTML without the
            client-side query string. */}
        <Suspense fallback={<div className="text-sm text-text-secondary">Loading form…</div>}>
          <VerifyContractClient />
        </Suspense>
      </div>
    </div>
  );
}
