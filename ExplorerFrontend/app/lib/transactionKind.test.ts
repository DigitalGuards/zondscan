import type { TransactionDetails } from '../types';
import { getTransactionKind } from './transactionKind';

const transaction: TransactionDetails = {
  hash: `0x${'1'.repeat(64)}`,
  from: `Q${'a'.repeat(128)}`,
  to: `Q${'b'.repeat(128)}`,
  value: '0x0',
  input: '0x',
  timestamp: 0,
};

describe('getTransactionKind', () => {
  it.each(['', '0x'])('labels explicit empty calldata %p as Transfer', (input) => {
    expect(getTransactionKind({ ...transaction, input })).toBe('Transfer');
  });

  it.each([undefined, '0x00', '0x12345678'])('keeps unclassified calldata %p neutral', (input) => {
    expect(getTransactionKind({ ...transaction, input })).toBe('Transaction');
  });

  it('recognizes a known contract even with empty calldata', () => {
    expect(getTransactionKind({ ...transaction, targetContract: {} })).toBe('Contract call');
  });

  it('uses token execution evidence even when zero-value events are hidden in the UI', () => {
    expect(
      getTransactionKind({
        ...transaction,
        tokenTransfers: [
          {
            contractAddress: transaction.to,
            from: transaction.from,
            to: transaction.to,
            amount: '0',
            tokenName: 'Example',
            tokenSymbol: 'EX',
            tokenDecimals: 18,
          },
        ],
      })
    ).toBe('Contract call');
  });

  it('uses internal execution evidence', () => {
    expect(
      getTransactionKind({
        ...transaction,
        internalTransactions: [
          {
            type: 'CALL',
            callType: 'CALL',
            from: transaction.from,
            to: transaction.to,
            input: '0x',
            output: '0x',
            value: 0,
            gas: '0x5208',
            gasUsed: '0x5208',
            traceAddress: [0],
          },
        ],
      })
    ).toBe('Contract call');
  });

  it('recognizes contract creation before its address is available', () => {
    expect(getTransactionKind({ ...transaction, to: '' })).toBe('Contract creation');
  });

  it('preserves a factory call when it also creates a child contract', () => {
    expect(
      getTransactionKind({
        ...transaction,
        targetContract: {},
        contractCreated: {
          address: `Q${'c'.repeat(128)}`,
          isToken: false,
          name: '',
          symbol: '',
          decimals: 0,
        },
      })
    ).toBe('Contract call');
  });

  it.each(['0x0', '0x1', undefined])(
    'keeps execution status %p separate from type',
    (receiptStatus) => {
      expect(getTransactionKind({ ...transaction, receiptStatus })).toBe('Transfer');
    }
  );
});
