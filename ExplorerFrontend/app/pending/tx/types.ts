export interface PendingTransaction {
  accessList: any[];
  blockHash: null;
  blockNumber: null;
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
}

export interface PendingTransactionsByNonce {
  [nonce: string]: PendingTransaction;
}

export interface PendingTransactionsByAddress {
  [address: string]: PendingTransactionsByNonce;
}

export interface PendingTransactionsResponse {
  pending: PendingTransactionsByAddress;
  queued: Record<string, unknown>;
}
