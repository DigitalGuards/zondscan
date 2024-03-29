"use client";

import axios from 'axios';
import { usePathname } from 'next/navigation'
import React, { useState, useEffect, useRef } from 'react';
import config from '../../../config';

type Block = {
  baseFeePerGas: number;
  gasLimit: number;
  gasUsed: number;
  hash: string;
  number: number;
  parentHash: string;
  receiptsRoot: string;
  stateRoot: string;
  timestamp: number;
  transactions: any[]; // Specify more detailed type if the structure of transactions is known
  transactionsRoot: string;
  difficulty: number;
  extraData: string;
  logsBloom: string;
  miner: string;
  mixHash: string;
  nonce: string;
  sha3Uncles: string;
  size: number;
  totalDifficulty: number;
  uncles: any[]; // Specify more detailed type if the structure of uncles is known
  withdrawals: any[]; // Specify more detailed type if the structure of withdrawals is known
  withdrawalsRoot: string;
};

export default function Home() {
  const [blockData, setBlockData] = useState({} as Block);
  const pathname = usePathname()

  function renderTransactions() {
    if (Array.isArray(blockData.transactions != null)) {
      return blockData.transactions.map((ptx, index) => (
        <div key={index} className="mb-4">
          Hash: {ptx.hash} <br />
          From: {ptx.from} <br />
        </div>
      ));
    } else {
      return "No transactions found";
    }
  }

  useEffect(() => {
    axios.get(`${config.handlerUrl}${pathname}`)
      .then(response => {
        setBlockData(response.data.response.result);
      })
      .catch(error => console.error('Error fetching block:', error));
  }, []);

  console.log(blockData);

  return (
    <>
        <div className="overflow-x-auto bg-gray-100 py-4 flex items-center justify-center">
          <div className="bg-white p-6 rounded-lg shadow-md w-full lg:w-5/6">
            <h1 className="text-2xl font-bold mb-8 border-b pb-4">Block {pathname?.replace("/block/", "")} - { new Date(blockData.timestamp * 1000).toLocaleDateString('en-GB', {
              day: '2-digit', month: 'short', year: 'numeric'
            }).replace(/ /g, ' ')
          }</h1>
            <table className="min-w-max w-full table-auto">
              <thead>
                <tr className="bg-gray-200 text-gray-600 uppercase text-sm leading-normal">
                  <th className="py-3 px-6 text-left">Field</th>
                  <th className="py-3 px-6 text-left">Value</th>
                </tr>
              </thead>
              <tbody className="text-gray-600 text-sm font-light">
                {blockData && (
                  <>
                    <tr className="border-b border-gray-200 hover:bg-gray-100 even:bg-gray-50">
                      <td className="py-3 px-6 text-left">Gas Limit</td>
                      <td className="py-3 px-6 text-left">{blockData.gasLimit}</td>
                    </tr>
                    <tr className="border-b border-gray-200 hover:bg-gray-100 even:bg-gray-50">
                      <td className="py-3 px-6 text-left">Total Difficulty</td>
                      <td className="py-3 px-6 text-left">{blockData.totalDifficulty}</td>
                    </tr>
                    <tr className="border-b border-gray-200 hover:bg-gray-100 even:bg-gray-50">
                      <td className="py-3 px-6 text-left">Gas Used</td>
                      <td className="py-3 px-6 text-left">{blockData.gasUsed}</td>
                    </tr>
                    <tr className="border-b border-gray-200 hover:bg-gray-100 even:bg-gray-50">
                      <td className="py-3 px-6 text-left">Gas Limit</td>
                      <td className="py-3 px-6 text-left">{blockData.gasLimit}</td>
                    </tr>
                    <tr className="border-b border-gray-200 hover:bg-gray-100 even:bg-gray-50">
                      <td className="py-3 px-6 text-left">Base Fee Per Gas</td>
                      <td className="py-3 px-6 text-left">{blockData.baseFeePerGas}</td>
                    </tr>
                    <tr className="border-b border-gray-200 hover:bg-gray-100 even:bg-gray-50">
                      <td className="py-3 px-6 text-left">Extra Data</td>
                      <td className="py-3 px-6 text-left">{blockData.extraData}</td>
                    </tr>
                    <tr className="border-b border-gray-200 hover:bg-gray-100 even:bg-gray-50">
                      <td className="py-3 px-6 text-left">Transactions</td>
                      <td className="py-3 px-6 text-left">
                      {renderTransactions()}
                      </td>
                    </tr>
                    <tr className="border-b border-gray-200 hover:bg-gray-100 even:bg-gray-50">
                      <td className="py-3 px-6 text-left">Hash</td>
                      <td className="py-3 px-6 text-left">{blockData.hash}</td>
                    </tr>
                    <tr className="border-b border-gray-200 hover:bg-gray-100 even:bg-gray-50">
                      <td className="py-3 px-6 text-left">Parent Hash</td>
                      <td className="py-3 px-6 text-left">{blockData.parentHash}</td>
                    </tr>
                    <tr className="border-b border-gray-200 hover:bg-gray-100 even:bg-gray-50">
                      <td className="py-3 px-6 text-left">Transactions Root</td>
                      <td className="py-3 px-6 text-left">{blockData.transactionsRoot}</td>
                    </tr>
                    <tr className="border-b border-gray-200 hover:bg-gray-100 even:bg-gray-50">
                      <td className="py-3 px-6 text-left">State Root</td>
                      <td className="py-3 px-6 text-left">{blockData.stateRoot}</td>
                    </tr>
                    <tr className="border-b border-gray-200 hover:bg-gray-100 even:bg-gray-50">
                      <td className="py-3 px-6 text-left">Receipts Root</td>
                      <td className="py-3 px-6 text-left">{blockData.receiptsRoot}</td>
                    </tr>
                    <tr className="border-b border-gray-200 hover:bg-gray-100 even:bg-gray-50">
                      <td className="py-3 px-6 text-left">Withdrawls Root</td>
                      <td className="py-3 px-6 text-left">{blockData.withdrawalsRoot}</td>
                    </tr>
                    <tr className="border-b border-gray-200 hover:bg-gray-100 even:bg-gray-50">
                      <td className="py-3 px-6 text-left">Nonce</td>
                      <td className="py-3 px-6 text-left">{blockData.nonce}</td>
                    </tr>
                    <tr className="border-b border-gray-200 hover:bg-gray-100 even:bg-gray-50">
                      <td className="py-3 px-6 text-left">Sha3Uncles</td>
                      <td className="py-3 px-6 text-left">{blockData.sha3Uncles}</td>
                    </tr>
                  </>
                )}
              </tbody>
            </table>
          </div>
        </div>
    </>
  );
}