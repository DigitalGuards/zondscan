import { Suspense } from 'react';
import ValidatorsWrapper from './validators-client';

export const metadata = {
  title: 'Validators | QRL Explorer',
  description: 'View active validators, their ages, uptime, and staking information on the QRL network',
};

export default async function ValidatorsPage() {
  return (
    <main>
      <h1 className="sr-only">QRL Network Validators</h1>
      <Suspense fallback={<div className="p-4 text-center">Loading validators...</div>}>
        <ValidatorsWrapper />
      </Suspense>
    </main>
  );
}
