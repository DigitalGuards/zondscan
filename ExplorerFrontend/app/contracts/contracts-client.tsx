'use client';

import { useState, useEffect, useMemo, useCallback } from 'react';
import Link from 'next/link';
import axios from 'axios';
import config from '../../config';
import Badge from '../components/Badge';
import EmptyState from '../components/EmptyState';

interface ContractData {
  _id: string;
  creatorAddress: string;
  address: string;
  name?: string;
  symbol?: string;
  decimals?: number;
  totalSupply?: string;
  creationBlockNumber?: string;
  isToken: boolean;
  tokenStandard?: 'ERC-20' | 'ERC-721' | 'ERC-1155' | string;
}

interface ContractsClientProps {
  initialData: ContractData[];
  totalContracts: number;
}

// Per-tab → backend filter mapping. ERC-20/721/1155 use the ?standard= filter
// added in Phase 1; 'contracts' (other) uses ?isToken=false to surface non-
// token contracts. 'all' has no filter — useful for search-everything.
type TabType = 'erc20' | 'erc721' | 'erc1155' | 'contracts';

const TAB_TO_STANDARD: Record<TabType, 'ERC-20' | 'ERC-721' | 'ERC-1155' | null> = {
  erc20: 'ERC-20',
  erc721: 'ERC-721',
  erc1155: 'ERC-1155',
  contracts: null,
};

const STANDARD_LABEL: Record<string, string> = {
  'ERC-20': 'Token',
  'ERC-721': 'NFT',
  'ERC-1155': 'Multi-Token',
};

const TAB_RESULT_NOUN: Record<TabType, string> = {
  erc20: 'tokens',
  erc721: 'NFT collections',
  erc1155: 'multi-token collections',
  contracts: 'contracts',
};

const TAB_EMPTY_TITLE: Record<TabType, string> = {
  erc20: 'No tokens found',
  erc721: 'No NFT collections found',
  erc1155: 'No multi-token collections found',
  contracts: 'No contracts found',
};

const ITEMS_PER_PAGE = 15;

// Hoisted out of the render body so each ContractsClient render doesn't
// allocate a brand-new component identity. React 19 (and the new
// react-hooks/error-boundaries rule) flag in-render component creation as
// a bug — every new identity loses state and re-mounts children.
function TabButton({
  tab,
  label,
  count,
  activeTab,
  onSelect,
}: {
  tab: TabType;
  label: string;
  count?: number;
  activeTab: TabType;
  onSelect: (t: TabType) => void;
}): JSX.Element {
  const active = activeTab === tab;
  return (
    <button
      role="tab"
      aria-selected={active}
      onClick={() => onSelect(tab)}
      className={`px-4 py-2 text-sm font-medium rounded-lg transition-all duration-200 ${
        active ? 'bg-[#ffa729] text-black' : 'bg-[#2d2d2d] text-gray-300 hover:bg-[#3d3d3d]'
      }`}
    >
      {label}
      {count !== undefined && (
        <span
          className={`ml-2 px-2 py-0.5 rounded-full text-xs ${
            active ? 'bg-black/20' : 'bg-[#1f1f1f]'
          }`}
        >
          {count}
        </span>
      )}
    </button>
  );
}

// Format total supply (uses 10^decimals)
function formatTotalSupply(supply: string | undefined, decimals: number | undefined): string {
  if (!supply || supply === '0') return '0';
  try {
    const value = BigInt(supply);
    const divisor = BigInt(10 ** (decimals ?? 18));
    const formatted = Number(value) / Number(divisor);

    if (formatted >= 1_000_000_000) {
      return (formatted / 1_000_000_000).toFixed(2) + 'B';
    } else if (formatted >= 1_000_000) {
      return (formatted / 1_000_000).toFixed(2) + 'M';
    } else if (formatted >= 1_000) {
      return (formatted / 1_000).toFixed(2) + 'K';
    } else {
      return formatted.toLocaleString(undefined, { maximumFractionDigits: 2 });
    }
  } catch {
    return '0';
  }
}

// Format block number from hex
function formatBlockNumber(blockNum: string | undefined): string {
  if (!blockNum) return '-';
  try {
    if (blockNum.startsWith('0x')) {
      return parseInt(blockNum, 16).toLocaleString();
    }
    return parseInt(blockNum).toLocaleString();
  } catch {
    return '-';
  }
}

// Truncate address for display
function truncateAddress(addr: string, start = 8, end = 6): string {
  if (!addr) return '';
  if (addr.length <= start + end) return addr;
  return `${addr.slice(0, start)}...${addr.slice(-end)}`;
}

