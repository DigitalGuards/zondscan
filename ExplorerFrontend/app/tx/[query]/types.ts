export interface TransactionDetails {
  hash: string;
  blockNumber: string;
  from: string;
  to: string;
  value: string;
  timestamp: number;
  status: string;
  gasUsed?: string;
  gasPrice?: string;
  nonce?: string;
}

export interface TransactionResponse {
  transaction: TransactionDetails;
  error?: string;
}
