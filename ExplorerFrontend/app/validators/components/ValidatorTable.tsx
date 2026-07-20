'use client';

import { useState, useMemo } from 'react';
import Link from 'next/link';
import { epochsToDays, formatValidatorBalance } from '../../lib/helpers';
import Badge from '../../components/Badge';

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

function SortIcon({ field, sortField, sortDirection }: { field: SortField; sortField: SortField; sortDirection: SortDirection }) {
  if (sortField !== field) {
    return <span className="text-text-muted ml-1">↕</span>;
  }
  return (
    <span className="text-accent ml-1">
      {sortDirection === 'asc' ? '↑' : '↓'}
    </span>
  );
}

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
          const aAmount = BigInt(a.stakedAmount);
          const bAmount = BigInt(b.stakedAmount);
          if (aAmount > bAmount) {
            comparison = 1;
          } else if (aAmount < bAmount) {
            comparison = -1;
          }
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
    const variantMap: Record<string, 'success' | 'warning' | 'neutral' | 'error'> = {
      active: 'success',
      pending: 'warning',
      exited: 'neutral',
      slashed: 'error',
    };
    return (
      <Badge variant={variantMap[status] || 'warning'} dot>
        {status.charAt(0).toUpperCase() + status.slice(1)}
      </Badge>
    );
  };

  if (loading) {
    return (
      <div className="card overflow-hidden">
        <div className="p-3 sm:p-4 space-y-3 sm:space-y-4">
          {[...Array(10)].map((_, i) => (
            <div key={i} className="h-10 sm:h-12 skeleton" />
          ))}
        </div>
      </div>
    );
  }

  return (
    <div className="card overflow-hidden">
      {/* Search and Controls */}
      <div className="p-3 sm:p-4 border-b border-border">
        <div className="flex flex-col sm:flex-row gap-2 sm:gap-4">
          <input
            type="text"
            aria-label="Search validators"
            placeholder="Search validators..."
            value={searchQuery}
            onChange={(e) => {
              setSearchQuery(e.target.value);
              setCurrentPage(1);
            }}
            className="flex-1 p-2 text-sm sm:text-base bg-surface-2 border border-border rounded-lg text-text-secondary placeholder-text-muted focus:outline-none focus:ring-2 focus:ring-accent focus:border-transparent"
          />
          <div className="text-xs sm:text-sm text-text-secondary flex items-center">
            {filteredAndSortedValidators.length} validators
          </div>
        </div>
      </div>

      {/* Table */}
      <div className="overflow-x-auto">
        <table aria-label="Validators list" className="min-w-full divide-y divide-border">
          <thead className="border-b border-border">
            <tr>
              <th
                scope="col"
                aria-sort={sortField === 'index' ? (sortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}
                className="px-4 py-3 text-left text-[11px] font-medium text-text-muted uppercase tracking-[0.12em]"
              >
                <button
                  onClick={() => handleSort('index')}
                  className="flex items-center hover:text-text-primary focus:outline-none focus:underline"
                >
                  Index <SortIcon field="index" sortField={sortField} sortDirection={sortDirection} />
                </button>
              </th>
              <th scope="col" className="px-4 py-3 text-left text-[11px] font-medium text-text-muted uppercase tracking-[0.12em]">
                Address
              </th>
              <th
                scope="col"
                aria-sort={sortField === 'status' ? (sortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}
                className="px-4 py-3 text-left text-[11px] font-medium text-text-muted uppercase tracking-[0.12em]"
              >
                <button
                  onClick={() => handleSort('status')}
                  className="flex items-center hover:text-text-primary focus:outline-none focus:underline"
                >
                  Status <SortIcon field="status" sortField={sortField} sortDirection={sortDirection} />
                </button>
              </th>
              <th
                scope="col"
                aria-sort={sortField === 'age' ? (sortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}
                className="hidden sm:table-cell px-4 py-3 text-left text-[11px] font-medium text-text-muted uppercase tracking-[0.12em]"
              >
                <button
                  onClick={() => handleSort('age')}
                  className="flex items-center hover:text-text-primary focus:outline-none focus:underline"
                >
                  Age <SortIcon field="age" sortField={sortField} sortDirection={sortDirection} />
                </button>
              </th>
              <th
                scope="col"
                aria-sort={sortField === 'stakedAmount' ? (sortDirection === 'asc' ? 'ascending' : 'descending') : 'none'}
                className="px-4 py-3 text-left text-[11px] font-medium text-text-muted uppercase tracking-[0.12em]"
              >
                <button
                  onClick={() => handleSort('stakedAmount')}
                  className="flex items-center hover:text-text-primary focus:outline-none focus:underline"
                >
                  Stake <SortIcon field="stakedAmount" sortField={sortField} sortDirection={sortDirection} />
                </button>
              </th>
            </tr>
          </thead>
          <tbody className="divide-y divide-border">
            {currentValidators.map((validator) => {
              const [stakeValue, stakeUnit] = formatValidatorBalance(validator.stakedAmount);
              return (
              <tr
                key={validator.index}
                className="hover:bg-surface-2/30 cursor-pointer"
              >
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <Link
                    href={`/validators/${validator.index}`}
                    className="text-accent hover:underline font-mono"
                  >
                    #{validator.index}
                  </Link>
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  <Link
                    href={`/validators/${validator.index}`}
                    className="text-text-secondary hover:text-accent font-mono"
                  >
                    <span className="hidden md:inline">
                      Q{validator.address.slice(0, 16)}...{validator.address.slice(-8)}
                    </span>
                    <span className="md:hidden">
                      Q{validator.address.slice(0, 8)}...
                    </span>
                  </Link>
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm">
                  {getStatusBadge(validator.status)}
                </td>
                <td className="hidden sm:table-cell px-4 py-3 whitespace-nowrap text-sm text-text-secondary">
                  {epochsToDays(validator.age).toFixed(1)} days
                </td>
                <td className="px-4 py-3 whitespace-nowrap text-sm text-text-secondary font-mono">
                  {stakeValue} {stakeUnit}
                </td>
              </tr>
              );
            })}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="p-3 sm:p-4 border-t border-border flex flex-wrap justify-center items-center gap-1 sm:gap-2">
          <button
            aria-label="Go to previous page"
            onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
            disabled={currentPage === 1}
            className="px-2 sm:px-3 py-1 sm:py-1.5 rounded-lg bg-surface-2 text-text-secondary border border-border hover:border-accent disabled:opacity-50 disabled:hover:border-border text-xs sm:text-sm"
          >
            Prev
          </button>

          <span className="text-xs sm:text-sm text-text-secondary mx-1 sm:mx-2">
            {currentPage}/{totalPages}
          </span>

          {/* Hide page numbers on very small screens */}
          <div className="hidden sm:flex gap-1">
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
                  aria-label={`Go to page ${pageNum}`}
                  onClick={() => setCurrentPage(pageNum)}
                  className={`w-8 h-8 rounded-lg text-sm ${
                    currentPage === pageNum
                      ? 'bg-accent text-background font-semibold'
                      : 'bg-surface-2 text-text-secondary hover:bg-surface-3'
                  }`}
                >
                  {pageNum}
                </button>
              );
            })}
          </div>

          <button
            aria-label="Go to next page"
            onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
            disabled={currentPage === totalPages}
            className="px-2 sm:px-3 py-1 sm:py-1.5 rounded-lg bg-surface-2 text-text-secondary border border-border hover:border-accent disabled:opacity-50 disabled:hover:border-border text-xs sm:text-sm"
          >
            Next
          </button>
        </div>
      )}
    </div>
  );
}
