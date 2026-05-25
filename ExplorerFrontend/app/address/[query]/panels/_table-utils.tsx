'use client';

import { useEffect, useState } from 'react';
import { flexRender } from '@tanstack/react-table';
import type { Cell, Header, HeaderGroup, Row, Table } from '@tanstack/react-table';

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
    <tr key={headerGroup.id} className="border-b border-[#3d3d3d]">
      {headerGroup.headers.map((header: Header<T, unknown>) => (
        <th
          key={header.id}
          scope="col"
          className="px-4 py-3 text-left text-sm font-medium text-[#ffa729]"
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
      className="border-b border-[#3d3d3d] hover:bg-[rgba(255,167,41,0.05)]"
    >
      {row.getVisibleCells().map((cell: Cell<T, unknown>) => (
        <td key={cell.id} className="px-4 py-3 text-sm text-gray-300">
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
    <div className="flex flex-col md:flex-row items-center justify-between gap-2 p-4 border-t border-[#3d3d3d]">
      <div className="flex flex-wrap items-center gap-2">
        <button
          aria-label="Go to first page"
          onClick={goFirst}
          disabled={!canPrev}
          className="px-3 py-1.5 text-sm text-gray-400 hover:text-white disabled:text-gray-600"
        >
          {'<<'}
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
          {'>>'}
        </button>
      </div>
      <div className="text-sm text-gray-400">
        Page {pageIndex + 1} of {Math.max(1, pageCount)}
      </div>
    </div>
  );
};

/**
 * Mobile / desktop switch for panels that render a card list under the
 * 768px breakpoint and a `<table>` above it. Lifted out of the old
 * TanStackTable so every panel can use the same threshold instead of
 * redefining the listener.
 */
export const useIsMobile = (breakpointPx = 768): boolean => {
  const [isMobile, setIsMobile] = useState(false);
  useEffect(() => {
    const check = (): void => setIsMobile(window.innerWidth < breakpointPx);
    check();
    window.addEventListener('resize', check);
    return () => window.removeEventListener('resize', check);
  }, [breakpointPx]);
  return isMobile;
};
