import { notFound } from 'next/navigation';
import TransactionView from './transaction-view';
import type { TransactionDetails } from './types';
import { decodeBase64ToHexadecimal } from '../../lib/server-helpers';

interface PageProps {
  params: Promise<{ query: string }>;
}

async function getTransaction(txHash: string): Promise<TransactionDetails> {
  // Validate transaction hash format
  const hashRegex = /^0x[0-9a-fA-F]{64}$/;
  if (!hashRegex.test(txHash)) {
    throw new Error('Invalid transaction hash format');
  }

  const response = await fetch(`${process.env.NEXT_PUBLIC_HANDLER_URL}/tx/${txHash}`, {
    method: 'GET',
    headers: { Accept: 'application/json' },
    next: { revalidate: 60 }, // Cache for 60 seconds
  });

  if (!response.ok) {
    if (response.status === 404) {
      notFound();
    }
    throw new Error('Failed to fetch transaction details');
  }

  const data = await response.json();
  if (!data.response) {
    notFound();
  }

  const txData = data.response;
  return {
    hash: txData.hash || txHash,
    blockNumber: txData.blockNumber || '',
    from: txData.from ? decodeBase64ToHexadecimal(txData.from) : '',
    to: txData.to ? decodeBase64ToHexadecimal(txData.to) : '',
    value: txData.value || '0',
    timestamp: txData.timestamp || 0,
    gasUsed: txData.gasUsed,
    gasPrice: txData.gasPrice,
    nonce: txData.nonce,
    latestBlock: data.latestBlock // Include latestBlock from response
  };
}

export default async function TransactionPage({ params }: PageProps): Promise<JSX.Element> {
  try {
    const resolvedParams = await params;
    const transaction = await getTransaction(resolvedParams.query);
    return <TransactionView transaction={transaction} />;
  } catch (error) {
    console.error('Error in TransactionPage:', error);
    return (
      <div className="py-8">
        <div className="bg-red-900/50 border border-red-500 text-red-200 px-6 py-4 rounded-xl" role="alert">
          <strong className="font-bold">Error: </strong>
          <span className="block sm:inline">
            {error instanceof Error ? error.message : 'Failed to load transaction details'}
          </span>
        </div>
      </div>
    );
  }
}
