import type { EpochInfo } from '../types';

export interface BlockResult {
  number: string;
  timestamp: string;
  hash: string;
  miner: string;
  transactions: unknown[];
}

export interface TxResult {
  TxHash: string;
  TimeStamp: string | number;
  From: string;
  To: string;
  Amount: string | number;
  BlockNumber?: string;
  TxType?: string;
}

export type HomePart =
  | 'overview'
  | 'blockHeight'
  | 'totalTransactions'
  | 'epochInfo'
  | 'blocks'
  | 'totalStaked'
  | 'avgGasPriceHex'
  | 'txs';
export type HomeStatus = 'loading' | 'ready' | 'error';

export interface HomeData {
  blockHeight: number | null;
  totalTransactions: number | null;
  validatorCount: number | null;
  epochInfo: EpochInfo | null;
  blocks: BlockResult[] | null;
  txs: TxResult[] | null;
  totalStaked: string | null;
  marketCap: number | null;
  circulating: string | null;
  avgGasPriceHex: string | null;
  dataInitialized: boolean | null;
  status: Record<HomePart, HomeStatus>;
}

export function initialHomeData(): HomeData {
  return {
    blockHeight: null,
    totalTransactions: null,
    validatorCount: null,
    epochInfo: null,
    blocks: null,
    txs: null,
    totalStaked: null,
    marketCap: null,
    circulating: null,
    avgGasPriceHex: null,
    dataInitialized: null,
    status: {
      overview: 'loading',
      blockHeight: 'loading',
      totalTransactions: 'loading',
      epochInfo: 'loading',
      blocks: 'loading',
      totalStaked: 'loading',
      avgGasPriceHex: 'loading',
      txs: 'loading',
    },
  };
}

function count(value: unknown): number | null {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0 ? value : null;
}

/** Publish each response immediately while the poll still owns the full snapshot. */
export async function loadHomeData(
  request: (path: string, signal: AbortSignal) => Promise<Record<string, unknown>>,
  signal: AbortSignal,
  publish: (update: (previous: HomeData) => HomeData) => void
): Promise<void> {
  async function part(
    key: HomePart,
    path: string,
    select: (body: Record<string, unknown>) => Partial<HomeData>
  ) {
    try {
      const fields = select(await request(path, signal));
      if (!signal.aborted)
        publish((previous) => ({
          ...previous,
          ...fields,
          status: { ...previous.status, [key]: 'ready' },
        }));
    } catch {
      // Keep the last successful snapshot during a failed background refresh.
      if (!signal.aborted)
        publish((previous) => ({ ...previous, status: { ...previous.status, [key]: 'error' } }));
    }
  }

  await Promise.all([
    part('overview', '/overview', (body) => ({
      validatorCount: count(body.validatorCount),
      marketCap:
        typeof body.marketcap === 'number' && Number.isFinite(body.marketcap)
          ? body.marketcap
          : null,
      circulating: typeof body.circulating === 'string' ? body.circulating : null,
      dataInitialized:
        typeof (body.status as { dataInitialized?: unknown } | undefined)?.dataInitialized ===
        'boolean'
          ? (body.status as { dataInitialized: boolean }).dataInitialized
          : null,
    })),
    part('blockHeight', '/latestblock', (body) => {
      const blockHeight = count(body.blockNumber);
      if (blockHeight === null) throw new Error('Block height unavailable');
      return { blockHeight };
    }),
    part('totalTransactions', '/txs?page=1', (body) => {
      const totalTransactions = count(body.total);
      if (totalTransactions === null) throw new Error('Transaction count unavailable');
      return { totalTransactions };
    }),
    part('epochInfo', '/epoch', (body) => {
      if (typeof body.headEpoch !== 'string') throw new Error('Epoch unavailable');
      return { epochInfo: body as unknown as EpochInfo };
    }),
    part('blocks', '/blocks?page=1&limit=10', (body) => {
      if (!Array.isArray(body.blocks)) throw new Error('Blocks unavailable');
      return { blocks: body.blocks as BlockResult[] };
    }),
    part('totalStaked', '/epochs?page=1&limit=1', (body) => {
      const totalStaked = (body.epochs as { totalStaked?: unknown }[] | undefined)?.[0]
        ?.totalStaked;
      return { totalStaked: typeof totalStaked === 'string' ? totalStaked : null };
    }),
    part('avgGasPriceHex', '/gas/summary', (body) => ({
      avgGasPriceHex: typeof body.avgGasPriceHex === 'string' ? body.avgGasPriceHex : null,
    })),
    part('txs', '/transactions', (body) => {
      if (!Array.isArray(body.response)) throw new Error('Transactions unavailable');
      const seen = new Set<string>();
      const txs = (body.response as TxResult[])
        .filter((tx) => {
          if (!tx?.TxHash || seen.has(tx.TxHash)) return false;
          seen.add(tx.TxHash);
          return true;
        })
        .slice(0, 8);
      return { txs };
    }),
  ]);
}
