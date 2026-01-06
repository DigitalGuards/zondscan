import type { Transaction, InternalTransaction } from './transaction';

/**
 * Contract data associated with an address
 */
export interface ContractData {
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

/**
 * Full address data including transactions and contract info
 */
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

/**
 * Props for balance display component
 */
export interface BalanceDisplayProps {
  balance: number;
}

/**
 * Props for activity display component
 */
export interface ActivityDisplayProps {
  firstSeen: number;
  lastSeen: number;
}
