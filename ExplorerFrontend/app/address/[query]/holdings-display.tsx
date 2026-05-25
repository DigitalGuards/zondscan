'use client';

import { useEffect, useMemo, useState } from 'react';
import axios from 'axios';
import Link from 'next/link';
import {
  createColumnHelper,
  flexRender,
  useReactTable,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  type Cell,
  type Header,
  type HeaderGroup,
  type Row,
} from '@tanstack/react-table';
import Badge from '../../components/Badge';
import DebouncedInput from '../../components/DebouncedInput';
import ImageWithFallback from '../../components/ImageWithFallback';
import config from '../../../config';
import { formatTokenAmount } from '../../lib/helpers';

/**
 * Holdings panel for an address page: ERC-20 token balances and per-tokenID
 * NFT (ERC-721 / ERC-1155) holdings. The data has been collected by the
 * syncer for a while now (tokenBalances collection, joined with contractCode
 * + tokenMetadata via the backend's /address/:addr/tokens and /nfts
 * endpoints), it just wasn't surfaced on the address page UI.
 *
 * Fetches both endpoints in parallel on mount. Each section renders
 * independently; a 200 with an empty array shows the empty state for that
 * section but doesn't block the other. Network failures are silent: the
 * section stays empty rather than throwing, matching the rest of the
 * address page's resilience pattern.
 *
 * Pagination + search are now backed by TanStack Table, mirroring the
 * Transactions table's UX: hard cap of 5 rows per page, first/prev/next/last
 * controls, and a name/symbol/contract-address filter. Each section runs its
 * own table so the search input scopes to the section the user is looking at.
 */

interface TokenBalanceRow {
  contractAddress: string;
  balance: string;
  name?: string;
  symbol?: string;
  decimals?: number;
  tokenStandard?: string;
  tokenID?: string;
}

interface NFTBalanceRow {
  contractAddress: string;
  tokenID: string;
  tokenStandard: string;
  balance: string;
  collectionName?: string;
  collectionSymbol?: string;
  name?: string;
  image?: string;
  externalURL?: string;
}

interface HoldingsDisplayProps {
  address: string;
}

const NFT_STANDARDS = new Set(['ERC-721', 'ERC-1155']);
// Match the user-requested holdings page size. The transactions paginator
// uses TanStack's default 10/page; the holdings list is denser per row
// (image tile + multi-line text), so 5 keeps both sections about the same
// vertical height as one paginated transactions page.
const PAGE_SIZE = 5;

const tokenColumnHelper = createColumnHelper<TokenBalanceRow>();
const nftColumnHelper = createColumnHelper<NFTBalanceRow>();

const renderHeader = <T,>(headerGroups: ReturnType<ReturnType<typeof useReactTable<T>>['getHeaderGroups']>): JSX.Element[] => {
  return headerGroups.map((headerGroup: HeaderGroup<T>) => (
    <tr key={headerGroup.id} className="border-b border-border">
      {headerGroup.headers.map((header: Header<T, unknown>) => (
        <th key={header.id} scope="col" className="px-4 py-3 text-left text-xs md:text-sm font-medium text-accent">
          {header.isPlaceholder ? null : flexRender(header.column.columnDef.header, header.getContext())}
        </th>
      ))}
    </tr>
  ));
};

const renderBody = <T,>(rows: Row<T>[]): JSX.Element[] => {
  return rows.map((row) => (
    <tr key={row.id} className="border-b border-border hover:bg-[rgba(255,167,41,0.05)]">
      {row.getVisibleCells().map((cell: Cell<T, unknown>) => (
        <td key={cell.id} className="px-4 py-3 text-sm text-gray-300 align-top">
          {flexRender(cell.column.columnDef.cell, cell.getContext())}
        </td>
      ))}
    </tr>
  ));
};

interface PaginatorProps {
  pageIndex: number;
  pageCount: number;
  canPrev: boolean;
  canNext: boolean;
  goFirst: () => void;
  goPrev: () => void;
  goNext: () => void;
  goLast: () => void;
}

const Paginator = ({ pageIndex, pageCount, canPrev, canNext, goFirst, goPrev, goNext, goLast }: PaginatorProps): JSX.Element => {
  return (
    <div className="flex flex-col md:flex-row items-center justify-between gap-2 p-4 border-t border-border">
      <div className="flex flex-wrap items-center gap-2">
        <button
          aria-label="Go to first page"
          onClick={goFirst}
          disabled={!canPrev}
          className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
        >
          {"<<"}
        </button>
        <button
          aria-label="Go to previous page"
          onClick={goPrev}
          disabled={!canPrev}
          className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
        >
          Previous
        </button>
        <button
          aria-label="Go to next page"
          onClick={goNext}
          disabled={!canNext}
          className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
        >
          Next
        </button>
        <button
          aria-label="Go to last page"
          onClick={goLast}
          disabled={!canNext}
          className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
        >
          {">>"}
        </button>
      </div>
      <div className="text-sm text-gray-400">
        Page {pageIndex + 1} of {Math.max(1, pageCount)}
      </div>
    </div>
  );
};

