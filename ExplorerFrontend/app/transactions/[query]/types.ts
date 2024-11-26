import { SVGProps } from 'react';

export interface Transaction {
  TxHash: string;
  TimeStamp: number;
  InOut: number;
  Amount: number | string;
  Type?: string;
}

export interface TransactionsResponse {
  txs: Transaction[];
  total: number;
}

export interface TransactionsListProps {
  initialData: {
    txs: Transaction[];
    total: number;
  };
  currentPage: number;
}

export interface TransactionCardProps {
  transaction: Transaction;
}

export type SVGIconProps = SVGProps<SVGSVGElement>;

export interface PaginationProps {
  currentPage: number;
  totalPages: number;
  onNextPage: () => void;
  onPreviousPage: () => void;
}

export interface NavigationHandlers {
  navigateToPage: (page: number) => void;
  goToNextPage: () => void;
  goToPreviousPage: () => void;
}

export interface PageProps {
  params: { query: string };
  searchParams: { [key: string]: string | string[] | undefined };
}
