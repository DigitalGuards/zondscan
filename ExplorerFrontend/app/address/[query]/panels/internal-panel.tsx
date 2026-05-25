'use client';

import { useMemo, useState } from 'react';
import Link from 'next/link';
import {
  createColumnHelper,
  getCoreRowModel,
  getFilteredRowModel,
  getPaginationRowModel,
  useReactTable,
} from '@tanstack/react-table';
import type { ColumnDef, Row } from '@tanstack/react-table';
import {
  formatAddress,
  formatAmount,
  formatTimestamp,
  normalizeHexString,
} from '../../../lib/helpers';
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

/**
 * Internal-transactions panel. Sub-frame calls emitted by contract
 * interactions (CALL / DELEGATECALL / CREATE). Rows come from the
 * `/address/aggregate/:addr` response that the parent already fetched.
 *
 * No `total` count badge: the aggregate doesn't return one for internal
 * transactions, and a dedicated endpoint doesn't exist today. Adding
 * either is out of scope for this refactor.
 */

const columnHelper = createColumnHelper<
  InternalTransaction & { formattedValue: string }
>();

interface InternalPanelProps {
  internalt: InternalTransaction[];
}

export default function InternalPanel({
  internalt,
}: InternalPanelProps): JSX.Element {
  const [filter, setFilter] = useState('');
  const isMobile = useIsMobile();

  const data = useMemo(
    () =>
      internalt.map((tx) => {
        const [value, valueUnit] = formatAmount(tx.Value);
        return { ...tx, formattedValue: `${value} ${valueUnit}` };
      }),
    [internalt],
  );

  const columns = useMemo(
    () => [
      columnHelper.accessor(() => '', {
        id: 'Number',
        header: 'Number',
        cell: (info) => <span>{internalt.length - info.row.index}</span>,
      }),
      columnHelper.accessor('Type', {
        header: 'Type',
        cell: (info) => <span>{atob(String(info.getValue()))}</span>,
      }),
      columnHelper.accessor('From', {
        header: 'From',
        cell: (info) => {
          const a = formatAddress('0x' + normalizeHexString(info.getValue()));
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
          const a = formatAddress('0x' + normalizeHexString(info.getValue()));
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
            <Link href={'/tx/' + h} title={h}>
              {truncateMiddle(h)}
            </Link>
          );
        },
      }),
      columnHelper.accessor('formattedValue', {
        header: 'Value',
        cell: (info) => <span>{info.getValue()}</span>,
      }),
      columnHelper.accessor('GasUsed', {
        header: 'Gas Used (in Units)',
        cell: (info) => <span>{info.getValue()} Units</span>,
      }),
      columnHelper.accessor('AmountFunctionIdentifier', {
        header: 'Token Units',
        cell: (info) => <span>{info.getValue()}</span>,
      }),
      columnHelper.accessor('BlockTimestamp', {
        header: 'Timestamp',
        cell: (info) => <span>{formatTimestamp(info.getValue())}</span>,
      }),
      columnHelper.accessor('Output', {
        header: 'Status',
        cell: (info) => (
          <span>{info.getValue() === 1 ? 'Success' : 'Failure'}</span>
        ),
      }),
    ],
    [internalt.length],
  );

  const table = useReactTable({
    data,
    // @ts-expect-error - ColumnDef types conflict with index signature.
    columns: columns as ColumnDef<InternalTransaction & { formattedValue: string }>[],
    state: { globalFilter: filter },
    onGlobalFilterChange: setFilter,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  });

  if (internalt.length === 0) {
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
      <div key={row.id} className="p-4 border-b border-[#3d3d3d] last:border-b-0">
        <div className="space-y-3">
          <div>
            <div className="text-xs text-gray-400">Type</div>
            <div className="text-sm text-white">{atob(String(r.Type))}</div>
          </div>

          <div>
            <div className="text-xs text-gray-400">Transaction Hash</div>
            <Link
              href={'/tx/0x' + normalizeHexString(r.Hash)}
              className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
            >
              {'0x' + normalizeHexString(r.Hash)}
            </Link>
          </div>

          <div>
            <div className="text-xs text-gray-400">From</div>
            <Link
              href={'/address/' + formatAddress('0x' + normalizeHexString(r.From))}
              className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
            >
              {truncateMiddle(formatAddress('0x' + normalizeHexString(r.From)))}
            </Link>
          </div>

          <div>
            <div className="text-xs text-gray-400">To</div>
            <Link
              href={'/address/' + formatAddress('0x' + normalizeHexString(r.To))}
              className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
            >
              {truncateMiddle(formatAddress('0x' + normalizeHexString(r.To)))}
            </Link>
          </div>

          <div className="grid grid-cols-2 gap-4">
            <div>
              <div className="text-xs text-gray-400">Value</div>
              <div className="text-sm text-white">{r.formattedValue}</div>
            </div>
            <div>
              <div className="text-xs text-gray-400">Gas Used</div>
              <div className="text-sm text-white">{r.GasUsed} Units</div>
            </div>
          </div>

          <div>
            <div className="text-xs text-gray-400">Time</div>
            <div className="text-sm text-white">{formatTimestamp(r.BlockTimestamp)}</div>
          </div>

          <div>
            <div className="text-xs text-gray-400">Status</div>
            <div className="text-sm text-white">
              {r.Output === 1 ? 'Success' : 'Failure'}
            </div>
          </div>
        </div>
      </div>
    );
  };

  return (
    <div className="w-full">
      <div className="p-4 border-b border-[#3d3d3d]">
        <div className="flex flex-col md:flex-row md:items-center justify-between gap-4">
          <div className="text-sm text-gray-400">
            {internalt.length} internal call{internalt.length === 1 ? '' : 's'} loaded
          </div>
          <div className="flex items-center space-x-4">
            <DebouncedInput
              value={filter}
              onChange={(v) => setFilter(String(v))}
              className="px-4 py-2 text-sm bg-[#1a1a1a] border border-[#3d3d3d] rounded-lg focus:outline-none focus:border-[#ffa729] text-white w-full md:w-auto"
              placeholder="Search internal transactions..."
            />
            <DownloadBtnInternal data={internalt} />
          </div>
        </div>
      </div>

      <div className="overflow-x-auto">
        {isMobile ? (
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
