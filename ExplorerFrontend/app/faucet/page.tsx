import FaucetClient from './faucet-client';
import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'Testnet Faucet | QRL 2.0 Explorer',
  description:
    'Claim free QRL 2.0 testnet funds to experiment with the Quantum Resistant Ledger network — send transactions, deploy contracts, and explore Zond.',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: 'https://zondscan.com/faucet',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'Testnet Faucet | QRL 2.0 Explorer',
    description:
      'Claim free QRL 2.0 testnet funds to experiment with the Quantum Resistant Ledger network.',
    url: 'https://zondscan.com/faucet',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'Testnet Faucet | QRL 2.0 Explorer',
    description:
      'Claim free QRL 2.0 testnet funds to experiment with the Quantum Resistant Ledger network.',
  },
};

export default function FaucetPage(): JSX.Element {
  return (
    <main aria-labelledby="faucet-heading">
      <h1 id="faucet-heading" className="sr-only">QRL 2.0 Testnet Faucet</h1>
      <FaucetClient />
    </main>
  );
}
