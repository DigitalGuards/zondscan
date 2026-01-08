import type { SVGProps } from 'react';

/**
 * Transaction type enumeration
 */
export enum TransactionType {
  Coinbase = 0,
  Attest = 1,
  Transfer = 2,
  Stake = 3
}

/**
 * Base transaction fields that are always present
 */
interface BaseTransaction {
  InOut: number;
  TxType: TransactionType;
  TxHash: string;
  TimeStamp: number;
  Amount: number | string;
  PaidFees?: number;
  gasUsed?: string;
  gasPrice?: string;
  gasUsedStr?: string;
  gasPriceStr?: string;
}

/**
 * Additional fields that might be present on a transaction
 */
interface OptionalTransactionFields {
  ID?: string;
  id?: string;
  Address?: string;
  TransactionAddress?: string;
  From?: string;
  To?: string;
  Type?: string;
  [key: string]: string | number | undefined;
}

/**
 * Full transaction type combining base and optional fields
 */
export type Transaction = BaseTransaction & OptionalTransactionFields;

/**
 * Base internal transaction fields
 */
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

/**
 * Additional fields for internal transactions
 */
interface OptionalInternalTransactionFields {
  TraceAddress: Array<number | string>;
  [key: string]: string | number | Array<number | string> | undefined;
}

/**
 * Full internal transaction type
 */
export type InternalTransaction = BaseInternalTransaction & OptionalInternalTransactionFields;

/**
 * Token transfer information for a transaction
 */
export interface TokenTransferInfo {
  contractAddress: string;
  from: string;
  to: string;
  amount: string;
  tokenName: string;
  tokenSymbol: string;
  tokenDecimals: number;
}

/**
 * Detailed transaction information for transaction detail pages
 */
export interface TransactionDetails {
  hash: string;
  blockNumber?: string | number;
  from: string;
  to: string;
  value: string;
  timestamp: number;
  status?: string;
  gasUsed?: string;
  gasPrice?: string;
  nonce?: number;
  latestBlock?: number;
  PaidFees?: number;
  contractCreated?: {
    address: string;
    isToken: boolean;
    name: string;
    symbol: string;
    decimals: number;
  };
  tokenTransfer?: TokenTransferInfo;
}

/**
 * Pending transaction from the mempool
 */
export interface PendingTransaction {
  accessList: unknown[];
  blockHash: null;
  blockNumber?: string;
  chainId: string;
  from: string;
  gas: string;
  gasPrice: string;
  hash: string;
  input: string;
  maxFeePerGas?: string;
  maxPriorityFeePerGas?: string;
  nonce: string;
  publicKey: string;
  to?: string;
  transactionIndex: null;
  type: string;
  value: string;
  status: 'pending' | 'mined' | 'dropped';
  lastSeen: number;
  createdAt: number;
}

/**
 * Calculate the number of confirmations for a transaction
 */
export function getConfirmations(txBlockNumber?: string | number, latestBlock?: number): number | null {
  if (!txBlockNumber || !latestBlock) return null;
  const blockNum = typeof txBlockNumber === 'string' ? parseInt(txBlockNumber) : txBlockNumber;
  return latestBlock - blockNum + 1;
}

/**
 * Get transaction status based on confirmations
 */
export function getTransactionStatus(confirmations?: number | null): {
  text: string;
  color: string;
} {
  if (confirmations === null) {
    return { text: 'Pending', color: 'bg-yellow-500' };
  }

  if (confirmations && confirmations > 0) {
    if (confirmations >= 1) {
      return { text: 'Confirmed', color: 'bg-green-500' };
    }
    return { text: 'Processing', color: 'bg-blue-500' };
  }

  return { text: 'Pending', color: 'bg-yellow-500' };
}

/**
 * SVG icon props type
 */
export type SVGIconProps = SVGProps<SVGSVGElement>;
