import 'server-only';
import { isRecord } from '@/app/lib/guards';
import { canonicalizeQrlAddress } from '@/app/lib/qrlAddress';
import { normalizeQnsName } from '@/app/lib/qns';
import { qnsRequestHeaders } from '@/app/lib/qnsRequest';

/**
 * A 0x + 64 hex search can be a transaction hash, a block hash, or a QNS
 * name hash (namehash node). When no transaction matches, the tx page asks
 * the backend about the other two shapes, in that order.
 */

export interface QnsNodeRecord {
  node: string;
  owner: string;
  resolver: string | null;
  name: string | null;
}

export type HashFallback =
  | { kind: 'block'; number: number }
  | { kind: 'qns'; record: QnsNodeRecord }
  | { kind: 'none' };

interface FallbackResponse {
  ok: boolean;
  status: number;
  json(): Promise<unknown>;
}

export type FallbackFetch = (input: string, init?: RequestInit) => Promise<FallbackResponse>;

function defaultFetch(input: string, init?: RequestInit): Promise<FallbackResponse> {
  return fetch(input, init);
}

async function getJson(
  fetcher: FallbackFetch,
  url: string,
  init: RequestInit
): Promise<unknown> {
  try {
    const response = await fetcher(url, init);
    if (!response.ok || response.status !== 200) return null;
    return await response.json();
  } catch {
    return null;
  }
}

function blockNumberFor(payload: unknown, hash: string): number | null {
  if (!isRecord(payload) || !isRecord(payload.block)) return null;
  const result = payload.block.result;
  if (!isRecord(result)) return null;
  if (typeof result.hash !== 'string' || result.hash.toLowerCase() !== hash) return null;
  if (typeof result.number !== 'string' || !/^0x[0-9a-fA-F]{1,16}$/.test(result.number)) {
    return null;
  }
  const number = Number.parseInt(result.number, 16);
  return Number.isSafeInteger(number) ? number : null;
}

function optionalAddress(value: unknown): string | null | undefined {
  if (value === null) return null;
  if (typeof value !== 'string') return undefined;
  return canonicalizeQrlAddress(value) === value ? value : undefined;
}

function qnsRecordFor(payload: unknown, hash: string): QnsNodeRecord | null {
  if (!isRecord(payload) || payload.node !== hash) return null;
  const owner = optionalAddress(payload.owner);
  const resolver = optionalAddress(payload.resolver);
  if (!owner || resolver === undefined) return null;
  let name: string | null = null;
  if (payload.name !== null) {
    if (typeof payload.name !== 'string' || normalizeQnsName(payload.name) !== payload.name) {
      return null;
    }
    name = payload.name;
  }
  return { node: hash, owner, resolver, name };
}

export async function resolveUnknownHash(
  hash: string,
  handlerUrl: string | undefined,
  fetcher: FallbackFetch = defaultFetch
): Promise<HashFallback> {
  const base = handlerUrl?.trim().replace(/\/+$/, '');
  if (!base) return { kind: 'none' };
  const normalized = hash.toLowerCase();

  const blockPayload = await getJson(fetcher, `${base}/block/${normalized}`, {
    cache: 'no-store',
  });
  const blockNumber = blockNumberFor(blockPayload, normalized);
  if (blockNumber !== null) return { kind: 'block', number: blockNumber };

  const qnsPayload = await getJson(fetcher, `${base}/qns/node/${normalized}`, {
    cache: 'no-store',
    ...qnsRequestHeaders(),
  });
  const record = qnsRecordFor(qnsPayload, normalized);
  if (record) return { kind: 'qns', record };

  return { kind: 'none' };
}
