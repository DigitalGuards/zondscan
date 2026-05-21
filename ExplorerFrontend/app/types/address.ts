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
  /** ERC-20 / ERC-721 / ERC-1155 / empty for unclassified. Drives the
   *  "Token Contract" / "NFT Collection" / "Multi-Token Collection"
   *  header in address-view + token-contract-view. */
  tokenStandard?: 'ERC-20' | 'ERC-721' | 'ERC-1155' | string;
  hasERC165?: boolean;
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

  // M6a — populated only after a user has triggered the on-demand AI
  // explanation. Frontend treats absence as "not generated yet"; the
  // AiExplainCard renders a button instead of the cached body.
  aiExplanation?: string;
  aiExplanationAt?: string;
  aiExplanationModel?: string;
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
