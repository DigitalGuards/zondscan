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
  hexToNumber,
  normalizeHexString,
} from '../../../lib/helpers';
import CopyButton from '../../../components/CopyButton';
import DebouncedInput from '../../../components/DebouncedInput';
import { DownloadBtnInternal } from '../../../components/DownloadBtn';
import EmptyState from '../../../components/EmptyState';
import type { InternalTransaction } from '@/app/types';
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
 * Internal-transactions panel. Sub-frame calls emitted by contract
 * interactions (CALL / DELEGATECALL / CREATE). Server-side paginated the
 * same way as the Transactions panel: page 1 comes from the parent's live
 * aggregate poll, pages > 1 through useAggregatePage.
 *
 * `total` is the aggregate's internal_transactions_count. Handlers
 * predating that field leave it undefined; the panel then degrades to
 * sequential paging (Next enabled while pages come back full) instead of
 * pretending the loaded rows are the whole history.
 */

const columnHelper = createColumnHelper<
  InternalTransaction & { formattedValue: string }
>();

interface InternalPanelProps {
  address: string;
  /** Page-1 rows from the parent's live aggregate poll. */
  internalt: InternalTransaction[];
  /** True total when the handler provides it; undefined = unknown. */
  total?: number;
}

export default function InternalPanel({
  address,
  internalt,
  total,
}: InternalPanelProps): JSX.Element {
  const [filter, setFilter] = useState('');
  // URL-backed page (?itxPage) so browser Back restores it; read during
  // render, so a lazy-mounted panel hydrates the page before its first
  // fetch. replace (the hook default) keeps tab-internal paging out of
  // browser history.
  const [page, setPage] = useUrlIntParam('itxPage', 1);
  const isMobile = useIsMobile();

  const pageQuery = useAggregatePage(address, page);
  const pageRows = pageQuery.data?.internal_transactions_by_address;
  const rows = useMemo(
    () => (page === 1 ? internalt : pageRows ?? []),
    [page, internalt, pageRows],
  );

  const knownTotal = typeof total === 'number' && total > 0 ? total : undefined;
  // Unknown total: assume one more page whenever the current page is full.
  const hasMore = rows.length === PAGE_LIMIT;
  const pageCount = knownTotal
    ? Math.max(1, Math.ceil(knownTotal / PAGE_LIMIT))
    : page + (hasMore ? 1 : 0);

  const data = useMemo(
    () =>
      rows.map((tx) => {
        const [value, valueUnit] = formatAmount(tx.Value);
        return { ...tx, formattedValue: `${value} ${valueUnit}` };
      }),
    [rows],
  );

  const columns = useMemo(
    () => [
      columnHelper.accessor(() => '', {
        id: 'Number',
        header: 'Number',
        // Descending position within the full history when the total is
        // known; page-local numbering otherwise (pre-count handlers).
        cell: (info) => (
          <span>
            {knownTotal
              ? knownTotal - ((page - 1) * PAGE_LIMIT + info.row.index)
              : rows.length - info.row.index}
          </span>
        ),
      }),
      columnHelper.accessor('Type', {
        header: 'Type',
        cell: (info) => <span>{info.getValue() ? String(info.getValue()) : '-'}</span>,
      }),
      columnHelper.accessor('From', {
        header: 'From',
        cell: (info) => {
          const raw = info.getValue();
          if (!raw) return <span>-</span>;
          const a = formatAddress(String(raw));
          return (
            <Link href={'/address/' + a} title={a}>
              {truncateMiddle(a)}
            </Link>
          );
        },
      }),
      columnHelper.accessor('To', {
        header: 'To',
        cell: (info) => {
          const raw = info.getValue();
          if (!raw) return <span>-</span>;
          const a = formatAddress(String(raw));
          return (
            <Link href={'/address/' + a} title={a}>
              {truncateMiddle(a)}
            </Link>
          );
        },
      }),
      columnHelper.accessor('Hash', {
        header: 'Transaction Hash',
        cell: (info) => {
          const h = '0x' + normalizeHexString(info.getValue());
          return (
            <div className="flex items-center gap-2">
              <Link href={'/tx/' + h} title={h}>
                {truncateMiddle(h)}
              </Link>
              <CopyButton value={h} label="Copy hash" size="sm" stopPropagation />
            </div>
          );
        },
      }),
      columnHelper.accessor('formattedValue', {
        header: 'Value',
        cell: (info) => <span>{info.getValue()}</span>,
      }),
      columnHelper.accessor('GasUsed', {
        header: 'Gas Used (in Units)',
        cell: (info) => <span>{hexToNumber(String(info.getValue()))} Units</span>,
      }),
      columnHelper.accessor('AmountFunctionIdentifier', {
        header: 'Token Units',
        cell: (info) => <span>{info.getValue()}</span>,
      }),
      columnHelper.accessor('BlockTimestamp', {
        header: 'Timestamp',
        cell: (info) => <span>{formatTimestamp(info.getValue())}</span>,
      }),
    ],
    [knownTotal, page, rows.length],
  );

  const table = useReactTable({
    data,
    // @ts-expect-error - ColumnDef types conflict with index signature.
    columns: columns as ColumnDef<InternalTransaction & { formattedValue: string }>[],
    state: { globalFilter: filter },
    onGlobalFilterChange: setFilter,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
  });

  if (rows.length === 0 && page === 1) {
    return (
      <EmptyState
        title="No internal transactions"
        description="This address hasn't been involved in any internal contract calls."
      />
    );
  }

  const renderCard = (
    row: Row<InternalTransaction & { formattedValue: string }>,
  ): JSX.Element => {
    const r = row.original;
    return (
      <div key={row.id} className="p-4 border-b border-border last:border-b-0">
        <div className="space-y-3">
          <div>
            <div className="text-xs text-text-secondary">Type</div>
            <div className="text-sm text-text-primary">{r.Type ? String(r.Type) : '-'}</div>
          </div>

          <div>
            <div className="text-xs text-text-secondary">Transaction Hash</div>
            <div className="flex items-start gap-2">
              <Link
                href={'/tx/0x' + normalizeHexString(r.Hash)}
                className="text-sm text-accent hover:text-accent-hover break-all"
              >
                {'0x' + normalizeHexString(r.Hash)}
              </Link>
              <CopyButton
                value={'0x' + normalizeHexString(r.Hash)}
                label="Copy hash"
                size="sm"
                stopPropagation
              />
            </div>
          </div>

          <div>
            <div className="text-xs text-text-secondary">From</div>
            {r.From ? (
              <Link
                href={'/address/' + formatAddress(String(r.From))}
                className="text-sm text-accent hover:text-accent-hover break-all"
              >
                {truncateMiddle(formatAddress(String(r.From)))}
              </Link>
            ) : (
              <div className="text-sm text-text-primary">-</div>
            )}
          </div>

          <div>
            <div className="text-xs text-text-secondary">To</div>
            {r.To ? (
              <Link
                href={'/address/' + formatAddress(String(r.To))}
                className="text-sm text-accent hover:text-accent-hover break-all"
              >
                {truncateMiddle(formatAddress(String(r.To)))}
              </Link>
            ) : (
              <div className="text-sm text-text-primary">-</div>
            )}
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <div className="text-xs text-text-secondary">Value</div>
              <div className="text-sm text-text-primary">{r.formattedValue}</div>
            </div>
            <div>
              <div className="text-xs text-text-secondary">Gas Used</div>
              <div className="text-sm text-text-primary">{hexToNumber(String(r.GasUsed))} Units</div>
            </div>
          </div>

          <div>
            <div className="text-xs text-text-secondary">Time</div>
            <div className="text-sm text-text-primary">{formatTimestamp(r.BlockTimestamp)}</div>
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
            {knownTotal
              ? `${knownTotal.toLocaleString('en-US')} internal call${knownTotal === 1 ? '' : 's'}`
              : `${rows.length} internal call${rows.length === 1 ? '' : 's'} loaded`}
          </div>
          <div className="flex items-center space-x-4">
            <DebouncedInput
              value={filter}
              onChange={(v) => setFilter(String(v))}
              className="px-4 py-2 text-sm bg-background border border-border rounded-lg focus:outline-none focus:border-accent text-text-primary w-full md:w-auto"
              placeholder="Search this page..."
            />
            <DownloadBtnInternal
              data={rows}
              fileName={`internal-transactions-${address}`}
              getData={() =>
                fetchAllAggregateRows(address, (p) => p.internal_transactions_by_address)
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
          <div className="p-8 text-center text-text-secondary">Loading internal transactions...</div>
        ) : isMobile ? (
          <div className="overflow-hidden">
            {table.getRowModel().rows.map((row) => renderCard(row))}
          </div>
        ) : (
          <table aria-label="Internal transactions" className="w-full">
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
