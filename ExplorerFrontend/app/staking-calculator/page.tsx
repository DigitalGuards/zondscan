import { Suspense } from 'react';
import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';
import StakingCalculatorClient from './staking-calculator-client';

const title = 'QRL Staking Calculator: Validator Rewards and APY | ZondScan';
const description =
  'Estimate QRL 2.0 validator rewards. Model APR, real yield after inflation, and reward payouts from live network data, using the beacon-chain reward formula.';

export const metadata: Metadata = {
  ...sharedMetadata,
  title,
  description,
  alternates: {
    ...sharedMetadata.alternates,
    canonical: 'https://zondscan.com/staking-calculator',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title,
    description,
    url: 'https://zondscan.com/staking-calculator',
    siteName: 'ZondScan',
    type: 'website',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title,
    description,
  },
};

export default function StakingCalculatorPage(): JSX.Element {
  return (
    <main aria-labelledby="staking-calculator-heading">
      <h1 id="staking-calculator-heading" className="sr-only">
        QRL Staking Calculator
      </h1>
      {/* useSearchParams reads the shared-scenario params, which needs a
          Suspense boundary under static prerendering. */}
      <Suspense fallback={null}>
        <StakingCalculatorClient />
      </Suspense>
    </main>
  );
}
