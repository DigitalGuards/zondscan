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
}

// Helper function to calculate confirmations
export function getConfirmations(txBlockNumber?: string | number, latestBlock?: number): number | null {
  if (!txBlockNumber || !latestBlock) return null;
  const blockNum = typeof txBlockNumber === 'string' ? parseInt(txBlockNumber) : txBlockNumber;
  return latestBlock - blockNum + 1;
}

// Helper function to get transaction status
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
