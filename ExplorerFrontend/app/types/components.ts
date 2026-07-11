import type { Transaction, InternalTransaction } from './transaction';

/**
 * Props for download button with transactions
 */
export interface DownloadBtnProps {
  data: Transaction[];
  fileName?: string;
  /** When set, the button resolves the full row set through this instead
   *  of exporting `data` (which may be just the currently loaded page). */
  getData?: () => Promise<Transaction[]>;
}

/**
 * Props for download button with internal transactions
 */
export interface DownloadBtnInternalProps {
  data: InternalTransaction[];
  fileName?: string;
  /** Same contract as DownloadBtnProps.getData. */
  getData?: () => Promise<InternalTransaction[]>;
}

/**
 * Props for transactions list component
 */
export interface TransactionsListProps {
  initialData: {
    txs: Transaction[];
    total: number;
  };
  currentPage: number;
}

/**
 * Props for transaction card component
 */
export interface TransactionCardProps {
  transaction: Transaction;
}

/**
 * Props for pagination component
 */
export interface PaginationProps {
  currentPage: number;
  totalPages: number;
  onNextPage: () => void;
  onPreviousPage: () => void;
}

/**
 * Common page props with params and search params
 */
export interface PageProps {
  params: { query: string };
  searchParams: { [key: string]: string | string[] | undefined };
}
