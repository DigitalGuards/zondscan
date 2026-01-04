'use client';

import { useState, useMemo } from 'react';
import Link from 'next/link';
import { epochsToDays } from '../../lib/helpers';

// Format staked amount (uses 10^12 decimals for QRL validators)
function formatValidatorStake(amount: string): [string, string] {
  if (!amount || amount === '0') return ['0', 'QRL'];
  try {
    const value = BigInt(amount);
    const divisor = BigInt('1000000000000'); // 10^12 (Shor)
    const qrlValue = Number(value) / Number(divisor);
    return [qrlValue.toLocaleString(undefined, { maximumFractionDigits: 0 }), 'QRL'];
  } catch {
    return ['0', 'QRL'];
  }
}

interface Validator {
  index: string;
  address: string;
  status: string;
  age: number;
  stakedAmount: string;
  isActive: boolean;
}

interface ValidatorTableProps {
  validators: Validator[];
  loading: boolean;
}

type SortField = 'index' | 'age' | 'stakedAmount' | 'status';
type SortDirection = 'asc' | 'desc';

const statusOrder: Record<string, number> = {
  active: 0,
  pending: 1,
  exited: 2,
  slashed: 3,
};

export default function ValidatorTable({ validators, loading }: ValidatorTableProps) {
  const [searchQuery, setSearchQuery] = useState('');
  const [sortField, setSortField] = useState<SortField>('index');
  const [sortDirection, setSortDirection] = useState<SortDirection>('asc');
  const [currentPage, setCurrentPage] = useState(1);
  const itemsPerPage = 15;

  const handleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDirection(sortDirection === 'asc' ? 'desc' : 'asc');
    } else {
      setSortField(field);
      setSortDirection('asc');
    }
  };

  const filteredAndSortedValidators = useMemo(() => {
    let result = [...validators];

    // Filter
    if (searchQuery) {
      const query = searchQuery.toLowerCase();
      result = result.filter(
        (v) =>
          v.index.toLowerCase().includes(query) ||
          v.address.toLowerCase().includes(query) ||
          v.status.toLowerCase().includes(query)
      );
    }

    // Sort
    result.sort((a, b) => {
      let comparison = 0;

      switch (sortField) {
        case 'index':
          comparison = parseInt(a.index) - parseInt(b.index);
          break;
        case 'age':
          comparison = a.age - b.age;
          break;
        case 'stakedAmount':
          comparison = BigInt(a.stakedAmount) > BigInt(b.stakedAmount) ? 1 : -1;
          break;
        case 'status':
          comparison = statusOrder[a.status] - statusOrder[b.status];
          break;
      }

      return sortDirection === 'asc' ? comparison : -comparison;
    });

    return result;
  }, [validators, searchQuery, sortField, sortDirection]);

  const totalPages = Math.ceil(filteredAndSortedValidators.length / itemsPerPage);
  const startIndex = (currentPage - 1) * itemsPerPage;
  const currentValidators = filteredAndSortedValidators.slice(
    startIndex,
    startIndex + itemsPerPage
  );

  const getStatusBadge = (status: string) => {
    const styles: Record<string, string> = {
      active: 'bg-green-900/30 text-green-400 border-green-800',
      pending: 'bg-yellow-900/30 text-yellow-400 border-yellow-800',
      exited: 'bg-gray-900/30 text-gray-400 border-gray-700',
      slashed: 'bg-red-900/30 text-red-400 border-red-800',
    };

    return (
      <span
        className={`inline-flex items-center px-2.5 py-0.5 rounded-full text-xs font-medium border ${
          styles[status] || styles.pending
        }`}
      >
        {status.charAt(0).toUpperCase() + status.slice(1)}
      </span>
    );
  };

  const SortIcon = ({ field }: { field: SortField }) => {
    if (sortField !== field) {
      return (
        <span className="text-gray-600 ml-1">&#8597;</span>
      );
    }
    return (
      <span className="text-[#ffa729] ml-1">
        {sortDirection === 'asc' ? '&#8593;' : '&#8595;'}
      </span>
    );
  };

  if (loading) {
    return (
      <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] overflow-hidden">
        <div className="p-4 space-y-4">
          {[...Array(10)].map((_, i) => (
            <div key={i} className="h-12 bg-gray-700/30 rounded animate-pulse" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] overflow-hidden">
      {/* Search and Controls */}
      <div className="p-4 border-b border-[#3d3d3d]">
        <div className="flex flex-col sm:flex-row gap-4">
          <input
            type="text"
            placeholder="Search by index, address, or status..."
            value={searchQuery}
            onChange={(e) => {
              setSearchQuery(e.target.value);
              setCurrentPage(1);
            }}
            className="flex-1 p-2 bg-[#1f1f1f] border border-[#3d3d3d] rounded-lg text-gray-300 placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-[#ffa729] focus:border-transparent"
          />
          <div className="text-sm text-gray-400 flex items-center">
            {filteredAndSortedValidators.length} validators
          </div>
        </div>
      </div>

      {/* Table */}
      <div className="overflow-x-auto">
        <table className="min-w-full divide-y divide-[#3d3d3d]">
          <thead className="bg-[#2d2d2d]/50">
            <tr>
              <th
                className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-200"
                onClick={() => handleSort('index')}
              >
                Index <SortIcon field="index" />
              </th>
              <th className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
                Address
              </th>
              <th
                className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-200"
                onClick={() => handleSort('status')}
              >
                Status <SortIcon field="status" />
              </th>
              <th
                className="hidden sm:table-cell px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-200"
                onClick={() => handleSort('age')}
              >
                Age <SortIcon field="age" />
              </th>
              <th
                className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider cursor-pointer hover:text-gray-200"
                onClick={() => handleSort('stakedAmount')}
              >
                Stake <SortIcon field="stakedAmount" />
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-[#3d3d3d]">
            {currentValidators.map((validator) => (
              <tr
                key={validator.index}
                className="hover:bg-[#2d2d2d]/30 cursor-pointer"
              >
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <Link
                    href={`/validators/${validator.index}`}
                    className="text-[#ffa729] hover:underline font-mono"
                  >
                    #{validator.index}
                  </Link>
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <Link
                    href={`/validators/${validator.index}`}
                    className="text-gray-300 hover:text-[#ffa729] font-mono"
                  >
                    <span className="hidden md:inline">
                      Z{validator.address.slice(0, 16)}...{validator.address.slice(-8)}
                    </span>
                    <span className="md:hidden">
                      Z{validator.address.slice(0, 8)}...
                    </span>
                  </Link>
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  {getStatusBadge(validator.status)}
                </td>
                <td className="hidden sm:table-cell px-4 py-3 whitespace-nowrap text-sm text-gray-300">
                  {epochsToDays(validator.age).toFixed(1)} days
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm text-gray-300 font-mono">
                  {formatValidatorStake(validator.stakedAmount)[0]} {formatValidatorStake(validator.stakedAmount)[1]}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="p-4 border-t border-[#3d3d3d] flex flex-wrap justify-center items-center gap-2">
          <button
            onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
            disabled={currentPage === 1}
            className="px-3 py-1.5 rounded-lg bg-[#1f1f1f] text-gray-300 border border-[#3d3d3d] hover:border-[#ffa729] disabled:opacity-50 disabled:hover:border-[#3d3d3d] text-sm"
          >
            Previous
          </button>

          <span className="text-sm text-gray-400 mx-2">
            Page {currentPage} of {totalPages}
          </span>

          {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
            let pageNum;
            if (totalPages <= 5) {
              pageNum = i + 1;
            } else if (currentPage <= 3) {
              pageNum = i + 1;
            } else if (currentPage >= totalPages - 2) {
              pageNum = totalPages - 4 + i;
            } else {
              pageNum = currentPage - 2 + i;
            }

            return (
              <button
                key={i}
                onClick={() => setCurrentPage(pageNum)}
                className={`w-8 h-8 rounded-lg text-sm ${
                  currentPage === pageNum
                    ? 'bg-[#ffa729] text-black'
                    : 'bg-[#1f1f1f] text-gray-300 hover:bg-[#3d3d3d]'
                }`}
              >
                {pageNum}
              </button>
            );
          })}

          <button
            onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
            disabled={currentPage === totalPages}
            className="px-3 py-1.5 rounded-lg bg-[#1f1f1f] text-gray-300 border border-[#3d3d3d] hover:border-[#ffa729] disabled:opacity-50 disabled:hover:border-[#3d3d3d] text-sm"
          >
            Next
          </button>
        </div>
      )}
    </div>
  );
}
