import CheckerClient from './checker-client';
import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';


export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'Balance Checker | QRL 2.0 Explorer',
  description:
    'Check wallet balances on the Quantum Resistant Ledger network with our intuitive balance checker tool. Verify holdings quickly and accurately.',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: 'https://zondscan.com/checker',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'Balance Checker | QRL 2.0 Explorer',
    description:
      'Check wallet balances on the Quantum Resistant Ledger network with our intuitive balance checker tool. Verify holdings quickly and accurately.',
    url: 'https://zondscan.com/checker',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'Balance Checker | QRL 2.0 Explorer',
    description:
      'Check wallet balances on the Quantum Resistant Ledger network with our intuitive balance checker tool. Verify holdings quickly and accurately.',
  },
};


export default function BalanceChecker(): JSX.Element {
  return (
    <main aria-labelledby="checker-heading">
      <h1 id="checker-heading" className="sr-only">QRL Balance Checker</h1>
      <CheckerClient />
    </main>
  );
}
