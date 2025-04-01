import type { Transaction, InternalTransaction } from '../../components/types';

interface ContractData {
  creatorAddress: string;
  address: string;
  contractCode: string;
  creationTransaction: string;
  isToken: boolean;
  status: string;
  decimals: number;
  name: string;
  symbol: string;
  updatedAt: string;
}

export interface AddressData {
  address: {
    balance: number;
  };
  rank: number;
  transactions_by_address: Transaction[];
  internal_transactions_by_address: InternalTransaction[];
  contract_code: ContractData | null;
  response: unknown;
}

export interface BalanceDisplayProps {
  balance: number;
}

export interface ActivityDisplayProps {
  firstSeen: number;
  lastSeen: number;
}
