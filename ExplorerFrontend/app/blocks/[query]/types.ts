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

export interface BlocksResponse {
  blocks: Block[];
  total: number;
}

export interface PageProps {
  params: { query: string };
  searchParams: { [key: string]: string | string[] | undefined };
}