export default function ContractsClient({ initialData, totalContracts }: ContractsClientProps) {
  const [activeTab, setActiveTab] = useState<TabType>('erc20');
  const [searchQuery, setSearchQuery] = useState('');
  const [currentPage, setCurrentPage] = useState(0);
  const [contracts, setContracts] = useState<ContractData[]>(initialData);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(totalContracts);

  const fetchContracts = useCallback(async (page: number, search: string, tab: TabType) => {
    try {
      setLoading(true);
      const cleanSearch = search ? search.toLowerCase().replace(/^0x/, '') : undefined;

      const params: Record<string, any> = {
        page,
        limit: ITEMS_PER_PAGE,
      };

      if (cleanSearch) {
        params.search = cleanSearch;
      }

      const standard = TAB_TO_STANDARD[tab];
      if (standard !== null) {
        params.standard = standard;
      } else {
        // 'contracts' tab → non-token contracts only.
        params.isToken = false;
      }

      const response = await axios.get(`${config.handlerUrl}/contracts`, { params });

      if (response.data?.response) {
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

  // Fetch when tab, search, or page changes
  useEffect(() => {
    const timer = setTimeout(() => {
      fetchContracts(currentPage, searchQuery, activeTab);
    }, searchQuery ? 300 : 0);

    return () => clearTimeout(timer);
  }, [activeTab, searchQuery, currentPage, fetchContracts]);

  // Reset page when tab or search changes — adjusting state on prop/state
  // change instead of writing it in an effect (set-state-in-effect).
  const [prevReset, setPrevReset] = useState<string>(`${activeTab}|${searchQuery}`);
  const resetKey = `${activeTab}|${searchQuery}`;
  if (prevReset !== resetKey) {
    setPrevReset(resetKey);
    if (currentPage !== 0) setCurrentPage(0);
  }

  const totalPages = Math.max(1, Math.ceil(total / ITEMS_PER_PAGE));

  return (
    <div className="max-w-7xl mx-auto p-4 sm:p-6 lg:p-8">
      {/* Header */}
      <div className="mb-6">
        <h1 className="text-xl sm:text-2xl font-bold text-[#ffa729] mb-2">Smart Contracts</h1>
        <p className="text-gray-400">Browse deployed tokens and smart contracts on the QRL Zond network</p>
      </div>

      {/* Tabs */}
      <div role="tablist" className="flex flex-wrap gap-2 mb-6">
        <TabButton tab="erc20" label="ERC-20" activeTab={activeTab} onSelect={setActiveTab} />
        <TabButton tab="erc721" label="NFTs (ERC-721)" activeTab={activeTab} onSelect={setActiveTab} />
        <TabButton tab="erc1155" label="Multi-Token (ERC-1155)" activeTab={activeTab} onSelect={setActiveTab} />
        <TabButton tab="contracts" label="Other Contracts" activeTab={activeTab} onSelect={setActiveTab} />
      </div>

      {/* Search */}
      <div className="mb-6">
        <div className="relative">
          <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none">
            <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
          </div>
          <input
            type="text"
            aria-label="Search contracts"
            placeholder={activeTab === 'contracts' ? 'Search by contract address...' : 'Search by token name or address...'}
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full p-3 pl-10 bg-[#2d2d2d] border border-[#3d3d3d] rounded-lg text-gray-300 placeholder-gray-500 focus:outline-none focus:ring-2 focus:ring-[#ffa729] focus:border-transparent"
          />
        </div>
      </div>

      {/* Results Count */}
      <div className="mb-4 text-sm text-gray-400">
        {loading
          ? 'Loading...'
          : `${total} ${TAB_RESULT_NOUN[activeTab]} found`}
      </div>

      {/* Content */}
      <div role="tabpanel" className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] overflow-hidden">
        {loading ? (
          <div className="p-4 space-y-4">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="h-16 bg-gray-700/30 rounded animate-pulse" />
            ))}
          </div>
        ) : contracts.length === 0 ? (
          <EmptyState
            title={TAB_EMPTY_TITLE[activeTab]}
            description="Try adjusting your search or check back later."
          />
        ) : activeTab === 'contracts' ? (
          <ContractsTable contracts={contracts} />
        ) : activeTab === 'erc20' ? (
          <TokensTable contracts={contracts} />
        ) : (
          <NFTsTable
            contracts={contracts}
            standard={activeTab === 'erc721' ? 'ERC-721' : 'ERC-1155'}
          />
        )}
      </div>

      {/* Pagination */}
      {totalPages > 1 && !loading && (
        <div className="mt-6 flex flex-wrap justify-center items-center gap-2">
          <button
            aria-label="Go to previous page"
            onClick={() => setCurrentPage((p) => Math.max(0, p - 1))}
            disabled={currentPage === 0}
            className="px-3 py-1.5 rounded-lg bg-[#1f1f1f] text-gray-300 border border-[#3d3d3d] hover:border-[#ffa729] disabled:opacity-50 disabled:hover:border-[#3d3d3d] text-sm"
          >
            Previous
          </button>

          <span className="text-sm text-gray-400 mx-2">
            Page {currentPage + 1} of {totalPages}
          </span>

          {Array.from({ length: Math.min(5, totalPages) }, (_, i) => {
            let pageNum;
            if (totalPages <= 5) {
              pageNum = i;
            } else if (currentPage <= 2) {
              pageNum = i;
            } else if (currentPage >= totalPages - 3) {
              pageNum = totalPages - 5 + i;
            } else {
              pageNum = currentPage - 2 + i;
            }

            return (
              <button
                key={i}
                aria-label={`Go to page ${pageNum + 1}`}
                onClick={() => setCurrentPage(pageNum)}
                className={`w-8 h-8 rounded-lg text-sm ${
                  currentPage === pageNum
                    ? 'bg-[#ffa729] text-black'
                    : 'bg-[#1f1f1f] text-gray-300 hover:bg-[#3d3d3d]'
                }`}
              >
                {pageNum + 1}
              </button>
            );
          })}

          <button
            aria-label="Go to next page"
            onClick={() => setCurrentPage((p) => Math.min(totalPages - 1, p + 1))}
            disabled={currentPage >= totalPages - 1}
            className="px-3 py-1.5 rounded-lg bg-[#1f1f1f] text-gray-300 border border-[#3d3d3d] hover:border-[#ffa729] disabled:opacity-50 disabled:hover:border-[#3d3d3d] text-sm"
          >
            Next
          </button>
        </div>
      )}
    </div>
  );
}

// Tokens Table Component
function TokensTable({ contracts }: { contracts: ContractData[] }) {
  return (
    <div className="overflow-x-auto">
      <table aria-label="Token contracts" className="min-w-full divide-y divide-[#3d3d3d]">
        <thead className="bg-[#2d2d2d]/50">
          <tr>
            <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
              Token
            </th>
            <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
              Contract Address
            </th>
            <th scope="col" className="hidden md:table-cell px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
              Decimals
            </th>
            <th scope="col" className="hidden sm:table-cell px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
              Total Supply
            </th>
            <th scope="col" className="hidden lg:table-cell px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
              Creator
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-[#3d3d3d]">
          {contracts.map((contract, index) => (
            <tr key={contract._id || index} className="hover:bg-[#2d2d2d]/30">
              <td className="px-4 py-4 whitespace-nowrap">
                <div className="flex items-center gap-3">
                  <div className="w-8 h-8 rounded-full bg-gradient-to-br from-[#ffa729] to-[#ff8c00] flex items-center justify-center text-black font-bold text-sm">
                    {contract.symbol ? contract.symbol.charAt(0) : '?'}
                  </div>
                  <div>
                    <div className="text-white font-medium">{contract.name || 'Unknown Token'}</div>
                    <div className="text-gray-500 text-sm">{contract.symbol || '-'}</div>
                  </div>
                </div>
              </td>
              <td className="px-4 py-4 whitespace-nowrap">
                <Link
                  href={`/address/${contract.address}`}
                  className="text-[#ffa729] hover:underline font-mono text-sm"
                >
                  {truncateAddress(contract.address)}
                </Link>
              </td>
              <td className="hidden md:table-cell px-4 py-4 whitespace-nowrap text-gray-300 text-sm">
                {contract.decimals ?? '-'}
              </td>
              <td className="hidden sm:table-cell px-4 py-4 whitespace-nowrap text-gray-300 text-sm font-mono">
                {formatTotalSupply(contract.totalSupply, contract.decimals)}
              </td>
              <td className="hidden lg:table-cell px-4 py-4 whitespace-nowrap">
                <Link
                  href={`/address/${contract.creatorAddress}`}
                  className="text-gray-400 hover:text-[#ffa729] font-mono text-sm"
                >
                  {truncateAddress(contract.creatorAddress, 6, 4)}
                </Link>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// NFT / Multi-Token Table Component.
//
// Shares the visual identity of TokensTable but drops decimals + totalSupply
// (ERC-20-specific) and surfaces the standard badge inline so a user
// scanning the page knows whether each row is 721 or 1155.
function NFTsTable({
  contracts,
  standard,
}: {
  contracts: ContractData[];
  standard: 'ERC-721' | 'ERC-1155';
}) {
  const label = STANDARD_LABEL[standard] ?? standard;
  return (
    <div className="overflow-x-auto">
      <table aria-label={`${label} collections`} className="min-w-full divide-y divide-[#3d3d3d]">
        <thead className="bg-[#2d2d2d]/50">
          <tr>
            <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
              Collection
            </th>
            <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
              Contract Address
            </th>
            <th scope="col" className="hidden sm:table-cell px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
              Standard
            </th>
            <th scope="col" className="hidden lg:table-cell px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
              Creator
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-[#3d3d3d]">
          {contracts.map((contract, index) => (
            <tr key={contract._id || index} className="hover:bg-[#2d2d2d]/30">
              <td className="px-4 py-4 whitespace-nowrap">
                <div className="flex items-center gap-3">
                  <div className="w-8 h-8 rounded-full bg-gradient-to-br from-[#ffa729] to-[#ff8c00] flex items-center justify-center text-black font-bold text-sm">
                    {contract.symbol ? contract.symbol.charAt(0) : '?'}
                  </div>
                  <div>
                    <div className="text-white font-medium">{contract.name || 'Unknown Collection'}</div>
                    <div className="text-gray-500 text-sm">{contract.symbol || '-'}</div>
                  </div>
                </div>
              </td>
              <td className="px-4 py-4 whitespace-nowrap">
                <Link
                  href={`/address/${contract.address}`}
                  className="text-[#ffa729] hover:underline font-mono text-sm"
                >
                  {truncateAddress(contract.address)}
                </Link>
              </td>
              <td className="hidden sm:table-cell px-4 py-4 whitespace-nowrap">
                <Badge variant="warning">{standard === 'ERC-721' ? 'QRC-721' : 'QRC-1155'}</Badge>
              </td>
              <td className="hidden lg:table-cell px-4 py-4 whitespace-nowrap">
                <Link
                  href={`/address/${contract.creatorAddress}`}
                  className="text-gray-400 hover:text-[#ffa729] font-mono text-sm"
                >
                  {truncateAddress(contract.creatorAddress, 6, 4)}
                </Link>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

// All Contracts Table Component
function ContractsTable({ contracts }: { contracts: ContractData[] }) {
  return (
    <div className="overflow-x-auto">
      <table aria-label="Smart contracts" className="min-w-full divide-y divide-[#3d3d3d]">
        <thead className="bg-[#2d2d2d]/50">
          <tr>
            <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
              Contract Address
            </th>
            <th scope="col" className="px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
              Type
            </th>
            <th scope="col" className="hidden sm:table-cell px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
              Creator
            </th>
            <th scope="col" className="hidden md:table-cell px-4 py-3 text-left text-xs font-medium text-gray-400 uppercase tracking-wider">
              Created at Block
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-[#3d3d3d]">
          {contracts.map((contract, index) => (
            <tr key={contract._id || index} className="hover:bg-[#2d2d2d]/30">
              <td className="px-4 py-4 whitespace-nowrap">
                <Link
                  href={`/address/${contract.address}`}
                  className="text-[#ffa729] hover:underline font-mono text-sm"
                >
                  <span className="hidden sm:inline">{truncateAddress(contract.address, 10, 8)}</span>
                  <span className="sm:hidden">{truncateAddress(contract.address, 6, 4)}</span>
                </Link>
              </td>
              <td className="px-4 py-4 whitespace-nowrap">
                {contract.tokenStandard === 'ERC-721' ? (
                  <Badge variant="warning">NFT (QRC-721){contract.symbol ? ` · ${contract.symbol}` : ''}</Badge>
                ) : contract.tokenStandard === 'ERC-1155' ? (
                  <Badge variant="warning">Multi-Token (QRC-1155){contract.symbol ? ` · ${contract.symbol}` : ''}</Badge>
                ) : contract.isToken ? (
                  <Badge variant="success">Token (QRC-20){contract.symbol ? ` · ${contract.symbol}` : ''}</Badge>
                ) : (
                  <Badge variant="info">Contract</Badge>
                )}
              </td>
              <td className="hidden sm:table-cell px-4 py-4 whitespace-nowrap">
                <Link
                  href={`/address/${contract.creatorAddress}`}
                  className="text-gray-400 hover:text-[#ffa729] font-mono text-sm"
                >
                  {truncateAddress(contract.creatorAddress, 6, 4)}
                </Link>
              </td>
              <td className="hidden md:table-cell px-4 py-4 whitespace-nowrap text-gray-300 text-sm font-mono">
                {contract.creationBlockNumber ? (
                  <Link
                    href={`/block/${formatBlockNumber(contract.creationBlockNumber).replace(/,/g, '')}`}
                    className="text-gray-400 hover:text-[#ffa729]"
                  >
                    #{formatBlockNumber(contract.creationBlockNumber)}
                  </Link>
                ) : (
                  '-'
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
