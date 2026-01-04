import { Suspense } from 'react';
import ContractsWrapper from './contracts-wrapper';
import config from '../../config.js';
import { sharedMetadata } from '../lib/seo/metaData';
import type { Metadata } from 'next';

interface ContractResponse {
  response: any[];
  total: number;
}

async function getContracts(page: number = 0, limit: number = 15, isToken: boolean = true): Promise<ContractResponse> {
  try {
    const response = await fetch(`${config.handlerUrl}/contracts?page=${page}&limit=${limit}&isToken=${isToken}`, {
      next: { revalidate: 10 },
    });

    if (!response.ok) {
      console.error(`Failed to fetch contracts: ${response.status} ${response.statusText}`);
      return {
        response: [],
        total: 0
      };
    }

    // Check content type to ensure we're getting JSON
    const contentType = response.headers.get('content-type');
    if (!contentType || !contentType.includes('application/json')) {
      console.error(`Expected JSON but got ${contentType}`);
      const text = await response.text();
      console.error(`Response body: ${text.substring(0, 200)}...`);
      return {
        response: [],
        total: 0
      };
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

export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'Smart Contracts | QRL Explorer',
  description: 'View all smart contracts deployed on the QRL network',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: 'https://zondscan.com/smart-contracts',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'Smart Contracts | QRL Explorer',
    description: 'View all smart contracts deployed on the QRL network',
    url: 'https://zondscan.com/smart-contracts',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'Smart Contracts | QRL Explorer',
    description: 'View all smart contracts deployed on the QRL network',
  },
};

export default async function ContractsPage(): Promise<JSX.Element> {
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
