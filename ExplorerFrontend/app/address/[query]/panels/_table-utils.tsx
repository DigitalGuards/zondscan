'use client';

import { flexRender } from '@tanstack/react-table';
import type { Cell, Header, HeaderGroup, Row, Table } from '@tanstack/react-table';

// Re-exported from the shared hooks module so existing panel imports
// (`useIsMobile` from this file) keep working after consolidation.
export { useIsMobile } from '../../../lib/hooks';

/**
 * Shared TanStack table primitives for the address-page tab panels.
 *
 * The address page renders five tabs (Transactions, Internal Txns, Token
 * Transfers, Tokens, NFTs), each backed by its own TanStack table. Without
 * this module every panel would re-implement the same header/body row
 * walking and pagination controls. Keep this strictly presentational, no
 * data-fetching logic, no panel-specific column definitions.
 */

// Page size used by the Tokens / NFTs panels. Transactions / Internal /
// Token Transfers use TanStack's default 10/page (denser data per row).
export const PAGE_SIZE = 5;

/** Middle-truncate long hex strings (addresses, tx hashes) for table cells. */
export const truncateMiddle = (str: string, startChars = 8, endChars = 8): string => {
  if (str.length <= startChars + endChars) return str;
  return `${str.slice(0, startChars)}...${str.slice(-endChars)}`;
};

/**
 * Generic over the row shape, the walkers only touch table-internal
 * structures (header / cell descriptors), they don't read row fields.
 * Same markup the old TanStackTable.tsx used so the visual result is
 * indistinguishable after the refactor.
 */
export const renderTableHeader = <T,>(table: Table<T>): JSX.Element[] => {
  return table.getHeaderGroups().map((headerGroup: HeaderGroup<T>) => (
    <tr key={headerGroup.id} className="border-b border-border">
      {headerGroup.headers.map((header: Header<T, unknown>) => (
        <th
          key={header.id}
          scope="col"
          className="px-4 py-3 text-left text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted whitespace-nowrap"
        >
          {header.isPlaceholder
            ? null
            : flexRender(header.column.columnDef.header, header.getContext())}
        </th>
      ))}
    </tr>
  ));
};

export const renderTableBody = <T,>(table: Table<T>): JSX.Element[] => {
  return table.getRowModel().rows.map((row: Row<T>) => (
    <tr
      key={row.id}
      className="border-b border-border last:border-b-0 hover:bg-surface transition-colors"
    >
      {row.getVisibleCells().map((cell: Cell<T, unknown>) => (
        <td key={cell.id} className="px-4 py-3 text-sm text-text-secondary">
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

/** First / Prev / Next / Last + "Page X of Y". Identical to the controls
 *  the pre-refactor TanStackTable and HoldingsDisplay rendered. */
export const Paginator = ({
  pageIndex,
  pageCount,
  canPrev,
  canNext,
  goFirst,
  goPrev,
  goNext,
  goLast,
}: PaginatorProps): JSX.Element => {
  return (
    <div className="flex flex-col md:flex-row items-center justify-between gap-2 p-4 border-t border-border">
      <div className="flex flex-wrap items-center gap-2">
        <button
          aria-label="Go to first page"
          onClick={goFirst}
          disabled={!canPrev}
          className="px-3 py-1.5 text-sm rounded-lg bg-surface-2 border border-border text-text-secondary hover:text-text-primary hover:border-accent/40 disabled:opacity-40 disabled:hover:border-border transition-colors"
        >
          {'<<'}
        </button>
        <button
          aria-label="Go to previous page"
          onClick={goPrev}
          disabled={!canPrev}
          className="px-3 py-1.5 text-sm rounded-lg bg-surface-2 border border-border text-text-secondary hover:text-text-primary hover:border-accent/40 disabled:opacity-40 disabled:hover:border-border transition-colors"
        >
          Previous
        </button>
        <button
          aria-label="Go to next page"
          onClick={goNext}
          disabled={!canNext}
          className="px-3 py-1.5 text-sm rounded-lg bg-surface-2 border border-border text-text-secondary hover:text-text-primary hover:border-accent/40 disabled:opacity-40 disabled:hover:border-border transition-colors"
        >
          Next
        </button>
        <button
          aria-label="Go to last page"
          onClick={goLast}
          disabled={!canNext}
          className="px-3 py-1.5 text-sm rounded-lg bg-surface-2 border border-border text-text-secondary hover:text-text-primary hover:border-accent/40 disabled:opacity-40 disabled:hover:border-border transition-colors"
        >
          {'>>'}
        </button>
      </div>
      <div className="text-sm text-text-secondary">
        Page {pageIndex + 1} of {Math.max(1, pageCount)}
      </div>
    </div>
  );
};

