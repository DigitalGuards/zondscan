'use client';

import { useState, useEffect, useMemo, useRef } from 'react';
import Link from 'next/link';
import { epochsToDays, formatValidatorBalance, withdrawalCredentialsToAddress } from '../../lib/helpers';
import { setUrlParams, useUrlParam, useUrlIntParam } from '../../lib/use-url-param';
import Badge from '../../components/Badge';

interface Validator {
  index: string;
  publicKeyHex: string;
  withdrawalCredentialsHex?: string;
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

// Defensive parsing for URL-sourced sort state: unknown ?sort / ?dir values
// fall back to the defaults instead of breaking the table.
function parseSortField(value: string): SortField {
  switch (value) {
    case 'index':
    case 'age':
    case 'stakedAmount':
    case 'status':
      return value;
    default:
      return 'index';
  }
}

function parseSortDirection(value: string): SortDirection {
  return value === 'desc' ? 'desc' : 'asc';
}

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
  // Table state lives in the URL (?q, ?sort, ?dir, ?page) so Back/Forward
  // and shared links restore the same view. Defaults are removed from the
  // URL to keep canonical links clean.
  const [urlQuery] = useUrlParam('q', '');
  const [sortParam] = useUrlParam('sort', 'index');
  const [dirParam] = useUrlParam('dir', 'asc');
  const [currentPage, setCurrentPage] = useUrlIntParam('page', 1);
  const sortField = parseSortField(sortParam);
  const sortDirection = parseSortDirection(dirParam);

  // Local echo of ?q so typing stays responsive while the URL write debounces.
  const [searchQuery, setSearchQuery] = useState(urlQuery);
  const searchDebounceRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const lastWrittenQueryRef = useRef(urlQuery);
  const itemsPerPage = 15;

  // Adopt external ?q changes (Back/Forward while mounted); our own
  // debounced writes are skipped via lastWrittenQueryRef. The setState is
  // deferred so it lands outside the synchronous effect body
  // (set-state-in-effect rule).
  useEffect(() => {
    if (urlQuery === lastWrittenQueryRef.current) return;
    lastWrittenQueryRef.current = urlQuery;
    if (searchDebounceRef.current) clearTimeout(searchDebounceRef.current);
    const timer = setTimeout(() => setSearchQuery(urlQuery), 0);
    return () => clearTimeout(timer);
  }, [urlQuery]);

  // Cancel any pending debounced write on unmount so it cannot fire after
  // navigation and stamp ?q onto another page's URL.
  useEffect(() => {
    return () => {
      if (searchDebounceRef.current) clearTimeout(searchDebounceRef.current);
    };
  }, []);

  const handleSearchChange = (value: string) => {
    setSearchQuery(value);
    if (searchDebounceRef.current) clearTimeout(searchDebounceRef.current);
    // Debounce the URL write so typing does not spam replaceState; the page
    // reset rides in the same call so both restore atomically. The write
    // aborts if a navigation started meanwhile: during a route transition
    // this component can stay mounted after the URL has already flipped, and
    // firing then would stamp ?q onto the destination's URL.
    const scheduledPath = window.location.pathname;
    searchDebounceRef.current = setTimeout(() => {
      if (window.location.pathname !== scheduledPath) return;
      lastWrittenQueryRef.current = value;
      setUrlParams({ q: value || null, page: null });
    }, 300);
  };

  const handleSort = (field: SortField) => {
    const nextDirection: SortDirection =
      sortField === field && sortDirection === 'asc' ? 'desc' : 'asc';
    // Single replaceState call so sort, direction, and the page reset land
    // in the URL atomically.
    setUrlParams({
      sort: field === 'index' ? null : field,
      dir: nextDirection === 'asc' ? null : nextDirection,
      page: null,
    });
  };

