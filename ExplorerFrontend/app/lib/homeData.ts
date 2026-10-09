import type { EpochInfo } from '../types';
import { isArray, isRecord, InvalidInputError } from './guards';

export function isEpochInfo(value: unknown): value is EpochInfo {
  return (
    isRecord(value) &&
    typeof value.headEpoch === 'string' &&
    typeof value.headSlot === 'string' &&
    typeof value.finalizedEpoch === 'string' &&
    typeof value.justifiedEpoch === 'string' &&
    typeof value.slotsPerEpoch === 'number' &&
    typeof value.secondsPerSlot === 'number' &&
    typeof value.slotInEpoch === 'number' &&
    typeof value.timeToNextEpoch === 'number' &&
    typeof value.updatedAt === 'number'
  );
}

function isBlockResult(value: unknown): value is BlockResult {
  return (
    isRecord(value) &&
    typeof value.number === 'string' &&
    typeof value.timestamp === 'string' &&
    typeof value.hash === 'string' &&
    typeof value.miner === 'string' &&
    isArray(value.transactions)
  );
}

function isTxResult(value: unknown): value is TxResult {
  return (
    isRecord(value) &&
    typeof value.TxHash === 'string' &&
    (typeof value.TimeStamp === 'string' || typeof value.TimeStamp === 'number') &&
    typeof value.From === 'string' &&
    typeof value.To === 'string' &&
    (typeof value.Amount === 'string' || typeof value.Amount === 'number') &&
    (value.BlockNumber === undefined || typeof value.BlockNumber === 'string') &&
    (value.TxType === undefined ||
      typeof value.TxType === 'string' ||
      typeof value.TxType === 'number')
  );
}

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
  TxType?: string | number;
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
  request: (path: string, signal: AbortSignal) => Promise<unknown>,
  signal: AbortSignal,
  publish: (update: (previous: HomeData) => HomeData) => void
): Promise<void> {
  async function part(
    key: HomePart,
    path: string,
    select: (body: Record<string, unknown>) => Partial<HomeData>
  ) {
    try {
      const body = await request(path, signal);
      if (!isRecord(body)) throw new InvalidInputError('Invalid homepage response');
      const fields = select(body);
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
        isRecord(body.status) && typeof body.status.dataInitialized === 'boolean'
          ? body.status.dataInitialized
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
      if (!isEpochInfo(body)) throw new InvalidInputError('Epoch unavailable');
      return { epochInfo: body };
    }),
    part('blocks', '/blocks?page=1&limit=10', (body) => {
      if (!isArray(body.blocks) || !body.blocks.every(isBlockResult))
        throw new InvalidInputError('Blocks unavailable');
      return { blocks: body.blocks };
    }),
    part('totalStaked', '/epochs?page=1&limit=1', (body) => {
      const epoch = isArray(body.epochs) ? body.epochs[0] : undefined;
      const totalStaked = isRecord(epoch) ? epoch.totalStaked : undefined;
      return { totalStaked: typeof totalStaked === 'string' ? totalStaked : null };
    }),
    part('avgGasPriceHex', '/gas/summary', (body) => ({
      avgGasPriceHex: typeof body.avgGasPriceHex === 'string' ? body.avgGasPriceHex : null,
    })),
    part('txs', '/transactions', (body) => {
      if (!isArray(body.response)) throw new InvalidInputError('Transactions unavailable');
      const seen = new Set<string>();
      const txs = body.response
        .filter(isTxResult)
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
