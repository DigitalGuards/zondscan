import axios from 'axios';
import config from '../../../../config';
import type { PendingTransaction } from '@/app/types';
import PendingTransactionView from './pending-transaction-view';

interface PageProps {
  params: Promise<{ hash: string }>;
}

function validateTransactionHash(hash: string): boolean {
  const hashRegex = /^0x[0-9a-fA-F]+$/;
  return hashRegex.test(hash);
}

async function getTransactionStatus(hash: string): Promise<{ 
  status: 'pending' | 'mined' | 'dropped';
  transaction: PendingTransaction | null;
  blockNumber?: string;
}> {
  try {
    // First try pending transactions endpoint
    const response = await axios.get(`${config.handlerUrl}/pending-transaction/${hash}`);
    console.log('Transaction status response:', response.data);
    
    if (!response.data?.transaction) {
      return { status: 'dropped', transaction: null };
    }

    const tx = response.data.transaction;
    return {
      status: tx.status,
      transaction: tx,
      blockNumber: tx.blockNumber
    };
  } catch (error: any) {
    console.error('Error fetching transaction status:', error);
    
    // If we got a 404, check if it exists in regular transactions
    if (error.response?.status === 404) {
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
              chainId: '0x7e7e', // Zond chainId
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

export default async function PendingTransactionPage({ params }: PageProps): Promise<JSX.Element> {
  try {
    const resolvedParams = await params;
    const hash = resolvedParams.hash;
    console.log('Transaction hash:', hash);

    if (!validateTransactionHash(hash)) {
      return (
        <div className="container mx-auto px-4">
          <div className="bg-red-900/20 border border-red-500/50 rounded-xl p-6 shadow-lg mt-6">
            <h2 className="text-red-500 font-semibold mb-2">Invalid Transaction Hash</h2>
            <p className="text-gray-300">
              The provided transaction hash is not in the correct format. 
              Transaction hashes should start with &apos;0x&apos; followed by hexadecimal characters.
            </p>
          </div>
        </div>
      );
    }

    const { status, transaction } = await getTransactionStatus(hash);

    // If transaction is mined, use window.location for client-side redirect
    if (status === 'mined' && transaction) {
      return (
        <>
          <script
            dangerouslySetInnerHTML={{
              __html: `window.location.href = '/tx/${hash}';`,
            }}
          />
          <div>Redirecting to transaction page...</div>
        </>
      );
    }

    // If transaction is dropped
    if (status === 'dropped' || !transaction) {
      return (
        <div className="container mx-auto px-4">
          <div className="bg-red-900/20 border border-red-500/50 rounded-xl p-6 shadow-lg mt-6">
            <h2 className="text-red-500 font-semibold mb-2">Transaction Not Found</h2>
            <p className="text-gray-300">
              This transaction is no longer in the mempool. It may have been dropped 
              or replaced. Please check if a transaction with a higher gas price was 
              submitted with the same nonce.
            </p>
          </div>
        </div>
      );
    }

    // Transaction is pending
    return <PendingTransactionView pendingTx={transaction} />;
    
  } catch (error) {
    console.error('Error in PendingTransactionPage:', error);
    return (
      <div className="container mx-auto px-4">
        <div className="bg-red-900/20 border border-red-500/50 rounded-xl p-6 shadow-lg mt-6">
          <h2 className="text-red-500 font-semibold mb-2">Error</h2>
          <p className="text-gray-300">
            An error occurred while fetching the transaction. Please try again later.
          </p>
        </div>
      </div>
    );
  }
}
