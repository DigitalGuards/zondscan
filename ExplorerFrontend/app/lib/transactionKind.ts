import type { TransactionDetails } from '../types';

export type TransactionKind = 'Transfer' | 'Contract call' | 'Contract creation' | 'Transaction';

/** Describe the transaction independently of its execution status or the viewer's address. */
export function getTransactionKind(
  transaction: Pick<
    TransactionDetails,
    | 'to'
    | 'input'
    | 'targetContract'
    | 'contractCreated'
    | 'tokenTransfers'
    | 'internalTransactions'
    | 'receiptStatus'
  >
): TransactionKind {
  if (transaction.to === '') return 'Contract creation';
  if (
    transaction.contractCreated?.address ||
    transaction.targetContract ||
    transaction.tokenTransfers?.length ||
    transaction.internalTransactions?.length
  ) {
    return 'Contract call';
  }
  // Empty calldata indicates a value transfer, which can also target a contract.
  // Missing calldata or arbitrary data alone cannot establish a contract call.
  if (transaction.to && (transaction.input === '0x' || transaction.input === '')) return 'Transfer';
  return 'Transaction';
}
