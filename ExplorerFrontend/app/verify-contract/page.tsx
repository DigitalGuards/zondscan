import type { Metadata } from 'next';
import { Suspense } from 'react';
import VerifyContractClient from './verify-contract-client';

export const metadata: Metadata = {
  title: 'Verify Contract | Zondscan',
  description: 'Verify and publish a Hyperion smart contract on the QRL Zond network.',
};

export default function VerifyContractPage(): JSX.Element {
  return (
    <div className="mx-auto max-w-4xl px-3 md:px-4 py-4 md:py-8">
      <div className="card-simple p-4 md:p-6 lg:p-8 space-y-4 md:space-y-5">
        <div>
          <h1 className="text-lg md:text-xl lg:text-2xl font-semibold text-accent">Verify smart contract</h1>
          <p className="text-xs md:text-sm text-gray-400 mt-1">
            Submit your contract&apos;s source code. The backend re-compiles it with the pinned Hyperion build and byte-matches the result against the on-chain runtime code. On success the source, ABI, and compiler settings are published on the contract&apos;s page.
          </p>
        </div>
        {/* `useSearchParams()` requires a Suspense boundary during static
            export so the page can pre-render shell HTML without the
            client-side query string. */}
        <Suspense fallback={<div className="text-sm text-gray-400">Loading form…</div>}>
          <VerifyContractClient />
        </Suspense>
      </div>
    </div>
  );
}
