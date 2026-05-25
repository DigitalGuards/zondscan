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

/**
 * Native transactions panel. Renders the rows the parent address-view
 * already received from `/address/aggregate/:addr` (no fetch here).
 *
 * Notable departures from the pre-refactor TanStackTable Transactions tab:
 *   - "Transaction Type" column dropped. The syncer writes TxType as a hex
 *     string ("0x2") rather than the int the old TX_TYPE_MAP lookup
 *     expected, so the cell rendered empty for every row. Removed rather
 *     than translated because the value adds no information the In/Out
 *     column doesn't already convey.
 *   - "Paid Fees" now renders in Shor (10^-9 QRL / "Quanta") with a
 *     thousands-separator. The wire value is a decimal-Quanta string like
 *     "0.000078750000357000"; the old calculateFees only accepted a
 *     numeric type and fell back to 0 when handed the string, which is
 *     why the column showed "0.00 QRL" for every native tx. Multiplied
 *     by 1e9 to land in the Shor / Gwei range users actually read.
 */

const IN_OUT_MAP = ['Out', 'In'] as const;

const columnHelper = createColumnHelper<
  Transaction & { formattedAmount: string; formattedFees: string }
>();


interface TransactionsPanelProps {
  transactions: Transaction[];
}

export default function TransactionsPanel({
  transactions,
}: TransactionsPanelProps): JSX.Element {
  const [filter, setFilter] = useState('');
  const isMobile = useIsMobile();

  const data = useMemo(
    () =>
      transactions.map((tx) => {
        const [amount, amountUnit] = formatAmount(tx.Amount);
        // PaidFees comes off the wire as a decimal-Quanta string
        // ("0.0000787..."). Render in QRL with up to 8 decimals,
        // Etherscan-style ("0.00428184 QRL"). Cast via unknown because
        // the type still says `number?` even though the wire shape is
        // a string. Trailing zeros trim out via parseFloat round-trip
        // so 0.10000000 displays as 0.1, not 0.10000000.
        const rawFees = tx.PaidFees as unknown;
        const feeQuanta =
          typeof rawFees === 'string' || typeof rawFees === 'number'
            ? parseFloat(String(rawFees))
            : 0;
        const formattedFees = Number.isFinite(feeQuanta)
          ? `${parseFloat(feeQuanta.toFixed(8))} QRL`
          : '0 QRL';
        return {
          ...tx,
          formattedAmount: `${amount} ${amountUnit}`,
          formattedFees,
        };
      }),
    [transactions],
  );

  const columns = useMemo(
    () => [
      columnHelper.accessor(() => '', {
        id: 'Number',
        header: 'Number',
        cell: (info) => <span>{transactions.length - info.row.index}</span>,
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
                  <span className="text-gray-400 text-sm">From:</span>
                  <Link href={'/address/' + fromAddress} title={fromAddress}>
                    {truncateMiddle(fromAddress)}
                  </Link>
                </div>
              )}
              {toAddress && (
                <div className="flex items-center gap-1">
                  <span className="text-gray-400 text-sm">To:</span>
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
    [transactions.length],
  );

  const table = useReactTable({
    data,
    // @ts-expect-error - ColumnDef types conflict with the index signature on Transaction.
    columns: columns as ColumnDef<Transaction & { formattedAmount: string; formattedFees: string }>[],
    state: { globalFilter: filter },
    onGlobalFilterChange: setFilter,
    getCoreRowModel: getCoreRowModel(),
    getFilteredRowModel: getFilteredRowModel(),
    getPaginationRowModel: getPaginationRowModel(),
  });

  if (transactions.length === 0) {
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
      <div key={row.id} className="p-4 border-b border-[#3d3d3d] last:border-b-0">
        <div className="space-y-3">
          <div className="flex justify-end">
            <div className="px-2 py-1 rounded bg-[#3d3d3d] bg-opacity-40">
              <span className="text-xs text-[#ffa729]">{IN_OUT_MAP[r.InOut]}</span>
            </div>
          </div>

          <div>
            <div className="text-xs text-gray-400">Transaction Hash</div>
            <div className="flex items-start gap-2">
              <Link
                href={'/tx/0x' + normalizeHexString(r.TxHash)}
                className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
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
              <div className="text-xs text-gray-400">From</div>
              <Link
                href={'/address/' + formatAddress('0x' + normalizeHexString(r.From))}
                className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
              >
                {truncateMiddle(formatAddress('0x' + normalizeHexString(r.From)))}
              </Link>
            </div>
          )}

          {r.To && (
            <div>
              <div className="text-xs text-gray-400">To</div>
              <Link
                href={'/address/' + formatAddress('0x' + normalizeHexString(r.To))}
                className="text-sm text-[#ffa729] hover:text-[#ffb954] break-all"
              >
                {truncateMiddle(formatAddress('0x' + normalizeHexString(r.To)))}
              </Link>
            </div>
          )}

          <div className="grid grid-cols-2 gap-4">
            <div>
              <div className="text-xs text-gray-400">Amount</div>
              <div className="text-sm text-white">{r.formattedAmount}</div>
            </div>
            <div>
              <div className="text-xs text-gray-400">Fees</div>
              <div className="text-sm text-white">{r.formattedFees}</div>
            </div>
          </div>

          <div>
            <div className="text-xs text-gray-400">Time</div>
            <div className="text-sm text-white">{formatTimestamp(r.TimeStamp)}</div>
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
            {transactions.length} transaction{transactions.length === 1 ? '' : 's'} loaded
          </div>
          <div className="flex items-center space-x-4">
            <DebouncedInput
              value={filter}
              onChange={(v) => setFilter(String(v))}
              className="px-4 py-2 text-sm bg-[#1a1a1a] border border-[#3d3d3d] rounded-lg focus:outline-none focus:border-[#ffa729] text-white w-full md:w-auto"
              placeholder="Search transactions..."
            />
            <DownloadBtn data={transactions} />
          </div>
        </div>
      </div>

      <div className="overflow-x-auto">
        {isMobile ? (
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