interface TokensSectionProps {
  tokens: TokenBalanceRow[];
}

function TokensSection({ tokens }: TokensSectionProps): JSX.Element | null {
  const [filter, setFilter] = useState('');

  const columns = useMemo(() => [
    tokenColumnHelper.accessor((row) => row.name || row.symbol || row.contractAddress, {
      id: 'Token',
      header: 'Token',
      cell: (info) => {
        const row = info.row.original;
        const label = row.name || row.symbol || row.contractAddress;
        return (
          <div className="min-w-0">
            <Link
              href={`/address/${row.contractAddress}`}
              className="text-sm text-accent hover:text-accent-hover transition-colors font-medium break-all"
              title={row.contractAddress}
            >
              {label}
            </Link>
            {row.symbol && row.symbol !== row.name && (
              <span className="text-gray-500 ml-2 font-mono text-xs">({row.symbol})</span>
            )}
          </div>
        );
      },
    }),
    tokenColumnHelper.accessor((row) => row.balance, {
      id: 'Balance',
      header: 'Balance',
      cell: (info) => {
        const row = info.row.original;
        const formatted = formatTokenAmount(row.balance, row.decimals ?? 18);
        return (
          <span className="text-sm font-mono text-gray-200 break-all">
            {formatted}
            {row.symbol && <span className="text-gray-500 ml-2">{row.symbol}</span>}
          </span>
        );
      },
    }),
  ], []);

  const table = useReactTable({
    data: tokens,
    columns,
    state: { globalFilter: filter },
    onGlobalFilterChange: setFilter,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: { pagination: { pageIndex: 0, pageSize: PAGE_SIZE } },
  });

  if (tokens.length === 0) return null;

  return (
    <div className="rounded-xl border border-border overflow-hidden" role="region" aria-label="Token holdings">
      <div className="px-4 py-3 border-b border-border bg-[#1a1a1a]/40 flex flex-wrap items-center gap-2 justify-between">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-gray-200">Tokens</span>
          <Badge variant="brand">{tokens.length} QRC-20</Badge>
        </div>
        <DebouncedInput
          value={filter}
          onChange={(v) => setFilter(String(v))}
          className="px-3 py-1.5 text-sm bg-[#1a1a1a] border border-border rounded-lg focus:outline-none focus:border-accent text-white w-full md:w-auto"
          placeholder="Search by name, symbol, address"
        />
      </div>
      {table.getRowModel().rows.length === 0 ? (
        <div className="p-6 text-center text-sm text-gray-400">No tokens match your search.</div>
      ) : (
        <div className="overflow-x-auto">
          <table aria-label="Token holdings table" className="w-full">
            <thead>{renderHeader(table.getHeaderGroups())}</thead>
            <tbody>{renderBody(table.getRowModel().rows)}</tbody>
          </table>
        </div>
      )}
      <Paginator
        pageIndex={table.getState().pagination.pageIndex}
        pageCount={table.getPageCount()}
        canPrev={table.getCanPreviousPage()}
        canNext={table.getCanNextPage()}
        goFirst={() => table.setPageIndex(0)}
        goPrev={() => table.previousPage()}
        goNext={() => table.nextPage()}
        goLast={() => table.setPageIndex(table.getPageCount() - 1)}
      />
    </div>
  );
}

interface NftsSectionProps {
  nfts: NFTBalanceRow[];
}

