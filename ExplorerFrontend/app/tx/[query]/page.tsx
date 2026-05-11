import { notFound, redirect } from 'next/navigation';
import TransactionView from './transaction-view';
import type { TransactionDetails } from '@/app/types';
import config from '../../../config';

interface PageProps {
  params: Promise<{ query: string }>;
}

function isEmptyTransaction(txData: any): boolean {
  return !txData.TxHash && 
         !txData.From && 
         !txData.To && 
         (!txData.Value || txData.Value === '0x0') &&
         (!txData.BlockNumber || txData.BlockNumber === '0x0');
}

async function getTransaction(txHash: string): Promise<TransactionDetails> {
  // Validate transaction hash format
  const hashRegex = /^0x[0-9a-fA-F]{64}$/;
  if (!hashRegex.test(txHash)) {
    throw new Error('Invalid transaction hash format');
  }

  const response = await fetch(`${config.handlerUrl}/tx/${txHash}`, {
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

  // Check if we have a valid transaction response
  if (!data.response || isEmptyTransaction(data.response)) {
    throw new Error('Transaction not found');
  }

  const txData = data.response;

  // Helper function to handle hex values
  const ensureHexString = (value: string | null | undefined): string => {
    if (!value) return '0x0';
    return value.startsWith('0x') ? value : `0x${value}`;
  };

  const transaction: TransactionDetails = {
    hash: txData.TxHash,
    blockNumber: txData.BlockNumber ? parseInt(txData.BlockNumber, 16) : 0,
    from: txData.From,
    to: txData.To,
    value: ensureHexString(txData.Value),
    timestamp: txData.BlockTimestamp ? parseInt(txData.BlockTimestamp, 16) : 0,
    gasUsed: ensureHexString(txData.GasUsed),
    gasPrice: ensureHexString(txData.GasPrice),
    nonce: txData.Nonce ? parseInt(txData.Nonce, 16) : 0,
    latestBlock: data.latestBlock,
    PaidFees: txData.PaidFees ? Number(txData.PaidFees) : undefined,
    contractCreated: data.contractCreated || undefined,
    tokenTransfer: data.tokenTransfer || undefined
  };

  return transaction;
}

async function isPendingTransaction(txHash: string): Promise<boolean> {
  try {
    const pendingResponse = await fetch(`${config.handlerUrl}/pending-transaction/${txHash}`);
    if (!pendingResponse.ok || pendingResponse.status !== 200) return false;
    const pendingData = await pendingResponse.json();
    return Boolean(pendingData?.transaction && pendingData.transaction.status === 'pending');
  } catch (error) {
    console.error('Error checking pending transaction:', error);
    return false;
  }
}

export default async function TransactionPage({ params }: PageProps): Promise<JSX.Element> {
  const resolvedParams = await params;
  const txHash = resolvedParams.query;

  const hashRegex = /^0x[0-9a-fA-F]{64}$/;
  if (!hashRegex.test(txHash)) {
    return (
      <div className="container mx-auto px-4">
        <div className="bg-red-900/20 border border-red-500/50 rounded-xl p-6 shadow-lg mt-6">
          <h2 className="text-red-500 font-semibold mb-2">Invalid Transaction Hash</h2>
          <p className="text-gray-300">
            The provided transaction hash is not in the correct format.
            Transaction hashes should start with &apos;0x&apos; followed by 64 hexadecimal characters.
          </p>
        </div>
      </div>
    );
  }

  // redirect() throws a NEXT_REDIRECT sentinel that the framework must
  // receive uncaught — keep this call outside any try/catch.
  if (await isPendingTransaction(txHash)) {
    redirect(`/pending/tx/${txHash}`);
  }

  try {
    const transaction = await getTransaction(txHash);
    return <TransactionView transaction={transaction} />;
  } catch (error) {
    console.error('Error in TransactionPage:', error);
    return (
      <div className="container mx-auto px-4">
        <div className="bg-red-900/20 border border-red-500/50 rounded-xl p-6 shadow-lg mt-6">
          <h2 className="text-red-500 font-semibold mb-2">Transaction Not Found</h2>
          <p className="text-gray-300">
            The transaction could not be found. This could mean:
          </p>
          <ul className="list-disc ml-6 mt-2 text-gray-300">
            <li>The transaction hash is incorrect</li>
            <li>The transaction has not been mined yet</li>
            <li>The transaction was dropped from the network</li>
          </ul>
        </div>
      </div>
    );
  }
}
