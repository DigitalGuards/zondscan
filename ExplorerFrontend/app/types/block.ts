/**
 * Block data structure
 */
export interface Block {
  number: number;
  timestamp: number;
  hash: string;
  parentHash: string;
  gasLimit: string;
  gasUsed: string;
  miner: string;
  extraData: string;
  transactions: string[];
  prevRandao: string;
}

/**
 * Response containing multiple blocks
 */
export interface BlocksResponse {
  blocks: Block[];
  total: number;
}
