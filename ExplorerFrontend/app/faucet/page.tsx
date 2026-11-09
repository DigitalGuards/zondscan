import { SITE_NAME, siteUrl } from '../lib/site';
import FaucetClient from './faucet-client';
import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: `QRL Testnet v3 Faucet | ${SITE_NAME}`,
  description:
    'Claim free QRL Testnet v3 funds to experiment with the Quantum Resistant Ledger network: send transactions, deploy contracts, and explore the post-quantum chain.',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: siteUrl('/faucet'),
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: `QRL Testnet v3 Faucet | ${SITE_NAME}`,
    description:
      'Claim free QRL Testnet v3 funds to experiment with the Quantum Resistant Ledger network.',
    url: siteUrl('/faucet'),
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: `QRL Testnet v3 Faucet | ${SITE_NAME}`,
    description:
      'Claim free QRL Testnet v3 funds to experiment with the Quantum Resistant Ledger network.',
  },
};

export default function FaucetPage(): JSX.Element {
  return (
    <main aria-labelledby="faucet-heading">
      <h1 id="faucet-heading" className="sr-only">QRL Testnet v3 Faucet</h1>
      <FaucetClient />
    </main>
  );
}
