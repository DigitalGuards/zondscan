import { Suspense } from 'react';
import ContractsWrapper from './contracts-wrapper';
import config from '../../config.js';

interface ContractResponse {
  response: any[];
  total: number;
}

async function getContracts(page: number = 0, limit: number = 10): Promise<ContractResponse> {
  try {
    const response = await fetch(`${config.handlerUrl}/contracts?page=${page}&limit=${limit}`, {
      next: { revalidate: 10 },
    });

    if (!response.ok) {
      throw new Error(`Failed to fetch contracts: ${response.status}`);
    }

    const data = await response.json();
    return {
      response: data.response || [],
      total: data.total || 0
    };
  } catch (error) {
    console.error('Error fetching contracts:', error);
    return {
      response: [],
      total: 0
    };
  }
}

export const metadata = {
  title: 'Smart Contracts | QRL Explorer',
  description: 'View all smart contracts deployed on the QRL network',
};

export default async function ContractsPage() {
  const { response: initialData, total } = await getContracts();

  return (
    <Suspense fallback={<div className="p-4 text-center">Loading contracts...</div>}>
      <ContractsWrapper 
        initialData={initialData} 
        totalContracts={total} 
      />
    </Suspense>
  );
}
