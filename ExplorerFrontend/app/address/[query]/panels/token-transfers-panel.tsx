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
import type { ColumnDef, Row } from '@tanstack/react-table';
import { formatTimestamp, formatTokenAmount } from '../../../lib/helpers';
import CopyButton from '../../../components/CopyButton';
import DebouncedInput from '../../../components/DebouncedInput';
import config from '../../../../config';
import {
  Paginator,
  renderTableBody,
  renderTableHeader,
  truncateMiddle,
  useIsMobile,
} from './_table-utils';
import { useUrlIntParam } from '../../../lib/use-url-param';

/**
 * Token Transfers panel for the address page. Lazy-fetches up to 250 rows
 * from `/address/:addr/token-transfers` on mount so the in-table search +
 * pagination work without per-page server round-trips.
 *
 * The badge in the parent tab bar reads the server's unbounded `total`
 * (exposed by `setTotal` via the `onLoaded` callback) rather than
 * `rows.length`, which can be clipped at 250.
 *
 * Resets state on `address` prop change. The address-page parent reuses
 * the same panel instance for /address/A → /address/B navigations, and
 * without this reset the previous holder's rows would briefly render.
 */

interface TokenTransferRow {
  contractAddress: string;
  from: string;
  to: string;
  amount: string;
  blockNumber: string;
  txHash: string;
  logIndex?: string;
  timestamp: string;
  tokenSymbol: string;
  tokenDecimals: number;
  tokenName: string;
  transferType: string;
  tokenStandard?: string;
  tokenID?: string;
}

// TanStack's default 10/page, made explicit now that pagination state is
// controlled (URL-backed ?ttPage).
const TRANSFERS_PAGE_SIZE = 10;

const columnHelper = createColumnHelper<
  TokenTransferRow & { formattedAmount: string; tsSeconds: number }
>();

interface TokenTransfersPanelProps {
  address: string;
  /** Called with the server-reported total whenever a fetch completes. The
   *  parent surfaces it on the tab badge. */
  onLoaded?: (total: number) => void;
}

