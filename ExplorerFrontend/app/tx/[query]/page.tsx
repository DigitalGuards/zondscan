import type { Metadata } from 'next';
import { notFound, redirect } from 'next/navigation';
import TransactionView from './transaction-view';
import type { TransactionDetails } from '@/app/types';
import { sharedMetadata } from '@/app/lib/seo/metaData';
import { isTxHash } from '@/app/lib/helpers';
import config from '../../../config';

interface PageProps {
  params: Promise<{ query: string }>;
}

// Per-tx metadata so shared /tx/<hash> links surface a meaningful title +
// description in OG / Twitter previews. We deliberately don't fetch the
// tx body here (adds latency to crawlers + harms the page TTFB); a
// truncated hash + a stable description is enough for link previews.
// Mirrors the shape already used on /block/:n and /address/:addr.
export async function generateMetadata({ params }: PageProps): Promise<Metadata> {
  const resolvedParams = await params;
  const txHash = resolvedParams.query;
  const shortHash = txHash.length > 16
    ? `${txHash.slice(0, 10)}...${txHash.slice(-6)}`
    : txHash;
  const canonicalUrl = `https://zondscan.com/tx/${txHash}`;
  const title = `Transaction ${shortHash} | ZondScan`;
  const description = `View detailed information for QRL 2.0 transaction ${shortHash}. See from / to, value, gas, decoded events, and contract calls.`;

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

// Loose shape: the backend may return any subset of these fields and we
// just check whether the row is meaningful. Casting through `unknown` lets
// us narrow per-field without committing to a stricter Transaction type
// that's already represented elsewhere in app/types/transaction.ts.
type MaybeTxRecord = {
  TxHash?: string;
  From?: string;
  To?: string;
  Value?: string;
  BlockNumber?: string;
};

function isEmptyTransaction(txData: MaybeTxRecord): boolean {
  return !txData.TxHash &&
         !txData.From &&
         !txData.To &&
         (!txData.Value || txData.Value === '0x0') &&
         (!txData.BlockNumber || txData.BlockNumber === '0x0');
}

async function getTransaction(txHash: string): Promise<TransactionDetails> {
  // Validate transaction hash format
  if (!isTxHash(txHash)) {
    throw new Error('Invalid transaction hash format');
  }

  // The backend bundles `latestBlock` (from sync_state, dynamic) with the
  // tx record (immutable once mined) in the same payload. Caching the whole
  // response froze `latestBlock` for 60 s, so a freshly-mined tx viewed
  // mid-cache-window showed a stale confirmation count (e.g. "1 Confirmation")
  // and a refresh after expiry would jump dozens higher. Always fetch fresh.
  const response = await fetch(`${config.handlerUrl}/tx/${txHash}`, {
    method: 'GET',
    headers: { Accept: 'application/json' },
    cache: 'no-store',
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
    tokenTransfers: Array.isArray(data.tokenTransfers) ? data.tokenTransfers : undefined,
    // input comes from the top-level data.input field. The backend fetches
    // it from the node via qrl_getTransactionByHash because the syncer's
    // Transaction struct uses the wrong JSON tag (`data` instead of `input`),
    // leaving txData.Input empty for every historical tx.
    input: typeof data.input === 'string' ? data.input : (txData.Input || undefined),
    logs: Array.isArray(data.logs) ? data.logs : undefined,
    targetContract: data.targetContract || undefined,
    internalTransactions: Array.isArray(data.internalTransactions) ? data.internalTransactions : undefined,
    receiptStatus: typeof data.receiptStatus === 'string' ? data.receiptStatus : undefined,
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

  if (!isTxHash(txHash)) {
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
  // receive uncaught, keep this call outside any try/catch.
  if (await isPendingTransaction(txHash)) {
    redirect(`/pending/tx/${txHash}`);
  }

  // Constructing JSX inside the try/catch lets render-time errors escape
  // the catch block (React doesn't render synchronously), so the rule
  // react-hooks/error-boundaries flags it. Resolve the transaction first,
  // then return the JSX outside the try.
  let transaction;
  try {
    transaction = await getTransaction(txHash);
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
  return <TransactionView transaction={transaction} />;
}
