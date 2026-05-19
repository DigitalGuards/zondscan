import type { Transaction, InternalTransaction } from './transaction';

/**
 * Contract data associated with an address.
 *
 * Verification fields below are written exclusively by the backend verify
 * endpoint (M2). Until a contract is verified, `verified` is `false` and
 * the optional source/abi/compiler fields are absent — matching the Go
 * struct's `omitempty` bson tags.
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

  // Source verification (added by M1, populated by M2+)
  verified: boolean;
  sourceCode?: string;
  abi?: string;
  contractName?: string;
  compilerVersion?: string;
  optimizationEnabled?: boolean;
  optimizationRuns?: number;
  evmVersion?: string;
  constructorArguments?: string;
  libraries?: Record<string, string>;
  license?: string;
  verificationMethod?: string;
  verifiedAt?: string;
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
