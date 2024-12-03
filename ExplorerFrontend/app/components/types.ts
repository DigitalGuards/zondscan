export enum TransactionType {
  Coinbase = 0,
  Attest = 1,
  Transfer = 2,
  Stake = 3
}

// Base transaction fields that are always present
interface BaseTransaction {
  InOut: number;
  TxType: TransactionType;
  TxHash: string;
  TimeStamp: number;
  Amount: number;
  PaidFees?: number;  // Added to base fields since it's a core fee property
  gasUsed?: string;
  gasPrice?: string;
  gasUsedStr?: string;
  gasPriceStr?: string;
}

// Additional fields that might be present
interface OptionalTransactionFields {
  ID?: string;
  id?: string;
  Address?: string;
  TransactionAddress?: string;
  From?: string;
  To?: string;
  Type?: string;
  [key: string]: string | number | undefined; // Index signature for dynamic access
}

// Combine base and optional fields
export type Transaction = BaseTransaction & OptionalTransactionFields;

// Base internal transaction fields that are always present
interface BaseInternalTransaction {
  Type: number;
  CallType: string;
  Hash: string;
  From: string;
  To: string;
  Input: string;
  Output: number;
  Value: number;
  Gas: number;
  GasUsed: number;
  AddressFunctionIdentifier: string;
  AmountFunctionIdentifier: number;
  BlockTimestamp: number;
}

// Additional fields that might be present in internal transactions
interface OptionalInternalTransactionFields {
  TraceAddress: Array<number | string>;
  [key: string]: string | number | Array<number | string> | undefined; // Index signature for dynamic access
}

// Combine base and optional fields for internal transactions
export type InternalTransaction = BaseInternalTransaction & OptionalInternalTransactionFields;

export interface DownloadBtnProps {
  data: Transaction[];
  fileName?: string;
}

export interface DownloadBtnInternalProps {
  data: InternalTransaction[];
  fileName?: string;
}

export interface TableProps {
  transactions: Transaction[];
  internalt: InternalTransaction[];
}

export interface TableData {
  transactions: Transaction[];
  internalTransactions: InternalTransaction[];
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