export default function TokenTransfersPanel({
  address,
  onLoaded,
}: TokenTransfersPanelProps): JSX.Element {
  const [rows, setRows] = useState<TokenTransferRow[]>([]);
  const [total, setTotal] = useState(0);
  const [loaded, setLoaded] = useState(false);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [filter, setFilter] = useState('');
  // URL-backed page (?ttPage, 1-based) so browser Back restores it. Read
  // during render, so a lazy-mounted panel hydrates its page before the
  // rows arrive; autoResetPageIndex is off below because the post-mount
  // fetch swapping `data` in would otherwise clobber the restored page.
  const [pageParam, setPageParam] = useUrlIntParam('ttPage', 1);
  const pagination = { pageIndex: pageParam - 1, pageSize: TRANSFERS_PAGE_SIZE };
  const isMobile = useIsMobile();

  useEffect(() => {
    let cancelled = false;
    setRows([]);
    setTotal(0);
    setLoaded(false);
    setError(null);
    setFilter('');
    setLoading(true);
    (async () => {
      try {
        // page-1-indexed on the wire (matches sibling /address routes).
        // limit=250 matches the backend cap from PR #112 so the search box
        // filters across the full holder history without paging server-side.
        const res = await axios.get(
          `${config.handlerUrl}/address/${address}/token-transfers`,
          { params: { page: 1, limit: 250 } },
        );
        if (cancelled) return;
        const next: TokenTransferRow[] = Array.isArray(res.data?.transfers)
          ? res.data.transfers
          : [];
        const t = typeof res.data?.total === 'number' ? res.data.total : next.length;
        setRows(next);
        setTotal(t);
        setLoaded(true);
        onLoaded?.(t);
      } catch (err) {
        if (cancelled) return;
        console.error('Failed to load token transfers:', err);
        setError('Failed to load token transfers');
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [address, onLoaded]);

  const data = useMemo(
    () =>
      rows.map((row) => {
        const decimals = typeof row.tokenDecimals === 'number' ? row.tokenDecimals : 0;
        let tsSeconds = 0;
        if (row.timestamp) {
          try {
            tsSeconds = row.timestamp.startsWith('0x')
              ? parseInt(row.timestamp, 16)
              : parseInt(row.timestamp, 10);
            if (Number.isNaN(tsSeconds)) tsSeconds = 0;
          } catch {
            tsSeconds = 0;
          }
        }
        return {
          ...row,
          formattedAmount: formatTokenAmount(row.amount, decimals),
          tsSeconds,
        };
      }),
    [rows],
  );

  const columns = useMemo(
    () => [
      columnHelper.accessor(
        (row) => ({
          name: row.tokenName,
          symbol: row.tokenSymbol,
          standard: row.tokenStandard,
          tokenID: row.tokenID,
          contractAddress: row.contractAddress,
        }),
        {
          id: 'Token',
          header: 'Token',
          cell: (info) => {
            const { name, symbol, standard, tokenID, contractAddress } = info.getValue();
            const label =
              name || symbol || (contractAddress ? truncateMiddle(contractAddress) : 'Token');
            // Surface QRC-X branding on the row (DB rows stay ERC-X).
            const badge = standard ? standard.replace(/^ERC-/, 'QRC-') : 'Token';
            return (
              <div className="flex flex-col">
                <Link
                  href={`/address/${contractAddress}`}
                  className="text-accent hover:text-accent-hover font-medium"
                  title={contractAddress}
                >
                  {label}
                </Link>
                <div className="flex items-center gap-2 text-xs text-text-secondary">
                  <span className="font-mono">{badge}</span>
                  {tokenID && <span className="font-mono">#{tokenID}</span>}
                  {symbol && symbol !== name && (
                    <span className="font-mono">({symbol})</span>
                  )}
                </div>
              </div>
            );
          },
        },
      ),
      columnHelper.accessor((row) => ({ from: row.from, to: row.to }), {
        id: 'Parties',
        header: 'From/To',
        cell: (info) => {
          const { from, to } = info.getValue();
          return (
            <div className="flex flex-col gap-1">
              {from && (
                <div className="flex items-center gap-1">
                  <span className="text-text-secondary text-sm">From:</span>
                  <Link
                    href={`/address/${from}`}
                    title={from}
                    className="text-accent hover:text-accent-hover"
                  >
                    {truncateMiddle(from)}
                  </Link>
                </div>
              )}
              {to && (
                <div className="flex items-center gap-1">
                  <span className="text-text-secondary text-sm">To:</span>
                  <Link
                    href={`/address/${to}`}
                    title={to}
                    className="text-accent hover:text-accent-hover"
                  >
                    {truncateMiddle(to)}
                  </Link>
                </div>
              )}
            </div>
          );
        },
      }),
      columnHelper.accessor('formattedAmount', {
        header: 'Amount',
        cell: (info) => {
          const r = info.row.original;
          return (
            <span className="font-mono">
              {info.getValue()}
              {r.tokenSymbol && <span className="text-text-muted ml-2">{r.tokenSymbol}</span>}
            </span>
          );
        },
      }),
      columnHelper.accessor('txHash', {
        header: 'Transaction Hash',
        cell: (info) => {
          const fullHash = info.getValue();
          return (
            <div className="flex items-center gap-2">
              <Link
                href={`/tx/${fullHash}`}
                title={fullHash}
                className="text-accent hover:text-accent-hover"
              >
                {truncateMiddle(fullHash)}
              </Link>
              <CopyButton value={fullHash} label="Copy hash" size="sm" stopPropagation />
            </div>
          );
        },
      }),
      columnHelper.accessor('tsSeconds', {
        header: 'Timestamp',
        cell: (info) => <span>{formatTimestamp(info.getValue())}</span>,
      }),
    ],
    [],
  );

  const table = useReactTable({
    data,
    columns: columns as ColumnDef<
      TokenTransferRow & { formattedAmount: string; tsSeconds: number }
    >[],
    state: { globalFilter: filter, pagination },
    onGlobalFilterChange: setFilter,
    onPaginationChange: (updater) => {
      const next = typeof updater === 'function' ? updater(pagination) : updater;
      setPageParam(next.pageIndex + 1);
    },
    autoResetPageIndex: false,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  });

  if (loading && !loaded) {
    return <div className="p-8 text-center text-text-secondary">Loading token transfers...</div>;
  }
  if (error) {
    return <div className="p-8 text-center text-error">{error}</div>;
  }
  if (loaded && rows.length === 0) {
    return (
      <div className="p-8 text-center text-text-secondary">
        No token transfers for this address.
      </div>
    );
  }

  const renderCard = (
    row: Row<TokenTransferRow & { formattedAmount: string; tsSeconds: number }>,
  ): JSX.Element => {
    const r = row.original;
    const badge = r.tokenStandard ? r.tokenStandard.replace(/^ERC-/, 'QRC-') : 'Token';
    const label = r.tokenName || r.tokenSymbol || r.contractAddress;
    return (
      <div key={row.id} className="p-4 border-b border-border last:border-b-0">
        <div className="space-y-3">
          <div className="flex justify-between items-start">
            <div className="space-y-1">
              <div className="text-xs text-text-secondary">Token</div>
              <Link
                href={`/address/${r.contractAddress}`}
                className="text-sm text-accent hover:text-accent-hover break-all"
              >
                {label}
              </Link>
              <div className="text-xs text-text-secondary font-mono">
                {badge}
                {r.tokenID ? ` #${r.tokenID}` : ''}
              </div>
            </div>
            <div className="px-2 py-1 rounded bg-surface-2">
              <span className="text-xs text-accent">
                {r.formattedAmount}
                {r.tokenSymbol ? ' ' + r.tokenSymbol : ''}
              </span>
            </div>
          </div>

          <div>
            <div className="text-xs text-text-secondary">Transaction Hash</div>
            <div className="flex items-start gap-2">
              <Link
                href={`/tx/${r.txHash}`}
                className="text-sm text-accent hover:text-accent-hover break-all"
              >
                {r.txHash}
              </Link>
              <CopyButton value={r.txHash} label="Copy hash" size="sm" stopPropagation />
            </div>
          </div>

          {r.from && (
            <div>
              <div className="text-xs text-text-secondary">From</div>
              <Link
                href={`/address/${r.from}`}
                className="text-sm text-accent hover:text-accent-hover break-all"
              >
                {truncateMiddle(r.from)}
              </Link>
            </div>
          )}

          {r.to && (
            <div>
              <div className="text-xs text-text-secondary">To</div>
              <Link
                href={`/address/${r.to}`}
                className="text-sm text-accent hover:text-accent-hover break-all"
              >
                {truncateMiddle(r.to)}
              </Link>
            </div>
          )}

          <div>
            <div className="text-xs text-text-secondary">Time</div>
            <div className="text-sm text-text-primary">{formatTimestamp(r.tsSeconds)}</div>
          </div>
        </div>
      </div>
    );
  };

  return (
    <div className="w-full">
      <div className="p-4 border-b border-border">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div className="text-sm text-text-secondary">
            {total.toLocaleString('en-US')} transfer{total === 1 ? '' : 's'}
            {rows.length < total && (
              <span className="ml-2 text-xs">
                (showing {rows.length.toLocaleString('en-US')})
              </span>
            )}
          </div>
          <DebouncedInput
            value={filter}
            onChange={(v) => {
              setFilter(String(v));
              // Manual reset since autoResetPageIndex is off: a new search
              // starts from page 1.
              setPageParam(1);
            }}
            className="px-4 py-2 text-sm bg-background border border-border rounded-lg focus:outline-none focus:border-accent text-text-primary w-full md:w-auto"
            placeholder="Search token transfers..."
          />
        </div>
      </div>

      <div className="overflow-x-auto">
        {isMobile ? (
          <div className="overflow-hidden">
            {table.getRowModel().rows.map((row) => renderCard(row))}
          </div>
        ) : (
          <table aria-label="Token transfers" className="w-full">
            <thead>{renderTableHeader(table)}</thead>
            <tbody>{renderTableBody(table)}</tbody>
          </table>
        )}
      </div>

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
