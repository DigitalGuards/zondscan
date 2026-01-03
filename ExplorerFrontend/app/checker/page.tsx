import CheckerClient from './checker-client';
import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';


export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'Balance Checker | QRL Zond Explorer',
  description:
    'Check wallet balances on the Quantum Resistant Ledger network with our intuitive balance checker tool. Verify holdings quickly and accurately.',
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'Balance Checker | QRL Zond Explorer',
    description:
      'Check wallet balances on the Quantum Resistant Ledger network with our intuitive balance checker tool. Verify holdings quickly and accurately.',
    url: 'https://zondscan.com/balance-checker',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'Balance Checker | QRL Zond Explorer',
    description:
      'Check wallet balances on the Quantum Resistant Ledger network with our intuitive balance checker tool. Verify holdings quickly and accurately.',
  },
};


export default function BalanceChecker(): JSX.Element {
  return <CheckerClient />;
}