  // Decode withdrawal credentials once per data refresh; the decoded
  // execution address feeds both the Address column and the search filter.
  const decoratedValidators = useMemo(
    () =>
      validators.map((v) => ({
        ...v,
        withdrawalAddress: withdrawalCredentialsToAddress(v.withdrawalCredentialsHex),
      })),
    [validators]
  );

  const filteredAndSortedValidators = useMemo(() => {
    let result = [...decoratedValidators];

    // Filter: strip an optional leading Q (address prefix) so pasted
    // Q-addresses match the decoded withdrawal address hex; the query also
    // matches the raw public key hex, index, and status.
    if (searchQuery) {
      const normalized = searchQuery.trim().toLowerCase();
      const query = normalized.startsWith('q') ? normalized.slice(1) : normalized;
      result = result.filter(
        (v) =>
          v.index.toLowerCase().includes(query) ||
          v.publicKeyHex.toLowerCase().includes(query) ||
          (v.withdrawalAddress !== null && v.withdrawalAddress.slice(1).includes(query)) ||
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
  }, [decoratedValidators, searchQuery, sortField, sortDirection]);

  const totalPages = Math.ceil(filteredAndSortedValidators.length / itemsPerPage);
  // Clamp deep-linked ?page values that exceed the filtered result set.
  const safePage = Math.min(currentPage, Math.max(totalPages, 1));
  const startIndex = (safePage - 1) * itemsPerPage;
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
            onChange={(e) => handleSearchChange(e.target.value)}
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
              <th scope="col" className="hidden md:table-cell px-4 py-3 text-left text-[11px] font-medium text-text-muted uppercase tracking-[0.12em]">
                Public Key
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
                  {validator.withdrawalAddress ? (
                    <Link
                      href={`/address/${validator.withdrawalAddress}`}
                      className="text-accent hover:underline font-mono"
                    >
                      <span className="hidden md:inline">
                        {validator.withdrawalAddress.slice(0, 12)}...{validator.withdrawalAddress.slice(-8)}
                      </span>
                      <span className="md:hidden">
                        {validator.withdrawalAddress.slice(0, 8)}...
                      </span>
                    </Link>
                  ) : (
                    <span className="text-text-muted">-</span>
                  )}
                </td>
                <td className="hidden md:table-cell px-4 py-3 whitespace-nowrap text-sm text-text-secondary font-mono">
                  {validator.publicKeyHex.slice(0, 16)}...{validator.publicKeyHex.slice(-8)}
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
            onClick={() => setCurrentPage(Math.max(1, safePage - 1))}
            disabled={safePage === 1}
            className="px-2 sm:px-3 py-1 sm:py-1.5 rounded-lg bg-surface-2 text-text-secondary border border-border hover:border-accent disabled:opacity-50 disabled:hover:border-border text-xs sm:text-sm"
          >
            Prev
          </button>

          <span className="text-xs sm:text-sm text-text-secondary mx-1 sm:mx-2">
            {safePage}/{totalPages}
          </span>

          {/* Hide page numbers on very small screens */}
          <div className="hidden sm:flex gap-1">
            {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
              let pageNum;
              if (totalPages <= 5) {
                pageNum = i + 1;
              } else if (safePage <= 3) {
                pageNum = i + 1;
              } else if (safePage >= totalPages - 2) {
                pageNum = totalPages - 4 + i;
              } else {
                pageNum = safePage - 2 + i;
              }

              return (
                <button
                  key={i}
                  aria-label={`Go to page ${pageNum}`}
                  onClick={() => setCurrentPage(pageNum)}
                  className={`w-8 h-8 rounded-lg text-sm ${
                    safePage === pageNum
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
            onClick={() => setCurrentPage(Math.min(totalPages, safePage + 1))}
            disabled={safePage === totalPages}
            className="px-2 sm:px-3 py-1 sm:py-1.5 rounded-lg bg-surface-2 text-text-secondary border border-border hover:border-accent disabled:opacity-50 disabled:hover:border-border text-xs sm:text-sm"
          >
            Next
          </button>
        </div>
      )}
    </div>
  );
}
