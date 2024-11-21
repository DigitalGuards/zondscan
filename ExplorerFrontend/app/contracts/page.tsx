import { Suspense } from 'react';
import ContractsWrapper from './contracts-wrapper';

export const metadata = {
  title: 'Smart Contracts | QRL Explorer',
  description: 'View all smart contracts deployed on the QRL network',
};

export default async function ContractsPage() {
  return (
    <Suspense fallback={<div className="p-4 text-center">Loading contracts...</div>}>
      <ContractsWrapper />
    </Suspense>
  );
}
