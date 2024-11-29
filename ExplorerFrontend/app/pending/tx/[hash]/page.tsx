import React from 'react';
import axios from 'axios';
import config from '../../../../config';
import { redirect } from 'next/navigation';
import { PendingTransaction } from '../types';
import PendingTransactionView from './pending-transaction-view';

interface PageProps {
  params: Promise<{ hash: string }>;
}

function validateTransactionHash(hash: string): boolean {
  const hashRegex = /^0x[0-9a-fA-F]+$/;
  return hashRegex.test(hash);
}

async function checkIfMined(hash: string): Promise<boolean> {
  console.log(`Checking if transaction ${hash} is mined...`);
  try {
    const txResponse = await axios.get(`${config.handlerUrl}/tx/${hash}`);
    console.log('Mined check response:', txResponse.data);
    
    if (txResponse.data?.response && 
        txResponse.data.response.blockNumber) {
      console.log('Transaction found in blockchain');
      return true;
    }
    
    return false;
  } catch (error) {
    console.log('Transaction not found in blockchain:', error);
    return false;
  }
}

async function findPendingTransaction(hash: string): Promise<PendingTransaction | null> {
  console.log(`Looking for transaction ${hash} in mempool...`);
  try {
    const response = await axios.get(`${config.handlerUrl}/pending-transactions`);
    console.log('Pending transactions response:', response.data);
    const pendingData = response.data;

    if (pendingData?.pending) {
      for (const [address, nonceTxs] of Object.entries(pendingData.pending)) {
        if (typeof nonceTxs === 'object') {
          for (const [nonce, tx] of Object.entries(nonceTxs as any)) {
            if (tx && typeof tx === 'object' && 'hash' in tx && tx.hash === hash) {
              console.log('Found matching transaction in mempool');
              return tx as PendingTransaction;
            }
          }
        }
      }
    }
    console.log('Transaction not found in mempool');
    return null;
  } catch (error) {
    console.error('Error fetching pending transactions:', error);
    throw new Error('Failed to fetch pending transactions');
  }
}

async function getTransactionStatus(hash: string): Promise<{ 
  isPending: boolean; 
  pendingTx: PendingTransaction | null; 
  isMined: boolean;
  leftMempool: boolean;
}> {
  console.log(`Getting status for transaction ${hash}...`);
  
  if (!validateTransactionHash(hash)) {
    console.error('Invalid transaction hash format');
    throw new Error('Invalid transaction hash format');
  }

  try {
    const [isMined, pendingTx] = await Promise.all([
      checkIfMined(hash),
      findPendingTransaction(hash)
    ]);

    if (isMined) {
      return { 
        isPending: false, 
        pendingTx: null,
        isMined: true,
        leftMempool: false
      };
    }

    // If not mined and not in mempool, it has likely left mempool and is being processed
    if (!pendingTx) {
      return {
        isPending: true,
        pendingTx: null,
        isMined: false,
        leftMempool: true
      };
    }

    return { 
      isPending: true, 
      pendingTx,
      isMined: false,
      leftMempool: false
    };

  } catch (error) {
    console.error('Error in getTransactionStatus:', error);
    throw error;
  }
}

export default async function PendingTransactionPage({ params }: PageProps) {
  console.log('Rendering PendingTransactionPage');
  
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

    const { isPending, pendingTx, isMined, leftMempool } = await getTransactionStatus(hash);

    if (isMined) {
      console.log('Transaction is mined');
      return (
        <div className="container mx-auto px-4">
          <div className="bg-green-900/20 border border-green-500/50 rounded-xl p-6 shadow-lg mt-6">
            <h2 className="text-green-500 font-semibold mb-2">Transaction Confirmed</h2>
            <p className="text-gray-300 mb-4">
              This transaction has been mined and is now part of the blockchain.
            </p>
            <a 
              href={`/tx/${hash}`}
              className="inline-block bg-green-500/20 text-green-500 px-4 py-2 rounded-lg hover:bg-green-500/30 transition-colors"
            >
              View Transaction Details →
            </a>
          </div>
        </div>
      );
    }

    if (leftMempool) {
      return (
        <div className="container mx-auto px-4">
          <div className="bg-yellow-900/20 border border-yellow-500/50 rounded-xl p-6 shadow-lg mt-6">
            <h2 className="text-yellow-500 font-semibold mb-2">Transaction Processing</h2>
            <p className="text-gray-300 mb-2">
              This transaction has left the mempool and is likely being processed into a block.
              Please wait while the explorer fetches the block data.
            </p>
            <p className="text-gray-300">
              Transaction Hash: <span className="font-mono">{hash}</span>
            </p>
            <p className="text-gray-300 mt-4 text-sm">
              This page will automatically refresh every 10 seconds to check for updates.
            </p>
          </div>
          <meta httpEquiv="refresh" content="10" />
        </div>
      );
    }

    return (
      <>
        <PendingTransactionView pendingTx={pendingTx!} />
        <meta httpEquiv="refresh" content="10" />
      </>
    );

  } catch (error) {
    console.error('Error in page component:', error);
    return (
      <div className="container mx-auto px-4">
        <div className="bg-red-900/20 border border-red-500/50 rounded-xl p-6 shadow-lg mt-6">
          <h2 className="text-red-500 font-semibold mb-2">Error Loading Transaction</h2>
          <p className="text-gray-300">
            There was an error loading the transaction details: {error instanceof Error ? error.message : 'Unknown error'}
          </p>
          <p className="text-gray-300 mt-2">
            This page will automatically refresh every 10 seconds to check for updates.
          </p>
        </div>
        <meta httpEquiv="refresh" content="10" />
      </div>
    );
  }
}
