import type { Transaction, InternalTransaction } from './transaction';

/**
 * Props for download button with transactions
 */
export interface DownloadBtnProps {
  data: Transaction[];
  fileName?: string;
}

/**
 * Props for download button with internal transactions
 */
export interface DownloadBtnInternalProps {
  data: InternalTransaction[];
  fileName?: string;
}

/**
 * Props for table component
 */
export interface TableProps {
  transactions: Transaction[];
  internalt: InternalTransaction[];
}

/**
 * Data structure for table component
 */
export interface TableData {
  transactions: Transaction[];
  internalTransactions: InternalTransaction[];
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
 * Navigation handler functions
 */
export interface NavigationHandlers {
  navigateToPage: (page: number) => void;
  goToNextPage: () => void;
  goToPreviousPage: () => void;
}

/**
 * Common page props with params and search params
 */
export interface PageProps {
  params: { query: string };
  searchParams: { [key: string]: string | string[] | undefined };
}
