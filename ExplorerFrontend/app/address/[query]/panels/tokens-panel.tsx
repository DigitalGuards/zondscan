'use client';

import { useEffect, useMemo, useState } from 'react';
import axios from 'axios';
import Link from 'next/link';
import {
  createColumnHelper,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  useReactTable,
} from '@tanstack/react-table';
import { formatTokenAmount } from '../../../lib/helpers';
import DebouncedInput from '../../../components/DebouncedInput';
import config from '../../../../config';
import {
  PAGE_SIZE,
  Paginator,
  renderTableBody,
  renderTableHeader,
} from './_table-utils';

/**
 * QRC-20 holdings panel. Lazy-fetches `/address/:addr/tokens` on first
 * mount, filters the response to ERC-20 (and legacy un-tagged) rows so
 * per-tokenID NFT rows don't double-list with the NFTs panel.
 *
 * Resets on `address` prop change. Notifies the parent of the post-fetch
 * count via `onLoaded` so the tab badge can reflect it.
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

const columnHelper = createColumnHelper<TokenBalanceRow>();

interface TokensPanelProps {
  address: string;
  onLoaded?: (count: number) => void;
}

export default function TokensPanel({ address, onLoaded }: TokensPanelProps): JSX.Element {
  const [rows, setRows] = useState<TokenBalanceRow[]>([]);
  const [loaded, setLoaded] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState('');

  useEffect(() => {
    let cancelled = false;
    setRows([]);
    setLoaded(false);
    setError(null);
    setFilter('');
    setLoading(true);
    (async () => {
      try {
        const res = await axios.get(`${config.handlerUrl}/address/${address}/tokens`);
        if (cancelled) return;
        const arr: TokenBalanceRow[] = Array.isArray(res.data?.tokens) ? res.data.tokens : [];
        // /address/:addr/tokens returns ERC-20 balances AND legacy untagged
        // rows from before tokenStandard existed. Filter to ERC-20 / untagged
        // so NFT rows (which have their own panel) don't appear twice.
        const next = arr.filter((t) => !t.tokenStandard || t.tokenStandard === 'ERC-20');
        setRows(next);
        setLoaded(true);
        onLoaded?.(next.length);
      } catch (err) {
        if (cancelled) return;
        console.error('Failed to load tokens:', err);
        setError('Failed to load tokens');
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [address, onLoaded]);

  const columns = useMemo(
    () => [
      columnHelper.accessor((row) => row.name || row.symbol || row.contractAddress, {
        id: 'Token',
        header: 'Token',
        cell: (info) => {
          const r = info.row.original;
          const label = r.name || r.symbol || r.contractAddress;
          return (
            <div className="min-w-0">
              <Link
                href={`/address/${r.contractAddress}`}
                className="text-sm text-accent hover:text-accent-hover transition-colors font-medium break-all"
                title={r.contractAddress}
              >
                {label}
              </Link>
              {r.symbol && r.symbol !== r.name && (
                <span className="text-text-muted ml-2 font-mono text-xs">({r.symbol})</span>
              )}
            </div>
          );
        },
      }),
      columnHelper.accessor((row) => row.balance, {
        id: 'Balance',
        header: 'Balance',
        cell: (info) => {
          const r = info.row.original;
          const formatted = formatTokenAmount(r.balance, r.decimals ?? 18);
          return (
            <span className="text-sm font-mono text-text-primary break-all">
              {formatted}
              {r.symbol && <span className="text-text-muted ml-2">{r.symbol}</span>}
            </span>
          );
        },
      }),
    ],
    [],
  );

  const table = useReactTable({
    data: rows,
    columns,
    state: { globalFilter: filter },
    onGlobalFilterChange: setFilter,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
    initialState: { pagination: { pageIndex: 0, pageSize: PAGE_SIZE } },
  });

  if (loading && !loaded) {
    return <div className="p-8 text-center text-text-secondary">Loading tokens...</div>;
  }
  if (error) {
    return <div className="p-8 text-center text-error">{error}</div>;
  }
  if (loaded && rows.length === 0) {
    return <div className="p-8 text-center text-text-secondary">No tokens held by this address.</div>;
  }

  return (
    <div className="w-full">
      <div className="p-4 border-b border-border">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div className="text-sm text-text-secondary">
            {rows.length} QRC-20 token{rows.length === 1 ? '' : 's'} held
          </div>
          <DebouncedInput
            value={filter}
            onChange={(v) => setFilter(String(v))}
            className="px-4 py-2 text-sm bg-background border border-border rounded-lg focus:outline-none focus:border-accent text-text-primary w-full md:w-auto"
            placeholder="Search by name, symbol, address"
          />
        </div>
      </div>

      {table.getRowModel().rows.length === 0 ? (
        <div className="p-6 text-center text-sm text-text-secondary">No tokens match your search.</div>
      ) : (
        <div className="overflow-x-auto">
          <table aria-label="Token holdings table" className="w-full">
            <thead>{renderTableHeader(table)}</thead>
            <tbody>{renderTableBody(table)}</tbody>
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
