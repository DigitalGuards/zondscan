/**
 * Block data structure
 */
export interface Block {
  number: number;
  timestamp: number;
  hash: string;
  parentHash: string;
  nonce: string;
  difficulty: string;
  gasLimit: string;
  gasUsed: string;
  miner: string;
  extraData: string;
  transactions: string[];
}

/**
 * Response containing multiple blocks
 */
export interface BlocksResponse {
  blocks: Block[];
  total: number;
}
