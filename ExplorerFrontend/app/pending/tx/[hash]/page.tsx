import type { Metadata } from 'next';
import axios from 'axios';
import { redirect } from 'next/navigation';
import config from '../../../../config';
import type { PendingTransaction } from '@/app/types';
import { sharedMetadata } from '@/app/lib/seo/metaData';
import { isTxHash } from '@/app/lib/helpers';
import PendingTransactionView from './pending-transaction-view';

interface PageProps {
  params: Promise<{ hash: string }>;
}

// Per-pending-tx metadata. Same pattern as the confirmed tx page;
// shared pending-tx links are common during mempool congestion ("hey,
// is my tx going through?"), so previews carrying the truncated hash +
// "Pending Transaction" disambiguator are worth the small extra surface.
export async function generateMetadata({ params }: PageProps): Promise<Metadata> {
  const resolvedParams = await params;
  const hash = resolvedParams.hash;
  const shortHash = hash.length > 16
    ? `${hash.slice(0, 10)}...${hash.slice(-6)}`
    : hash;
  const canonicalUrl = `https://zondscan.com/pending/tx/${hash}`;
  const title = `Pending Transaction ${shortHash} | ZondScan`;
  const description = `Track QRL 2.0 pending transaction ${shortHash}: mempool status, ETA to inclusion, gas price vs median, and decoded calldata.`;

  return {
    ...sharedMetadata,
    title,
    description,
    alternates: {
      ...sharedMetadata.alternates,
      canonical: canonicalUrl,
    },
    openGraph: {
      ...sharedMetadata.openGraph,
      title,
      description,
      url: canonicalUrl,
      siteName: 'ZondScan',
      type: 'website',
    },
    twitter: {
      ...sharedMetadata.twitter,
      title,
      description,
    },
  };
}

function validateTransactionHash(hash: string): boolean {
  return isTxHash(hash);
}

async function getTransactionStatus(hash: string): Promise<{
  status: 'pending' | 'mined' | 'dropped';
  transaction: PendingTransaction | null;
  blockNumber?: string;
  // Verified-contract metadata for the tx's recipient, propagated from
  // /pending-transaction/:hash so the pending view can ABI-decode the
  // calldata when the target contract is source-verified.
  targetContract?: import('@/app/types').ContractMeta;
}> {
  try {
    // First try pending transactions endpoint
    const response = await axios.get(`${config.handlerUrl}/pending-transaction/${hash}`);

    if (!response.data?.transaction) {
      return { status: 'dropped', transaction: null };
    }

    const tx = response.data.transaction;
    return {
      status: tx.status,
      transaction: tx,
      blockNumber: tx.blockNumber,
      targetContract: response.data.targetContract,
    };
  } catch (error) {
    console.error('Error fetching transaction status:', error);

    // If we got a 404, check if it exists in regular transactions
    if (axios.isAxiosError(error) && error.response?.status === 404) {
      try {
        const txResponse = await axios.get(`${config.handlerUrl}/tx/${hash}`);
        if (txResponse.data?.response) {
          const tx = txResponse.data.response;
          return {
            status: 'mined',
            transaction: {
              hash: hash,
              status: 'mined',
              blockNumber: tx.blockNumber?.toString(),
              accessList: [],
              blockHash: null,
              chainId: '0x539', // QRL testnet v2 chainId (1337)
              from: tx.from || '',
              gas: tx.gas || '0x0',
              gasPrice: tx.gasPrice || '0x0',
              input: tx.input || '0x',
              nonce: tx.nonce?.toString() || '0',
              publicKey: tx.publicKey || '',
              to: tx.to,
              transactionIndex: null,
              type: tx.type || '0x0',
              value: tx.value || '0x0',
              lastSeen: Math.floor(Date.now() / 1000),
              createdAt: Math.floor(Date.now() / 1000)
            }
          };
        }
      } catch (txError) {
        console.error('Error checking regular transaction:', txError);
      }
    }
    
    return { status: 'dropped', transaction: null };
  }
}

function ErrorCard({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <div className="container mx-auto px-4">
      <div className="bg-red-900/20 border border-red-500/50 rounded-xl p-6 shadow-lg mt-6">
        <h2 className="text-red-500 font-semibold mb-2">{title}</h2>
        <p className="text-text-secondary">{children}</p>
      </div>
    </div>
  );
}

export default async function PendingTransactionPage({ params }: PageProps): Promise<JSX.Element> {
  const resolvedParams = await params;
  const hash = resolvedParams.hash;

  if (!validateTransactionHash(hash)) {
    return (
      <ErrorCard title="Invalid Transaction Hash">
        The provided transaction hash is not in the correct format.
        Transaction hashes should start with &apos;0x&apos; followed by hexadecimal characters.
      </ErrorCard>
    );
  }

  const { status, transaction, targetContract } = await getTransactionStatus(hash);

  // If transaction is mined, redirect to the confirmed transaction page
  if (status === 'mined' && transaction) {
    redirect(`/tx/${hash}`);
  }

  // If transaction is dropped
  if (status === 'dropped' || !transaction) {
    return (
      <ErrorCard title="Transaction Not Found">
        This transaction is no longer in the mempool. It may have been dropped
        or replaced. Please check if a transaction with a higher gas price was
        submitted with the same nonce.
      </ErrorCard>
    );
  }

  // Transaction is pending
  return <PendingTransactionView pendingTx={transaction} targetContract={targetContract} />;
}
