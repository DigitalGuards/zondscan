import type { Transaction, InternalTransaction } from '../../components/types';

interface ContractCode {
  contractCreatorAddress: string | null;
  contractAddress: string | null;
  contractCode: string | null;
  decodedCreatorAddress?: string;
  decodedContractAddress?: string;
  contractSize?: number;
  // Token information
  tokenName?: string;
  tokenSymbol?: string;
  tokenDecimals?: number;
  isToken?: boolean;
}

export interface AddressData {
  address: {
    balance: number;
  };
  rank: number;
  transactions_by_address: Transaction[];
  internal_transactions_by_address: InternalTransaction[];
  contract_code: ContractCode;
  response: unknown;
}

export interface BalanceDisplayProps {
  balance: number;
}

export interface ActivityDisplayProps {
  firstSeen: number;
  lastSeen: number;
}
