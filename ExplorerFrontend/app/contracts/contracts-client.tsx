'use client';

import { useState, useEffect, useCallback } from 'react';
import ImageWithFallback from '../components/ImageWithFallback';
import Link from 'next/link';
import { useRouter, useSearchParams } from 'next/navigation';
import axios from 'axios';
import config from '../../config';
import Badge from '../components/Badge';
import EmptyState from '../components/EmptyState';

const TAB_TYPES = ['erc20', 'erc721', 'erc1155', 'contracts'] as const;
function parseTab(raw: string | null): TabType {
  return (TAB_TYPES as readonly string[]).includes(raw ?? '')
    ? (raw as TabType)
    : 'erc20';
}

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
  // Phase 3a: off-chain collection metadata. metadataName preferred over
  // on-chain name() when both exist, NFT collections typically leave
  // name() empty and put the display title in the contractURI JSON.
  metadataName?: string;
  metadataImage?: string;
}

interface ContractsClientProps {
  initialData: ContractData[];
  totalContracts: number;
}

// Per-tab → backend filter mapping. ERC-20/721/1155 use the ?standard= filter
// added in Phase 1; 'contracts' (other) uses ?isToken=false to surface non-
// token contracts. 'all' has no filter, useful for search-everything.
type TabType = 'erc20' | 'erc721' | 'erc1155' | 'contracts';

