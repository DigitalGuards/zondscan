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
    <div className="flex justify-center gap-4 mt-6">
      <button
        onClick={() => onPageChange(currentPage - 1)}
        disabled={currentPage === 0}
        className="px-4 py-2 rounded-lg bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] text-white border border-[#3d3d3d] hover:border-[#ffa729] disabled:opacity-50 disabled:hover:border-[#3d3d3d] transition-all duration-300"
      >
        Previous
      </button>
      <span className="px-4 py-2 text-white bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-lg border border-[#3d3d3d]">
        Page {currentPage + 1} of {totalPages}
      </span>
      <button
        onClick={() => onPageChange(currentPage + 1)}
        disabled={currentPage >= totalPages - 1}
        className="px-4 py-2 rounded-lg bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] text-white border border-[#3d3d3d] hover:border-[#ffa729] disabled:opacity-50 disabled:hover:border-[#3d3d3d] transition-all duration-300"
      >
        Next
      </button>
    </div>
  );
}

function CustomTable({ data }: { data: ContractData[] }) {
  return (
    <div className="overflow-x-auto rounded-lg border border-[#3d3d3d] bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]">
      <table className="min-w-full">
        <thead>
          <tr className="border-b border-[#3d3d3d]">
            <th className="px-6 py-4 text-left text-sm font-medium text-[#ffa729]">From (Contract Creator)</th>
            <th className="px-6 py-4 text-left text-sm font-medium text-[#ffa729]">Transaction Hash</th>
            <th className="px-6 py-4 text-left text-sm font-medium text-[#ffa729]">Public Key</th>
            <th className="px-6 py-4 text-left text-sm font-medium text-[#ffa729]">Signature</th>
            <th className="px-6 py-4 text-left text-sm font-medium text-[#ffa729]">Nonce</th>
            <th className="px-6 py-4 text-left text-sm font-medium text-[#ffa729]">Value</th>
            <th className="px-6 py-4 text-left text-sm font-medium text-[#ffa729]">Contract Address</th>
          </tr>
        </thead>
        <tbody>
          {data.map((row) => (
            <tr key={row.id} className="border-b border-[#3d3d3d] hover:bg-[rgba(255,167,41,0.05)] transition-colors">
              <td className="px-6 py-4 whitespace-nowrap">
                <Link href={`/address/${row.from}`} className="text-[#ffa729] hover:text-[#ffb954] transition-colors">
                  {truncateMiddle(row.from, 8, 8)}
                </Link>
              </td>
              <td className="px-6 py-4 whitespace-nowrap">
                <Link href={`/tx/${row.txHash}`} className="text-[#ffa729] hover:text-[#ffb954] transition-colors">
                  {truncateMiddle(row.txHash, 8, 8)}
                </Link>
              </td>
              <td className="px-6 py-4 whitespace-nowrap text-gray-300">{truncateMiddle(row.pk, 8, 8)}</td>
              <td className="px-6 py-4 whitespace-nowrap text-gray-300">{truncateMiddle(row.signature, 8, 8)}</td>
              <td className="px-6 py-4 whitespace-nowrap text-gray-300">{row.nonce}</td>
              <td className="px-6 py-4 whitespace-nowrap text-gray-300">{row.value}</td>
              <td className="px-6 py-4 whitespace-nowrap">
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
    <div className="relative">
      <input
        type="text"
        placeholder="Search contracts..."
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="w-full max-w-md px-4 py-3 rounded-lg bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] text-white border border-[#3d3d3d] focus:outline-none focus:border-[#ffa729] transition-all duration-300 pl-10"
      />
      <svg
        className="absolute left-3 top-1/2 transform -translate-y-1/2 h-5 w-5 text-[#ffa729]"
        fill="none"
        stroke="currentColor"
        viewBox="0 0 24 24"
        xmlns="http://www.w3.org/2000/svg"
      >
        <path
          strokeLinecap="round"
          strokeLinejoin="round"
          strokeWidth={2}
          d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"
        />
      </svg>
    </div>
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
      <Box className="p-8 text-center">
        <Typography className="text-gray-300">No contracts found.</Typography>
      </Box>
    );
  }

  return (
    <div className="p-8 max-w-[1200px] mx-auto">
      <Typography 
        variant="h5" 
        component="h1" 
        className="mb-8 text-center font-bold text-[#ffa729] text-2xl"
      >
        Smart Contracts
      </Typography>
      <div className="mb-6 flex justify-center">
        <SearchInput value={filterValue} onChange={setFilterValue} />
      </div>
      <div className="mb-6">
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
