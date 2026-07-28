'use client';

import { useMemo, useState } from 'react';
import Link from 'next/link';
import {
  createColumnHelper,
  getCoreRowModel,
  getFilteredRowModel,
  useReactTable,
} from '@tanstack/react-table';
import type { ColumnDef, Row } from '@tanstack/react-table';
import {
  formatAddress,
  formatAmount,
  formatTimestamp,
  normalizeHexString,
  NATIVE_UNIT,
} from '../../../lib/helpers';
import CopyButton from '../../../components/CopyButton';
import DebouncedInput from '../../../components/DebouncedInput';
import { DownloadBtn } from '../../../components/DownloadBtn';
import EmptyState from '../../../components/EmptyState';
import type { Transaction } from '@/app/types';
import {
  Paginator,
  renderTableBody,
  renderTableHeader,
  truncateMiddle,
  useIsMobile,
} from './_table-utils';
import {
  PAGE_LIMIT,
  fetchAllAggregateRows,
  useAggregatePage,
} from './use-aggregate-page';
import { useUrlIntParam } from '../../../lib/use-url-param';

/**
 * Native transactions panel with server-side pagination.
 *
 * The parent address-view seeds page 1 from `/address/aggregate/:addr`
 * (kept live by the AddressTabs 15s poll); pages > 1 are fetched on demand
 * through useAggregatePage. The old TanStack client pagination only ever
 * paged the single server page it was handed, so any address with more
 * than one page of history rendered as "Page 1 of 1" with its 10 newest
 * rows and no way to reach the rest.
 *
 * Consequences of paging server-side:
 *   - The search box filters the page on screen, not the whole history.
 *   - Download exports the full history (fetchAllAggregateRows), not just
 *     the loaded rows.
 *   - The Number column counts down from the true total, so row numbers
 *     stay stable across pages.
 *
 * Notable departures from the pre-refactor TanStackTable Transactions tab:
 *   - "Transaction Type" column dropped. The syncer writes TxType as a hex
 *     string ("0x2") rather than the int the old TX_TYPE_MAP lookup
 *     expected, so the cell rendered empty for every row. Removed rather
 *     than translated because the value adds no information the In/Out
 *     column doesn't already convey.
 *   - "Paid Fees" renders in Quanta with up to 8 decimals; the wire value is
 *     a decimal-Quanta string like "0.000078750000357000" the old
 *     calculateFees treated as 0.
 */

const IN_OUT_MAP = ['Out', 'In'] as const;

const columnHelper = createColumnHelper<
  Transaction & { formattedAmount: string; formattedFees: string }
>();


interface TransactionsPanelProps {
  address: string;
  /** Page-1 rows from the parent's live aggregate poll. */
  transactions: Transaction[];
  /** True total from transactions_count, drives the page count. */
  total: number;
}

