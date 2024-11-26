import type { Transaction, InternalTransaction } from '../../components/types';

export interface AddressData {
  address: {
    balance: number;
  };
  rank: number;
  transactions_by_address: Transaction[];
  internal_transactions_by_address: InternalTransaction[];
  response: unknown;
}

export interface BalanceDisplayProps {
  balance: number;
}

export interface ActivityDisplayProps {
  firstSeen: number;
  lastSeen: number;
}
