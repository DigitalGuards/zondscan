"use client"

import * as React from 'react';
import Link from 'next/link';
import Box from '@mui/material/Box';
import Typography from '@mui/material/Typography';
import axios from 'axios';
import { decodeToHex } from '../lib/helpers';
import config from '../../config.js';

interface ContractData {
  _id: string;  // MongoDB ObjectId
  creatorAddress: string;     // Changed from contractCreatorAddress
  address: string;            // Changed from contractAddress
  name?: string;              // Changed from tokenName
  symbol?: string;            // Changed from tokenSymbol
  decimals?: number;          // Changed from tokenDecimals
  isToken: boolean;
}

interface ContractsClientProps {
  initialData: any[];
  totalContracts: number;
}

const ITEMS_PER_PAGE = 10;

const truncateMiddle = (str: string, startChars = 6, endChars = 6): string => {
  if (!str) return '';
  if (str.length <= startChars + endChars) return str;
  return `${str.slice(0, startChars)}...${str.slice(-endChars)}`;
};

const decodeField = (value: string): string => {
  if (!value) return '';
  return value;
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
  const isMobile = typeof window !== 'undefined' && window.innerWidth < 768;
  const [windowWidth, setWindowWidth] = React.useState(typeof window !== 'undefined' ? window.innerWidth : 0);

  React.useEffect(() => {
    const handleResize = () => setWindowWidth(window.innerWidth);
    window.addEventListener('resize', handleResize);
    return () => window.removeEventListener('resize', handleResize);
  }, []);

  // Generate unique keys for each row
  const rows = React.useMemo(() => 
    data.map((row, index) => ({
      ...row,
      key: row._id || `contract-${index}` // Ensure each row has a unique key
    }))
  , [data]);

  if (windowWidth < 768) {
    return (
      <div className="space-y-4">
        {rows.map((row) => (
          <div key={row.key} className="p-4 rounded-lg border border-[#3d3d3d] bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]">
            <div className="space-y-2">
              <div>
                <span className="text-[#ffa729] text-sm">Contract Address:</span>
                <Link href={`/address/${row.address}`} className="ml-2 text-white hover:text-[#ffa729] text-sm break-all">
                  {truncateMiddle(row.address)}
                </Link>
              </div>
              <div>
                <span className="text-[#ffa729] text-sm">Creator:</span>
                <Link href={`/address/${row.creatorAddress}`} className="ml-2 text-white hover:text-[#ffa729] text-sm break-all">
                  {truncateMiddle(row.creatorAddress)}
                </Link>
              </div>
              <div>
                <span className="text-[#ffa729] text-sm">Token Name:</span>
                <span className="ml-2 text-white text-sm break-all">{row.name}</span>
              </div>
              <div>
                <span className="text-[#ffa729] text-sm">Token Symbol:</span>
                <span className="ml-2 text-white text-sm break-all">{row.symbol}</span>
              </div>
              <div>
                <span className="text-[#ffa729] text-sm">Token Decimals:</span>
                <span className="ml-2 text-white text-sm break-all">{row.decimals}</span>
              </div>
              <div>
                <span className="text-[#ffa729] text-sm">Is Token:</span>
                <span className="ml-2 text-white text-sm break-all">{row.isToken ? 'Yes' : 'No'}</span>
              </div>
            </div>
          </div>
        ))}
      </div>
    );
  }

  return (
    <div className="overflow-x-auto rounded-lg border border-[#3d3d3d] bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]">
      <table className="min-w-full">
        <thead>
          <tr className="border-b border-[#3d3d3d]">
            <th className="px-3 md:px-6 py-3 md:py-4 text-left text-xs md:text-sm font-medium text-[#ffa729]">Contract Address</th>
            <th className="px-3 md:px-6 py-3 md:py-4 text-left text-xs md:text-sm font-medium text-[#ffa729]">Creator</th>
            <th className="px-3 md:px-6 py-3 md:py-4 text-left text-xs md:text-sm font-medium text-[#ffa729]">Token Name</th>
            <th className="hidden md:table-cell px-3 md:px-6 py-3 md:py-4 text-left text-xs md:text-sm font-medium text-[#ffa729]">Token Symbol</th>
            <th className="hidden md:table-cell px-3 md:px-6 py-3 md:py-4 text-left text-xs md:text-sm font-medium text-[#ffa729]">Token Decimals</th>
            <th className="hidden md:table-cell px-3 md:px-6 py-3 md:py-4 text-left text-xs md:text-sm font-medium text-[#ffa729]">Is Token</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.key} className="border-b border-[#3d3d3d] hover:bg-[rgba(255,167,41,0.05)] transition-colors">
              <td className="px-3 md:px-6 py-3 md:py-4 whitespace-nowrap">
                <Link href={`/address/${row.address}`} className="text-[#ffa729] hover:text-[#ffb954] transition-colors">
                  {truncateMiddle(row.address, 8, 8)}
                </Link>
              </td>
              <td className="px-3 md:px-6 py-3 md:py-4 whitespace-nowrap">
                <Link href={`/address/${row.creatorAddress}`} className="text-[#ffa729] hover:text-[#ffb954] transition-colors">
                  {truncateMiddle(row.creatorAddress, 8, 8)}
                </Link>
              </td>
              <td className="px-3 md:px-6 py-3 md:py-4 whitespace-nowrap text-gray-300">{row.name}</td>
              <td className="hidden md:table-cell px-3 md:px-6 py-3 md:py-4 whitespace-nowrap text-gray-300">{row.symbol}</td>
              <td className="hidden md:table-cell px-3 md:px-6 py-3 md:py-4 whitespace-nowrap text-gray-300">{row.decimals}</td>
              <td className="hidden md:table-cell px-3 md:px-6 py-3 md:py-4 whitespace-nowrap text-gray-300">{row.isToken ? 'Yes' : 'No'}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function SearchInput({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  return (
    <div className="relative mb-6">
      <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none">
        <svg className="w-4 h-4 text-gray-500" aria-hidden="true" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 20 20">
          <path stroke="currentColor" strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="m19 19-4-4m0-7A7 7 0 1 1 1 8a7 7 0 0 1 14 0Z"/>
        </svg>
      </div>
      <input
        type="search"
        value={value}
        onChange={(e) => onChange(e.target.value)}
        className="block w-full p-4 pl-10 text-sm text-white border border-[#3d3d3d] rounded-lg 
                  bg-[#2d2d2d] hover:border-[#4d4d4d] focus:ring-1 focus:ring-[#ffa729] focus:border-[#ffa729] 
                  placeholder-gray-400 outline-none transition-colors"
        placeholder="Search by contract address or token name"
      />
    </div>
  );
}

export default function ContractsClient({ initialData, totalContracts }: ContractsClientProps) {
  const [searchQuery, setSearchQuery] = React.useState('');
  const [currentPage, setCurrentPage] = React.useState(0);
  const [contracts, setContracts] = React.useState<ContractData[]>(initialData);
  const [loading, setLoading] = React.useState(false);
  const [total, setTotal] = React.useState(totalContracts);

  const fetchContracts = React.useCallback(async (page: number, search: string) => {
    try {
      setLoading(true);
      // Remove '0x' prefix if present and convert to lowercase
      const cleanSearch = search ? search.toLowerCase().replace(/^0x/, '') : undefined;
      
      const response = await axios.get(`${config.handlerUrl}/contracts`, {
        params: {
          page,
          limit: ITEMS_PER_PAGE,
          search: cleanSearch
        }
      });
      
      console.log('API response:', response.data);
      if (response.data?.response) {
        // Store the raw response data - will be properly transformed in the useMemo
        setContracts(response.data.response);
        setTotal(response.data.total || 0);
      }
    } catch (error) {
      console.error('Error fetching contracts:', error);
      setContracts([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, []);

  // Single effect to handle both search and pagination
  React.useEffect(() => {
    // Skip initial fetch if we have initialData and no search
    if (initialData.length > 0 && !searchQuery && currentPage === 0) {
      return;
    }

    const timer = setTimeout(() => {
      fetchContracts(currentPage, searchQuery);
    }, searchQuery ? 300 : 0); // Only debounce when searching

    return () => clearTimeout(timer);
  }, [searchQuery, currentPage, fetchContracts, initialData.length]);

  const transformedData = React.useMemo(() => {
    console.log('Raw contracts:', contracts);
    return contracts.map((item: any, index: number) => {
      console.log('Raw item before transform:', item);
      const transformed = {
        _id: item._id || `contract-${index}`,
        creatorAddress: item.creatorAddress || '',
        address: item.address || '',
        name: item.name || '',
        symbol: item.symbol || '',
        decimals: item.decimals,
        isToken: item.isToken || false
      };
      console.log('Transformed item:', transformed);
      return transformed;
    });
  }, [contracts]);

  const totalPages = Math.max(1, Math.ceil(total / ITEMS_PER_PAGE));

  return (
    <Box className="p-4">
      <SearchInput 
        value={searchQuery} 
        onChange={(value) => {
          setSearchQuery(value);
          setCurrentPage(0); // Reset page when searching
        }}
      />

      {loading ? (
        <div className="flex justify-center items-center py-8">
          Loading...
        </div>
      ) : transformedData.length > 0 ? (
        <CustomTable data={transformedData} />
      ) : (
        <div className="text-center py-8 text-gray-400">
          No contracts found
        </div>
      )}

      {totalPages > 1 && (
        <Pagination
          currentPage={currentPage}
          totalPages={totalPages}
          onPageChange={(page) => {
            setCurrentPage(page);
            window.scrollTo({ top: 0, behavior: 'smooth' });
          }}
        />
      )}
    </Box>
  );
}