export default function TransactionsPanel({
  address,
  transactions,
  total,
}: TransactionsPanelProps): JSX.Element {
  const [filter, setFilter] = useState('');
  // URL-backed page (?txPage) so browser Back restores it; read during
  // render, so a lazy-mounted panel hydrates the page before its first
  // fetch. replace (the hook default) keeps tab-internal paging out of
  // browser history.
  const [page, setPage] = useUrlIntParam('txPage', 1);
  const isMobile = useIsMobile();

  const pageQuery = useAggregatePage(address, page);
  const pageRows = pageQuery.data?.transactions_by_address;
  const rows = useMemo(
    () => (page === 1 ? transactions : pageRows ?? []),
    [page, transactions, pageRows],
  );

  // Guard against a zero/absent total (e.g. count query failed server-side)
  // so the Number column and page count still make sense for the rows we
  // actually have.
  const effectiveTotal = total > 0 ? total : rows.length;
  const pageCount = Math.max(1, Math.ceil(effectiveTotal / PAGE_LIMIT));

  const data = useMemo(
    () =>
      rows.map((tx) => {
        const [amount, amountUnit] = formatAmount(tx.Amount);
        // PaidFees comes off the wire as a decimal-Quanta string
        // ("0.0000787..."). Render in Quanta with up to 8 decimals,
        // Etherscan-style ("0.00428184 Quanta"). Cast via unknown because
        // the type still says `number?` even though the wire shape is
        // a string. Trailing zeros trim out via parseFloat round-trip
        // so 0.10000000 displays as 0.1, not 0.10000000.
        const rawFees = tx.PaidFees as unknown;
        const feeQuanta =
          typeof rawFees === 'string' || typeof rawFees === 'number'
            ? parseFloat(String(rawFees))
            : 0;
        const formattedFees = Number.isFinite(feeQuanta)
          ? `${parseFloat(feeQuanta.toFixed(8))} ${NATIVE_UNIT}`
          : `0 ${NATIVE_UNIT}`;
        return {
          ...tx,
          formattedAmount: `${amount} ${amountUnit}`,
          formattedFees,
        };
      }),
    [rows],
  );

  const columns = useMemo(
    () => [
      columnHelper.accessor(() => '', {
        id: 'Number',
        header: 'Number',
        // Descending position within the full history, newest = total.
        // row.index is the position in this page's data array, stable
        // under the in-page search filter.
        cell: (info) => (
          <span>{effectiveTotal - ((page - 1) * PAGE_LIMIT + info.row.index)}</span>
        ),
      }),
      columnHelper.accessor('InOut', {
        header: 'In/Out',
        cell: (info) => <span>{IN_OUT_MAP[info.getValue()]}</span>,
      }),
      columnHelper.accessor((row) => ({ from: row.From, to: row.To }), {
        id: 'Addresses',
        header: 'From/To',
        cell: (info) => {
          const { from, to } = info.getValue();
          const fromAddress = from
            ? formatAddress('0x' + normalizeHexString(from))
            : '';
          const toAddress = to
            ? formatAddress('0x' + normalizeHexString(to))
            : '';
          return (
            <div className="flex flex-col gap-1">
              {fromAddress && (
                <div className="flex items-center gap-1">
                  <span className="text-text-secondary text-sm">From:</span>
                  <Link href={'/address/' + fromAddress} title={fromAddress}>
                    {truncateMiddle(fromAddress)}
                  </Link>
                </div>
              )}
              {toAddress && (
                <div className="flex items-center gap-1">
                  <span className="text-text-secondary text-sm">To:</span>
                  <Link href={'/address/' + toAddress} title={toAddress}>
                    {truncateMiddle(toAddress)}
                  </Link>
                </div>
              )}
            </div>
          );
        },
      }),
      columnHelper.accessor('TxHash', {
        header: 'Transaction Hash',
        cell: (info) => {
          const fullHash = '0x' + normalizeHexString(info.getValue());
          return (
            <div className="flex items-center gap-2">
              <Link href={'/tx/' + fullHash} title={fullHash}>
                {truncateMiddle(fullHash)}
              </Link>
              <CopyButton value={fullHash} label="Copy hash" size="sm" stopPropagation />
            </div>
          );
        },
      }),
      columnHelper.accessor('TimeStamp', {
        header: 'Timestamp',
        cell: (info) => <span>{formatTimestamp(info.getValue())}</span>,
      }),
      columnHelper.accessor('formattedAmount', {
        header: 'Amount',
        cell: (info) => <span>{info.getValue()}</span>,
      }),
      columnHelper.accessor('formattedFees', {
        header: 'Paid Fees',
        cell: (info) => <span>{info.getValue()}</span>,
      }),
    ],
    [effectiveTotal, page],
  );

  const table = useReactTable({
    data,
    // @ts-expect-error - ColumnDef types conflict with the index signature on Transaction.
    columns: columns as ColumnDef<Transaction & { formattedAmount: string; formattedFees: string }>[],
    state: { globalFilter: filter },
    onGlobalFilterChange: setFilter,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
  });

  if (effectiveTotal === 0 && page === 1) {
    return (
      <EmptyState
        title="No transactions yet"
        description="This address has no native transaction history."
        actionLabel="Explore latest transactions"
        actionHref="/transactions/1"
      />
    );
  }

  const renderCard = (
    row: Row<Transaction & { formattedAmount: string; formattedFees: string }>,
  ): JSX.Element => {
    const r = row.original;
    return (
      <div key={row.id} className="p-4 border-b border-border last:border-b-0">
        <div className="space-y-3">
          <div className="flex justify-end">
            <div className="px-2 py-1 rounded bg-surface-2">
              <span className="text-xs text-accent">{IN_OUT_MAP[r.InOut]}</span>
            </div>
          </div>

          <div>
            <div className="text-xs text-text-secondary">Transaction Hash</div>
            <div className="flex items-start gap-2">
              <Link
                href={'/tx/0x' + normalizeHexString(r.TxHash)}
                className="text-sm text-accent hover:text-accent-hover break-all"
              >
                {'0x' + normalizeHexString(r.TxHash)}
              </Link>
              <CopyButton
                value={'0x' + normalizeHexString(r.TxHash)}
                label="Copy hash"
                size="sm"
                stopPropagation
              />
            </div>
          </div>

          {r.From && (
            <div>
              <div className="text-xs text-text-secondary">From</div>
              <Link
                href={'/address/' + formatAddress('0x' + normalizeHexString(r.From))}
                className="text-sm text-accent hover:text-accent-hover break-all"
              >
                {truncateMiddle(formatAddress('0x' + normalizeHexString(r.From)))}
              </Link>
            </div>
          )}

          {r.To && (
            <div>
              <div className="text-xs text-text-secondary">To</div>
              <Link
                href={'/address/' + formatAddress('0x' + normalizeHexString(r.To))}
                className="text-sm text-accent hover:text-accent-hover break-all"
              >
                {truncateMiddle(formatAddress('0x' + normalizeHexString(r.To)))}
              </Link>
            </div>
          )}

          <div className="grid grid-cols-2 gap-4">
            <div>
              <div className="text-xs text-text-secondary">Amount</div>
              <div className="text-sm text-text-primary">{r.formattedAmount}</div>
            </div>
            <div>
              <div className="text-xs text-text-secondary">Fees</div>
              <div className="text-sm text-text-primary">{r.formattedFees}</div>
            </div>
          </div>

          <div>
            <div className="text-xs text-text-secondary">Time</div>
            <div className="text-sm text-text-primary">{formatTimestamp(r.TimeStamp)}</div>
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
            {effectiveTotal.toLocaleString('en-US')} transaction
            {effectiveTotal === 1 ? '' : 's'}
          </div>
          <div className="flex items-center space-x-4">
            <DebouncedInput
              value={filter}
              onChange={(v) => setFilter(String(v))}
              className="px-4 py-2 text-sm bg-background border border-border rounded-lg focus:outline-none focus:border-accent text-text-primary w-full md:w-auto"
              placeholder="Search this page..."
            />
            <DownloadBtn
              data={rows}
              fileName={`transactions-${address}`}
              getData={() =>
                fetchAllAggregateRows(address, (p) => p.transactions_by_address)
              }
            />
          </div>
        </div>
      </div>

      {pageQuery.isError && page > 1 && (
        <div className="p-4 text-sm text-error border-b border-border">
          Failed to load page {page}.{' '}
          <button
            type="button"
            className="underline hover:text-red-300"
            onClick={() => { void pageQuery.refetch(); }}
          >
            Retry
          </button>
        </div>
      )}

      <div
        className={`overflow-x-auto${
          pageQuery.isFetching && page > 1 ? ' opacity-60' : ''
        }`}
      >
        {rows.length === 0 && pageQuery.isFetching ? (
          // First visit to a page > 1 has no previous query data to hold
          // on screen (page 1 lives in the parent poll), so show an
          // explicit loading state instead of an empty table.
          <div className="p-8 text-center text-text-secondary">Loading transactions...</div>
        ) : isMobile ? (
          <div className="overflow-hidden">
            {table.getRowModel().rows.map((row) => renderCard(row))}
          </div>
        ) : (
          <table aria-label="Transaction history" className="w-full">
            <thead>{renderTableHeader(table)}</thead>
            <tbody>{renderTableBody(table)}</tbody>
          </table>
        )}
      </div>

      <Paginator
        pageIndex={page - 1}
        pageCount={pageCount}
        canPrev={page > 1}
        canNext={page < pageCount}
        goFirst={() => setPage(1)}
        goPrev={() => setPage(Math.max(1, page - 1))}
        goNext={() => setPage(Math.min(pageCount, page + 1))}
        goLast={() => setPage(pageCount)}
      />
    </div>
  );
}