function NftsSection({ nfts }: NftsSectionProps): JSX.Element | null {
  const [filter, setFilter] = useState('');

  const columns = useMemo(() => [
    nftColumnHelper.accessor((row) => `${row.collectionName ?? ''} ${row.collectionSymbol ?? ''} ${row.name ?? ''} ${row.contractAddress} #${row.tokenID}`, {
      id: 'NFT',
      header: 'NFT',
      cell: (info) => {
        const row = info.row.original;
        const tokenLabel = row.name || `#${row.tokenID}`;
        const collectionLabel = row.collectionName || row.collectionSymbol || row.contractAddress;
        return (
          <div className="flex items-center gap-3 min-w-0">
            {row.image ? (
              // Off-chain metadata images live on arbitrary HTTP / IPFS
              // gateways. ImageWithFallback handles broken URLs by
              // swapping in the tokenID monogram on load failure;
              // unoptimized inside the component skips next/image's
              // domain-list requirement.
              <ImageWithFallback
                src={row.image}
                alt=""
                width={40}
                height={40}
                className="w-10 h-10 rounded-md object-cover bg-[#1a1a1a]"
                fallback={
                  <div className="w-10 h-10 rounded-md bg-[#1a1a1a] border border-border flex items-center justify-center text-xs font-mono text-gray-500">
                    {(row.tokenID || '?').slice(0, 3)}
                  </div>
                }
              />
            ) : (
              <div className="w-10 h-10 rounded-md bg-[#1a1a1a] border border-border flex items-center justify-center text-xs font-mono text-gray-500">
                {(row.tokenID || '?').slice(0, 3)}
              </div>
            )}
            <div className="min-w-0">
              <Link
                href={`/address/${row.contractAddress}`}
                className="text-sm text-accent hover:text-accent-hover transition-colors font-medium break-all"
                title={row.contractAddress}
              >
                {collectionLabel}
              </Link>
              <div className="text-xs text-gray-400 font-mono break-all">
                {tokenLabel}
                {row.name && row.tokenID && ` (#${row.tokenID})`}
              </div>
            </div>
          </div>
        );
      },
    }),
    nftColumnHelper.accessor((row) => row.tokenStandard, {
      id: 'Standard',
      header: 'Standard',
      cell: (info) => {
        const row = info.row.original;
        const standardBadge = row.tokenStandard === 'ERC-1155' ? 'QRC-1155' : 'QRC-721';
        const qty = (() => {
          try {
            return BigInt(row.balance).toLocaleString('en-US');
          } catch {
            return row.balance;
          }
        })();
        return (
          <div className="flex items-center gap-2">
            <Badge variant="warning">{standardBadge}</Badge>
            {row.tokenStandard === 'ERC-1155' && (
              <span className="text-sm font-mono text-gray-200">x{qty}</span>
            )}
          </div>
        );
      },
    }),
  ], []);

  const table = useReactTable({
    data: nfts,
    columns,
    state: { globalFilter: filter },
    onGlobalFilterChange: setFilter,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: { pagination: { pageIndex: 0, pageSize: PAGE_SIZE } },
  });

  if (nfts.length === 0) return null;

  return (
    <div className="rounded-xl border border-border overflow-hidden" role="region" aria-label="NFT holdings">
      <div className="px-4 py-3 border-b border-border bg-[#1a1a1a]/40 flex flex-wrap items-center gap-2 justify-between">
        <div className="flex items-center gap-2">
          <span className="text-sm font-medium text-gray-200">NFTs</span>
          <Badge variant="warning">{nfts.length} item{nfts.length === 1 ? '' : 's'}</Badge>
        </div>
        <DebouncedInput
          value={filter}
          onChange={(v) => setFilter(String(v))}
          className="px-3 py-1.5 text-sm bg-[#1a1a1a] border border-border rounded-lg focus:outline-none focus:border-accent text-white w-full md:w-auto"
          placeholder="Search by collection, tokenID, address"
        />
      </div>
      {table.getRowModel().rows.length === 0 ? (
        <div className="p-6 text-center text-sm text-gray-400">No NFTs match your search.</div>
      ) : (
        <div className="overflow-x-auto">
          <table aria-label="NFT holdings table" className="w-full">
            <thead>{renderHeader(table.getHeaderGroups())}</thead>
            <tbody>{renderBody(table.getRowModel().rows)}</tbody>
          </table>
        </div>
      )}
      <Paginator
        pageIndex={table.getState().pagination.pageIndex}
        pageCount={table.getPageCount()}
        canPrev={table.getCanPreviousPage()}
        canNext={table.getCanNextPage()}
        goFirst={() => table.setPageIndex(0)}
        goPrev={() => table.previousPage()}
        goNext={() => table.nextPage()}
        goLast={() => table.setPageIndex(table.getPageCount() - 1)}
      />
    </div>
  );
}

export default function HoldingsDisplay({ address }: HoldingsDisplayProps): JSX.Element | null {
  const [tokens, setTokens] = useState<TokenBalanceRow[]>([]);
  const [nfts, setNfts] = useState<NFTBalanceRow[]>([]);
  const [loaded, setLoaded] = useState(false);

  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const [tokensRes, nftsRes] = await Promise.allSettled([
          axios.get(`${config.handlerUrl}/address/${address}/tokens`),
          axios.get(`${config.handlerUrl}/address/${address}/nfts`),
        ]);
        if (cancelled) return;
        if (tokensRes.status === 'fulfilled') {
          const arr: TokenBalanceRow[] = Array.isArray(tokensRes.value.data?.tokens)
            ? tokensRes.value.data.tokens
            : [];
          // The /address/:addr/tokens endpoint returns ERC-20 balances and
          // also legacy un-classified rows from before the tokenStandard tag
          // existed. Filter to ERC-20 (or untagged) so per-tokenID NFT rows
          // don't appear twice (they'll show in the NFTs section).
          setTokens(arr.filter((t) => !t.tokenStandard || t.tokenStandard === 'ERC-20'));
        }
        if (nftsRes.status === 'fulfilled') {
          const arr: NFTBalanceRow[] = Array.isArray(nftsRes.value.data?.nfts)
            ? nftsRes.value.data.nfts
            : [];
          setNfts(arr.filter((n) => NFT_STANDARDS.has(n.tokenStandard)));
        }
      } finally {
        if (!cancelled) setLoaded(true);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [address]);

  if (!loaded) return null;
  if (tokens.length === 0 && nfts.length === 0) return null;

  return (
    <section aria-labelledby="holdings-heading" className="space-y-4 md:space-y-6">
      <h2 id="holdings-heading" className="text-base md:text-lg lg:text-xl font-semibold text-accent">Holdings</h2>
      <TokensSection tokens={tokens} />
      <NftsSection nfts={nfts} />
    </section>
  );
}
