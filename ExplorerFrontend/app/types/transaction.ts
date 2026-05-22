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
 * Token standard tag persisted on contract + transfer rows. Empty for
 * legacy un-classified rows; new rows are always one of the three.
 */
export type TokenStandard = 'ERC-20' | 'ERC-721' | 'ERC-1155';

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
  /** ERC-20 | ERC-721 | ERC-1155, drives the row's badge in transaction-view. */
  tokenStandard?: TokenStandard | string;
  /** uint256 decimal string. Populated for ERC-721 + ERC-1155 transfers. */
  tokenID?: string;
  /** Hex log index, used as a stable React key when a tx emits several events. */
  logIndex?: string;
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
    tokenStandard?: TokenStandard | string;
  };
  tokenTransfers?: TokenTransferInfo[];
  /** Raw calldata for the tx, "0x..." prefixed. Empty / "0x" for plain transfers. */
  input?: string;
  /** Receipt logs in emission order. Populated best-effort; absent when the RPC fetch fails. */
  logs?: TxLog[];
}

/**
 * One receipt log entry on a confirmed tx. Mirrors the subset returned by
 * the backend's /tx/:hash route, which itself mirrors qrl_getTransactionReceipt.logs[].
 */
export interface TxLog {
  address: string;
  topics: string[];
  data: string;
  logIndex: string;
  removed?: boolean;
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
 * Calculate the number of confirmations for a transaction. Clamped to >=0:
 * if the backend's latestBlock is briefly stale relative to the tx's own
 * block, we'd otherwise return a negative count and badge the (mined) tx
 * as "Pending" with "-N Confirmations", confusing UX.
 */
export function getConfirmations(txBlockNumber?: string | number, latestBlock?: number): number | null {
  if (!txBlockNumber || !latestBlock) return null;
  const blockNum = typeof txBlockNumber === 'string' ? parseInt(txBlockNumber) : txBlockNumber;
  return Math.max(0, latestBlock - blockNum + 1);
}

/**
 * Get transaction status based on confirmations. This helper is called from
 * the mined-tx view (the /pending/tx redirect filters pending-status rows
 * upstream), so any non-null confirmations means the tx is in a block →
 * Confirmed. Only the missing-block-data case (null) falls back to Pending.
 */
export function getTransactionStatus(confirmations?: number | null): {
  text: string;
  color: string;
} {
  if (confirmations === null || confirmations === undefined) {
    return { text: 'Pending', color: 'bg-yellow-500' };
  }
  return { text: 'Confirmed', color: 'bg-green-500' };
}

/**
 * SVG icon props type
 */
export type SVGIconProps = SVGProps<SVGSVGElement>;
