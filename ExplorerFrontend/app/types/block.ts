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
 * Per-block aggregated activity counts. Keyed by the block number string
 * the backend emits (typically the hex form, e.g. "0x1231b"). Surfaced on
 * the /blocks list so users can see at a glance which blocks were busy
 * without paging into each one. Absent block numbers had zero of both.
 */
export interface BlockActivity {
  tokenTransfers: number;
  internalCalls: number;
}

/**
 * Response containing multiple blocks
 */
export interface BlocksResponse {
  blocks: Block[];
  total: number;
  blockActivity?: Record<string, BlockActivity>;
}
