import { Suspense } from 'react';
import axios from 'axios';
import ContractsWrapper from './contracts-wrapper';
import config from '../../config.js';

export const metadata = {
  title: 'Smart Contracts | QRL Explorer',
  description: 'View all smart contracts deployed on the QRL network',
};

async function getContracts() {
  try {
    const response = await axios.get(`${config.handlerUrl}/contracts`, {
      timeout: 30000
    });
    return response.data?.response || [];
  } catch (error) {
    console.error('Error fetching contracts:', error);
    return [];
  }
}

export default async function ContractsPage() {
  const contracts = await getContracts();

  return (
    <Suspense fallback={<div className="p-4 text-center">Loading contracts...</div>}>
      <ContractsWrapper initialData={contracts} />
    </Suspense>
  );
}
