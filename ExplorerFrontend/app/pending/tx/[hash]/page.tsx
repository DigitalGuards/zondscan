import React from 'react';
import axios from 'axios';
import config from '../../../../config';
import { redirect } from 'next/navigation';
import { PendingTransaction } from '../types';
import PendingTransactionView from './pending-transaction-view';

interface PageProps {
  params: Promise<{ hash: string }>;
}

async function sleep(ms: number) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

function validateTransactionHash(hash: string): boolean {
  // Just check if it starts with 0x and contains valid hex characters
  const hashRegex = /^0x[0-9a-fA-F]+$/;
  return hashRegex.test(hash);
}

async function checkIfMined(hash: string, retries = 3): Promise<boolean> {
  console.log(`Checking if transaction ${hash} is mined...`);
  for (let i = 0; i < retries; i++) {
    try {
      const txResponse = await axios.get(`${config.handlerUrl}/tx/${hash}`);
      console.log(`Mined check attempt ${i + 1}, response:`, txResponse.data);
      
      // Check if we have a valid transaction response
      if (txResponse.data?.response && 
          txResponse.data.response.hash && 
          txResponse.data.response.blockNumber) {
        console.log('Transaction found in blockchain');
        return true;
      }
      
      console.log('Transaction not found in blockchain');
      return false;
    } catch (error) {
      console.log(`Mined check attempt ${i + 1} failed:`, error);
      if (i === retries - 1) return false;
      // Wait longer between each retry
      await sleep(1000 * (i + 1));
    }
  }
  console.log('Transaction not found in blockchain after all retries');
  return false;
}

async function findPendingTransaction(hash: string): Promise<PendingTransaction | null> {
  console.log(`Looking for transaction ${hash} in mempool...`);
  try {
    const response = await axios.get(`${config.handlerUrl}/pending-transactions`);
    console.log('Pending transactions response:', response.data);
    const pendingData = response.data;

    if (pendingData?.pending) {
      for (const [address, nonceTxs] of Object.entries(pendingData.pending)) {
        console.log(`Checking address ${address}...`);
        if (typeof nonceTxs === 'object') {
          for (const [nonce, tx] of Object.entries(nonceTxs as any)) {
            console.log(`Checking nonce ${nonce}:`, tx);
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

async function getTransactionStatus(hash: string): Promise<{ isPending: boolean; pendingTx?: PendingTransaction; isMined: boolean }> {
  console.log(`Getting status for transaction ${hash}...`);
  
  // Validate transaction hash format
  if (!validateTransactionHash(hash)) {
    console.error('Invalid transaction hash format');
    throw new Error('Invalid transaction hash format');
  }

  try {
    // First check if transaction is in mempool
    const pendingTx = await findPendingTransaction(hash);
    if (pendingTx) {
      return { isPending: true, pendingTx, isMined: false };
    }

    // If not in mempool, check if it's been mined with retries
    const isMined = await checkIfMined(hash);
    return { isPending: false, isMined };

  } catch (error) {
    console.error('Error in getTransactionStatus:', error);
    throw error;
  }
}

export default async function PendingTransactionPage({ params }: PageProps) {
  console.log('Rendering PendingTransactionPage');
  
  try {
    // Await params before using them
    const resolvedParams = await params;
    const hash = resolvedParams.hash;
    console.log('Transaction hash:', hash);

    // Validate transaction hash format first
    if (!validateTransactionHash(hash)) {
      return (
        <div className="container mx-auto px-4">
          <div className="bg-red-900/20 border border-red-500/50 rounded-xl p-6 shadow-lg mt-6">
            <h2 className="text-red-500 font-semibold mb-2">Invalid Transaction Hash</h2>
            <p className="text-gray-300">
              The provided transaction hash is not in the correct format. 
              Transaction hashes should start with '0x' followed by hexadecimal characters.
            </p>
          </div>
        </div>
      );
    }

    const { isPending, pendingTx, isMined } = await getTransactionStatus(hash);

    // If transaction is confirmed to be mined, show a message with a link instead of redirecting
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

    // If transaction is not pending and not mined, show "Not Found" message with auto-refresh
    if (!isPending) {
      console.log('Transaction not found in mempool or blockchain');
      return (
        <div className="container mx-auto px-4">
          <div className="bg-yellow-900/20 border border-yellow-500/50 rounded-xl p-6 shadow-lg mt-6">
            <h2 className="text-yellow-500 font-semibold mb-2">Transaction Status Unknown</h2>
            <p className="text-gray-300 mb-4">
              This transaction was not found in the mempool or blockchain. This could mean:
            </p>
            <ul className="list-disc ml-6 mb-4 text-gray-300">
              <li>The transaction is still propagating through the network</li>
              <li>The transaction was dropped from the mempool</li>
              <li>The transaction hash is incorrect</li>
            </ul>
            <p className="text-gray-300">
              This page will automatically refresh every 10 seconds to check for updates.
            </p>
          </div>
          {/* Add auto-refresh meta tag */}
          <meta httpEquiv="refresh" content="10" />
        </div>
      );
    }

    if (!pendingTx) {
      throw new Error('Transaction not found');
    }

    // Use the client component to render pending transaction details
    return <PendingTransactionView pendingTx={pendingTx} />;

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
        {/* Add auto-refresh meta tag */}
        <meta httpEquiv="refresh" content="10" />
      </div>
    );
  }
}
