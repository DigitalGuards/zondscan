import React from 'react';
import axios from 'axios';
import config from '../../../../config';
import { redirect } from 'next/navigation';
import { PendingTransaction } from '../types';

interface PageProps {
  params: { hash: string };
}

async function getTransactionStatus(hash: string): Promise<{ isPending: boolean; pendingTx?: PendingTransaction }> {
  try {
    // First check if transaction is in mempool
    const pendingResponse = await axios.get(`${config.handlerUrl}/pending-transactions`);
    const pendingData = pendingResponse.data;

    // Search through pending transactions
    if (pendingData?.pending) {
      for (const [address, nonceTxs] of Object.entries(pendingData.pending)) {
        for (const [nonce, tx] of Object.entries(nonceTxs as any)) {
          if ((tx as PendingTransaction).hash === hash) {
            return { isPending: true, pendingTx: tx as PendingTransaction };
          }
        }
      }
    }

    // If not in mempool, check if it's been mined
    const txResponse = await axios.get(`${config.handlerUrl}/tx/${hash}`);
    if (txResponse.data) {
      // Transaction has been mined, redirect to regular tx page
      return { isPending: false };
    }

    // Transaction not found in either mempool or blockchain
    throw new Error('Transaction not found');

  } catch (error) {
    console.error('Error fetching transaction:', error);
    throw error;
  }
}

export default async function PendingTransactionPage({ params }: PageProps) {
  try {
    const { isPending, pendingTx } = await getTransactionStatus(params.hash);

    // If transaction is no longer pending, redirect to regular tx page
    if (!isPending) {
      redirect(`/tx/${params.hash}`);
    }

    if (!pendingTx) {
      throw new Error('Transaction not found');
    }

    // Format values for display
    const value = (parseInt(pendingTx.value, 16) / 1e18).toString();
    const gasPrice = (parseInt(pendingTx.gasPrice, 16) / 1e18).toString();
    const maxFeePerGas = pendingTx.maxFeePerGas ? (parseInt(pendingTx.maxFeePerGas, 16) / 1e18).toString() : null;
    const maxPriorityFeePerGas = pendingTx.maxPriorityFeePerGas ? (parseInt(pendingTx.maxPriorityFeePerGas, 16) / 1e18).toString() : null;

    return (
      <div className="container mx-auto px-4">
        <h1 className="text-2xl font-bold mb-6 text-[#ffa729]">Pending Transaction Details</h1>
        
        <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] rounded-xl p-6 mb-8 shadow-xl">
          <div className="space-y-4">
            <div className="flex flex-col space-y-1">
              <span className="text-gray-400">Transaction Hash</span>
              <span className="text-gray-200 break-all font-mono">{pendingTx.hash}</span>
            </div>

            <div className="flex flex-col space-y-1">
              <span className="text-gray-400">Status</span>
              <div className="bg-yellow-500/20 text-yellow-500 px-3 py-1 rounded-lg text-sm inline-block">
                Pending
              </div>
            </div>

            <div className="flex flex-col space-y-1">
              <span className="text-gray-400">From</span>
              <span className="text-gray-200 break-all font-mono">{pendingTx.from}</span>
            </div>

            {pendingTx.to && (
              <div className="flex flex-col space-y-1">
                <span className="text-gray-400">To</span>
                <span className="text-gray-200 break-all font-mono">{pendingTx.to}</span>
              </div>
            )}

            <div className="flex flex-col space-y-1">
              <span className="text-gray-400">Value</span>
              <span className="text-gray-200">{value} QRL</span>
            </div>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="flex flex-col space-y-1">
                <span className="text-gray-400">Gas Limit</span>
                <span className="text-gray-200">{parseInt(pendingTx.gas, 16).toLocaleString()}</span>
              </div>

              <div className="flex flex-col space-y-1">
                <span className="text-gray-400">Gas Price</span>
                <span className="text-gray-200">{gasPrice} QRL</span>
              </div>

              {maxFeePerGas && (
                <div className="flex flex-col space-y-1">
                  <span className="text-gray-400">Max Fee Per Gas</span>
                  <span className="text-gray-200">{maxFeePerGas} QRL</span>
                </div>
              )}

              {maxPriorityFeePerGas && (
                <div className="flex flex-col space-y-1">
                  <span className="text-gray-400">Max Priority Fee Per Gas</span>
                  <span className="text-gray-200">{maxPriorityFeePerGas} QRL</span>
                </div>
              )}
            </div>

            <div className="flex flex-col space-y-1">
              <span className="text-gray-400">Nonce</span>
              <span className="text-gray-200">{parseInt(pendingTx.nonce, 16)}</span>
            </div>

            {pendingTx.input && pendingTx.input !== '0x' && (
              <div className="flex flex-col space-y-1">
                <span className="text-gray-400">Input Data</span>
                <span className="text-gray-200 break-all font-mono">{pendingTx.input}</span>
              </div>
            )}
          </div>
        </div>

        <div className="bg-yellow-900/20 border border-yellow-500/50 rounded-xl p-6 shadow-lg">
          <h2 className="text-yellow-500 font-semibold mb-2">About Pending Transactions</h2>
          <p className="text-gray-300">
            This transaction is currently in the mempool waiting to be included in a block. 
            The details shown here may change when the transaction is mined, and there is no 
            guarantee that this transaction will be included in a block.
          </p>
        </div>
      </div>
    );
  } catch (error) {
    console.error('Error:', error);
    redirect('/404');
  }
}