const TAB_TO_STANDARD: Record<TabType, 'ERC-20' | 'ERC-721' | 'ERC-1155' | null> = {
  erc20: 'ERC-20',
  erc721: 'ERC-721',
  erc1155: 'ERC-1155',
  contracts: null,
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
// a bug, every new identity loses state and re-mounts children.
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
      id={`contracts-tab-${tab}`}
      aria-selected={active}
      aria-controls="contracts-tabpanel"
      onClick={() => onSelect(tab)}
      className={`px-4 py-2 text-sm font-medium rounded-lg transition-all duration-200 ${
        active ? 'bg-accent text-background border border-accent font-semibold' : 'bg-surface-2 text-text-secondary border border-border hover:bg-surface-3 hover:text-text-primary'
      }`}
    >
      {label}
      {count !== undefined && (
        <span
          className={`ml-2 px-2 py-0.5 rounded-full text-xs ${
            active ? 'bg-background/25' : 'bg-surface-3'
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
  // URL-backed active tab so deep-links like /contracts?tab=erc1155 land on
  // the right pane and breadcrumb back-steps from contract detail pages
  // return the user to the section they came from. URL is the source of
  // truth; clicking a tab pushes a replace() so the back button stays
  // useful for in-page navigation.
  const router = useRouter();
  const searchParams = useSearchParams();
  const urlTab = parseTab(searchParams.get('tab'));
  const [activeTab, setActiveTabState] = useState<TabType>(urlTab);
  const setActiveTab = useCallback((t: TabType) => {
    setActiveTabState(t);
    const params = new URLSearchParams(searchParams.toString());
    params.set('tab', t);
    router.replace(`/contracts?${params.toString()}`, { scroll: false });
  }, [router, searchParams]);
  // Keep state in sync when the URL changes from outside (back/forward,
  // breadcrumb click on a different tab). Adjusting state during render
  // via a prev-value tracker, the React-recommended replacement for the
  // set-state-in-effect anti-pattern. Matches the cadence used in
  // pending-transaction-view.tsx for ETA resync.
  const [prevUrlTab, setPrevUrlTab] = useState<TabType>(urlTab);
  if (urlTab !== prevUrlTab) {
    setPrevUrlTab(urlTab);
    setActiveTabState(urlTab);
  }
  const [searchQuery, setSearchQuery] = useState('');
  const [currentPage, setCurrentPage] = useState(0);
  const [contracts, setContracts] = useState<ContractData[]>(initialData);
  const [loading, setLoading] = useState(false);
  const [total, setTotal] = useState(totalContracts);
  // Per-tab totals so the TabButtons can surface counts; loaded once on
  // mount via four small `total`-only requests (limit=1 keeps the response
  // payload trivial). Undefined while pending so the count chip renders
  // only when we have real data.
  const [tabCounts, setTabCounts] = useState<Partial<Record<TabType, number>>>({});

  // Single round trip via /contracts/counts (iter 24); the backend
  // aggregation returns all four buckets at once, replacing the
  // four mount-time /contracts limit=1 hits the previous version made.
  // 30s server-side cache absorbs concurrent traffic on the contracts
  // page so DB cost stays predictable.
  useEffect(() => {
    let cancelled = false;
    axios.get(`${config.handlerUrl}/contracts/counts`)
      .then((r) => {
        if (cancelled) return;
        const d = r.data;
        if (!d || typeof d !== 'object') return;
        setTabCounts({
          erc20: typeof d.erc20 === 'number' ? d.erc20 : undefined,
          erc721: typeof d.erc721 === 'number' ? d.erc721 : undefined,
          erc1155: typeof d.erc1155 === 'number' ? d.erc1155 : undefined,
          contracts: typeof d.other === 'number' ? d.other : undefined,
        });
      })
      .catch(() => {
        // Endpoint failure is silent: tab labels just render without
        // count chips. Less noise than retrying or surfacing a banner.
      });
    return () => {
      cancelled = true;
    };
  }, []);

  const fetchContracts = useCallback(async (page: number, search: string, tab: TabType) => {
    try {
      setLoading(true);
      const cleanSearch = search ? search.toLowerCase().replace(/^0x/, '') : undefined;

      // Axios stringifies primitives directly; the union covers everything
      // we actually assign here (page/limit as number, search/standard as
      // string, isToken as boolean). Narrowing it to `unknown` would lose
      // that, narrowing it to a closed union is the right shape.
      const params: Record<string, string | number | boolean> = {
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

  // Reset page when tab or search changes, adjusting state on prop/state
  // change instead of writing it in an effect (set-state-in-effect).
  const [prevReset, setPrevReset] = useState<string>(`${activeTab}|${searchQuery}`);
  const resetKey = `${activeTab}|${searchQuery}`;
  if (prevReset !== resetKey) {
    setPrevReset(resetKey);
    if (currentPage !== 0) setCurrentPage(0);
  }

  const totalPages = Math.max(1, Math.ceil(total / ITEMS_PER_PAGE));

  return (
    <div className="py-4 sm:py-6 lg:py-8">
      {/* Header */}
      <div className="mb-6">
        <h1 className="section-title mb-2">Smart Contracts</h1>
        <p className="text-text-secondary">Browse deployed tokens and smart contracts on the QRL 2.0 network</p>
      </div>

      {/* Tabs, QRC-X is the QRL-branded form of the EIP standards; the
          underlying tokenStandard string stays "ERC-X" in the DB / API. */}
      <div role="tablist" className="flex flex-wrap gap-2 mb-6">
        <TabButton tab="erc20" label="Tokens (QRC-20)" count={tabCounts.erc20} activeTab={activeTab} onSelect={setActiveTab} />
        <TabButton tab="erc721" label="NFTs (QRC-721)" count={tabCounts.erc721} activeTab={activeTab} onSelect={setActiveTab} />
        <TabButton tab="erc1155" label="Multi-Token (QRC-1155)" count={tabCounts.erc1155} activeTab={activeTab} onSelect={setActiveTab} />
        <TabButton tab="contracts" label="Other Contracts" count={tabCounts.contracts} activeTab={activeTab} onSelect={setActiveTab} />
      </div>

      {/* Search */}
      <div className="mb-6">
        <div className="relative">
          <div className="absolute inset-y-0 left-0 flex items-center pl-3 pointer-events-none">
            <svg className="w-4 h-4 text-text-muted" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z" />
            </svg>
          </div>
          <input
            type="text"
            aria-label="Search contracts"
            placeholder={activeTab === 'contracts' ? 'Search by contract address...' : 'Search by token name or address...'}
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
            className="w-full p-3 pl-10 bg-surface-2 border border-border rounded-lg text-text-secondary placeholder-text-muted focus:outline-none focus:ring-2 focus:ring-accent focus:border-transparent"
          />
        </div>
      </div>

      {/* Results Count */}
      <div className="mb-4 text-sm text-text-secondary">
        {loading
          ? 'Loading...'
          : `${total} ${TAB_RESULT_NOUN[activeTab]} found`}
      </div>

      {/* Content */}
      <div
        role="tabpanel"
        id="contracts-tabpanel"
        aria-labelledby={`contracts-tab-${activeTab}`}
        className="card overflow-hidden"
      >
        {loading ? (
          <div className="p-4 space-y-4">
            {[...Array(5)].map((_, i) => (
              <div key={i} className="h-16 skeleton" />
            ))}
          </div>
        ) : contracts.length === 0 ? (
          <EmptyState
            title={TAB_EMPTY_TITLE[activeTab]}
            description="Try adjusting your search or check back later."
          />
        ) : (
          <ContractRowsTable contracts={contracts} variant={activeTab} />
        )}
      </div>

      {/* Pagination */}
      {totalPages > 1 && !loading && (
        <div className="mt-6 flex flex-wrap justify-center items-center gap-2">
          <button
            aria-label="Go to previous page"
            onClick={() => setCurrentPage((p) => Math.max(0, p - 1))}
            disabled={currentPage === 0}
            className="px-3 py-1.5 rounded-lg bg-surface-2 text-text-secondary border border-border hover:border-accent disabled:opacity-50 disabled:hover:border-border text-sm"
          >
            Previous
          </button>

          <span className="text-sm text-text-secondary mx-2">
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
                    ? 'bg-accent text-background font-semibold'
                    : 'bg-surface-2 text-text-secondary hover:bg-surface-3'
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
            className="px-3 py-1.5 rounded-lg bg-surface-2 text-text-secondary border border-border hover:border-accent disabled:opacity-50 disabled:hover:border-border text-sm"
          >
            Next
          </button>
        </div>
      )}
    </div>
  );
}

// Single source of truth for cell chrome so every tab looks like the same
// table with a different column subset, the original split-into-three
// approach gave each tab its own visual identity, which read as four
// different pages stitched together.
const TH_BASE = 'px-4 py-3 text-left text-[11px] font-medium text-text-muted uppercase tracking-[0.12em]';
const TD_BASE = 'px-4 py-4 whitespace-nowrap';

const TAB_TABLE_LABEL: Record<TabType, string> = {
  erc20: 'Tokens',
  erc721: 'NFT collections',
  erc1155: 'Multi-token collections',
  contracts: 'Smart contracts',
};

// ContractRowsTable renders all four tabs through the same chrome, same
// header bar, same divider, same row hover, same identity cell. Per-tab
// columns toggle on/off via the `variant` prop, but the visual rhythm
// stays identical across tabs.
function ContractRowsTable({
  contracts,
  variant,
}: {
  contracts: ContractData[];
  variant: TabType;
}) {
  const showDecimalsAndSupply = variant === 'erc20';
  return (
    <div className="overflow-x-auto">
      <table aria-label={TAB_TABLE_LABEL[variant]} className="min-w-full divide-y divide-border">
        <thead className="border-b border-border">
          <tr>
            <th scope="col" className={TH_BASE}>
              {variant === 'contracts' ? 'Contract' : variant === 'erc20' ? 'Token' : 'Collection'}
            </th>
            <th scope="col" className={TH_BASE}>Contract Address</th>
            <th scope="col" className={`hidden sm:table-cell ${TH_BASE}`}>Type</th>
            {showDecimalsAndSupply && (
              <th scope="col" className={`hidden md:table-cell ${TH_BASE}`}>Decimals</th>
            )}
            {showDecimalsAndSupply && (
              <th scope="col" className={`hidden lg:table-cell ${TH_BASE}`}>Total Supply</th>
            )}
            <th scope="col" className={`hidden lg:table-cell ${TH_BASE}`}>Creator</th>
            <th scope="col" className={`hidden xl:table-cell ${TH_BASE}`}>Created at Block</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {contracts.map((contract, i) => (
            <ContractRow
              key={contract._id || i}
              contract={contract}
              variant={variant}
              showDecimalsAndSupply={showDecimalsAndSupply}
            />
          ))}
        </tbody>
      </table>
    </div>
  );
}

function ContractRow({
  contract,
  variant,
  showDecimalsAndSupply,
}: {
  contract: ContractData;
  variant: TabType;
  showDecimalsAndSupply: boolean;
}) {
  const isToken = variant !== 'contracts';
  // Identity cell: avatar + name/symbol stack for tokens; avatar + "Smart
  // Contract" + truncated address for non-token contracts. Same physical
  // layout in both cases, only the colour palette + text vary.
  const avatarSeed = isToken
    ? (contract.symbol || contract.name || '?')
    : contract.address.replace(/^Q/i, '');
  const avatarChar = (avatarSeed.charAt(0) || '?').toUpperCase();
  const avatarGradient = isToken
    ? 'from-accent to-accent-dark'
    : 'from-surface-3 to-surface-2';
  const avatarTextColor = isToken ? 'text-background' : 'text-text-primary';

  // Some standards (ERC-1155 in particular) don't expose name()/symbol(),
  // so the syncer stores empty strings. Phase 3a adds an off-chain name
  // from contractURI(); prefer it over the on-chain name when present.
  // Fall through to a truncated address + standard-label so the row
  // remains identifiable instead of a generic "Unknown".
  const standardFallback =
    variant === 'erc20' ? 'Token'
    : variant === 'erc721' ? 'NFT Collection'
    : variant === 'erc1155' ? 'Multi-Token Collection'
    : 'Contract';
  const displayName = (contract.metadataName?.trim() || contract.name || '').trim();
  const primary = isToken
    ? (displayName || truncateAddress(contract.address, 10, 8))
    : 'Smart Contract';
  const secondary = isToken
    ? (contract.symbol || standardFallback)
    : truncateAddress(contract.address, 6, 4);

  const typeBadge = (() => {
    if (variant === 'erc20') return <Badge variant="success">QRC-20</Badge>;
    if (variant === 'erc721') return <Badge variant="warning">QRC-721 NFT</Badge>;
    if (variant === 'erc1155') return <Badge variant="warning">QRC-1155</Badge>;
    return <Badge variant="info">Contract</Badge>;
  })();

  // Phase 3a: render the off-chain collection image in the avatar slot
  // when present. Falls back to the monogram gradient when missing/empty.
  // Fixed 32px square so the row height doesn't shift while next/image
  // resolves; `unoptimized` because the wallet backend's IPFS proxy
  // already streams pre-sized originals (no /_next/image rewrite path).
  const metaImage = (contract.metadataImage || '').trim();

  return (
    <tr className="hover:bg-surface-2/30">
      <td className={TD_BASE}>
        <div className="flex items-center gap-3">
          {metaImage ? (
            <div className="relative w-8 h-8 rounded-full overflow-hidden border border-border bg-black/30 shrink-0">
              <ImageWithFallback
                src={metaImage}
                alt={primary}
                fill
                sizes="32px"
                className="object-cover"
                fallback={
                  <div
                    className={`absolute inset-0 bg-gradient-to-br ${avatarGradient} flex items-center justify-center font-bold text-sm ${avatarTextColor}`}
                  >
                    {avatarChar}
                  </div>
                }
              />
            </div>
          ) : (
            <div
              className={`w-8 h-8 rounded-full bg-gradient-to-br ${avatarGradient} flex items-center justify-center font-bold text-sm shrink-0 ${avatarTextColor}`}
            >
              {avatarChar}
            </div>
          )}
          <div className="min-w-0">
            <div className="text-text-primary font-medium truncate">{primary}</div>
            <div className="text-text-muted text-sm truncate">{secondary}</div>
          </div>
        </div>
      </td>
      <td className={TD_BASE}>
        <Link
          href={`/address/${contract.address}`}
          className="text-accent hover:underline font-mono text-sm"
        >
          <span className="hidden sm:inline">{truncateAddress(contract.address, 10, 8)}</span>
          <span className="sm:hidden">{truncateAddress(contract.address, 6, 4)}</span>
        </Link>
      </td>
      <td className={`hidden sm:table-cell ${TD_BASE}`}>{typeBadge}</td>
      {showDecimalsAndSupply && (
        <td className={`hidden md:table-cell ${TD_BASE} text-text-secondary text-sm`}>
          {contract.decimals ?? '-'}
        </td>
      )}
      {showDecimalsAndSupply && (
        <td className={`hidden lg:table-cell ${TD_BASE} text-text-secondary text-sm font-mono`}>
          {formatTotalSupply(contract.totalSupply, contract.decimals)}
        </td>
      )}
      <td className={`hidden lg:table-cell ${TD_BASE}`}>
        {contract.creatorAddress ? (
          <Link
            href={`/address/${contract.creatorAddress}`}
            className="text-text-secondary hover:text-accent font-mono text-sm"
          >
            {truncateAddress(contract.creatorAddress, 6, 4)}
          </Link>
        ) : (
          <span className="text-text-muted text-sm">-</span>
        )}
      </td>
      <td className={`hidden xl:table-cell ${TD_BASE} text-text-secondary text-sm font-mono`}>
        {contract.creationBlockNumber ? (
          <Link
            href={`/block/${formatBlockNumber(contract.creationBlockNumber).replace(/,/g, '')}`}
            className="text-text-secondary hover:text-accent"
          >
            #{formatBlockNumber(contract.creationBlockNumber)}
          </Link>
        ) : (
          '-'
        )}
      </td>
    </tr>
  );
}
