"use client"

import * as React from 'react';
import { Buffer } from 'buffer';
import Link from 'next/link';
import Box from '@mui/material/Box';
import Typography from '@mui/material/Typography';

interface ContractData {
  id: number;
  from: string;
  txHash: string;
  pk: string;
  signature: string;
  nonce: string;
  value: string;
  contractAddress: string;
}

interface ContractsClientProps {
  initialData: any[];
}

const ITEMS_PER_PAGE = 10;

const truncateMiddle = (str: string, startChars = 6, endChars = 6): string => {
  if (str.length <= startChars + endChars) return str;
  return `${str.slice(0, startChars)}...${str.slice(-endChars)}`;
};

const DecoderAddress = (params: { row: { from: string } }): string => {
  try {
    const buffer = Buffer.from(params.row.from, 'base64');
    const bufString = buffer.toString('hex');
    return "0x" + bufString;
  } catch (error) {
    console.error('Error decoding address:', error);
    return 'Error decoding address';
  }
};

const DecoderTxHash = (params: { row: { txHash: string } }): string => {
  try {
    const buffer = Buffer.from(params.row.txHash, 'base64');
    const bufString = buffer.toString('hex');
    return "0x" + bufString;
  } catch (error) {
    console.error('Error decoding transaction hash:', error);
    return 'Error decoding transaction hash';
  }
};

const Decoder = (params: { row: { pk?: string; signature?: string } }): string => {
  try {
    const buffer = Buffer.from(params.row.pk || params.row.signature || '', 'base64');
    const bufString = buffer.toString('hex');
    return "0x" + bufString;
  } catch (error) {
    console.error('Error decoding data:', error);
    return 'Error decoding data';
  }
};

function Pagination({ 
  currentPage, 
  totalPages, 
  onPageChange 
}: { 
  currentPage: number; 
  totalPages: number; 
  onPageChange: (page: number) => void;
}) {
  return (
    <div className="flex justify-center gap-2 mt-4">
      <button
        onClick={() => onPageChange(currentPage - 1)}
        disabled={currentPage === 0}
        className="px-3 py-1 rounded bg-[#1a1b1e] text-white disabled:opacity-50"
      >
        Previous
      </button>
      <span className="px-3 py-1 text-white">
        Page {currentPage + 1} of {totalPages}
      </span>
      <button
        onClick={() => onPageChange(currentPage + 1)}
        disabled={currentPage >= totalPages - 1}
        className="px-3 py-1 rounded bg-[#1a1b1e] text-white disabled:opacity-50"
      >
        Next
      </button>
    </div>
  );
}

function CustomTable({ data }: { data: ContractData[] }) {
  return (
    <div className="overflow-x-auto">
      <table className="min-w-full bg-[#2c2d31] rounded-lg">
        <thead>
          <tr className="bg-[rgba(0,0,0,0.2)] text-white">
            <th className="px-4 py-3 text-left">From (Contract Creator)</th>
            <th className="px-4 py-3 text-left">Transaction Hash</th>
            <th className="px-4 py-3 text-left">Public Key</th>
            <th className="px-4 py-3 text-left">Signature</th>
            <th className="px-4 py-3 text-left">Nonce</th>
            <th className="px-4 py-3 text-left">Value</th>
            <th className="px-4 py-3 text-left">Contract Address</th>
          </tr>
        </thead>
        <tbody>
          {data.map((row) => (
            <tr key={row.id} className="border-t border-[rgba(255,255,255,0.1)] hover:bg-[rgba(255,255,255,0.04)]">
              <td className="px-4 py-3">
                <Link href={`/address/${row.from}`} className="text-[#ffa729] hover:text-[#ffb954] transition-colors">
                  {truncateMiddle(row.from, 8, 8)}
                </Link>
              </td>
              <td className="px-4 py-3">
                <Link href={`/tx/${row.txHash}`} className="text-[#ffa729] hover:text-[#ffb954] transition-colors">
                  {truncateMiddle(row.txHash, 8, 8)}
                </Link>
              </td>
              <td className="px-4 py-3 text-white">{truncateMiddle(row.pk, 8, 8)}</td>
              <td className="px-4 py-3 text-white">{truncateMiddle(row.signature, 8, 8)}</td>
              <td className="px-4 py-3 text-white">{row.nonce}</td>
              <td className="px-4 py-3 text-white">{row.value}</td>
              <td className="px-4 py-3">
                <Link href={`/address/${row.contractAddress}`} className="text-[#ffa729] hover:text-[#ffb954] transition-colors">
                  {truncateMiddle(row.contractAddress, 8, 8)}
                </Link>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function SearchInput({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  return (
    <input
      type="text"
      placeholder="Search..."
      value={value}
      onChange={(e) => onChange(e.target.value)}
      className="w-full max-w-md px-4 py-2 mb-4 rounded-lg bg-[#1a1b1e] text-white border border-[rgba(255,255,255,0.1)] focus:outline-none focus:border-[#ffa729]"
    />
  );
}

export default function ContractsClient({ initialData }: ContractsClientProps) {
  const [filterValue, setFilterValue] = React.useState('');
  const [currentPage, setCurrentPage] = React.useState(0);

  const transformedData = React.useMemo(() => {
    return initialData.map((item: any, index: number) => ({
      id: index,
      ...item,
      from: DecoderAddress({row: item}),
      txHash: DecoderTxHash({row: item}),
      pk: Decoder({row: { pk: item.pk }}),
      signature: Decoder({row: { signature: item.signature }}),
      contractAddress: DecoderAddress({row: item})
    }));
  }, [initialData]);

  const filteredData = React.useMemo(() => {
    if (!filterValue) return transformedData;
    
    return transformedData.filter(row => 
      Object.values(row).some(value => 
        value?.toString().toLowerCase().includes(filterValue.toLowerCase())
      )
    );
  }, [transformedData, filterValue]);

  const paginatedData = React.useMemo(() => {
    const startIndex = currentPage * ITEMS_PER_PAGE;
    return filteredData.slice(startIndex, startIndex + ITEMS_PER_PAGE);
  }, [filteredData, currentPage]);

  const totalPages = Math.ceil(filteredData.length / ITEMS_PER_PAGE);

  if (!initialData || initialData.length === 0) {
    return (
      <Box className="p-4 text-center">
        <Typography>No contracts found.</Typography>
      </Box>
    );
  }

  return (
    <div className="p-4">
      <Typography 
        variant="h5" 
        component="h1" 
        className="mb-6 text-center font-bold text-gray-900 dark:text-white"
      >
        Smart Contracts
      </Typography>
      <div className="mb-4">
        <SearchInput value={filterValue} onChange={setFilterValue} />
      </div>
      <div className="overflow-hidden rounded-lg shadow-lg">
        <CustomTable data={paginatedData} />
      </div>
      <Pagination 
        currentPage={currentPage}
        totalPages={totalPages}
        onPageChange={setCurrentPage}
      />
    </div>
  );
}
