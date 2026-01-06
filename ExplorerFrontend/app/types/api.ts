import type { Transaction, PendingTransaction } from './transaction';

/**
 * Response for transactions list API
 */
export interface TransactionsResponse {
  txs: Transaction[];
  total: number;
}

/**
 * Response for pending transaction API
 */
export interface PendingTransactionResponse {
  transaction: PendingTransaction;
}

/**
 * Pending transactions grouped by nonce
 */
export interface PendingTransactionsByNonce {
  [nonce: string]: PendingTransaction;
}

/**
 * Pending transactions grouped by address
 */
export interface PendingTransactionsByAddress {
  [address: string]: PendingTransactionsByNonce;
}

/**
 * Response for pending transactions list API
 */
export interface PendingTransactionsResponse {
  pending: PendingTransactionsByAddress;
  queued: Record<string, unknown>;
}
