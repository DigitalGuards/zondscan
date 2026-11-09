import { SITE_NAME, pageTitle, siteUrl } from '../lib/site';
import { Suspense } from 'react';
import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';
import StakingCalculatorClient from './staking-calculator-client';

const title = pageTitle('QRL Staking Calculator: Validator Rewards and APY');
const description =
  'Estimate QRL 2.0 validator rewards. Model APR, real yield after inflation, and reward payouts from live network data, using the beacon-chain reward formula.';

export const metadata: Metadata = {
  ...sharedMetadata,
  title,
  description,
  alternates: {
    ...sharedMetadata.alternates,
    canonical: siteUrl('/staking-calculator'),
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title,
    description,
    url: siteUrl('/staking-calculator'),
    siteName: SITE_NAME,
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
