import type { Transaction, InternalTransaction } from '../../components/types';

interface ContractData {
  contractCreatorAddress: string;
  contractAddress: string;
  contractCode: string;
  creationTransaction: string;
  isToken: boolean;
  status: string;
  tokenDecimals: number;
  tokenName: string;
  tokenSymbol: string;
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
