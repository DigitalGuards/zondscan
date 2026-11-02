import 'server-only';
import { isRecord } from '@/app/lib/guards';
import { canonicalizeQrlAddress } from '@/app/lib/qrlAddress';
import { normalizeQnsName } from '@/app/lib/qns';

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
  | { kind: 'unavailable' }
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

// A 404 is a definitive "no match"; any other failure means the check
// could not run, so the page must not claim the hash matched nothing.
type JsonResult = { payload: unknown } | { payload: null; failed: boolean };

async function getJson(
  fetcher: FallbackFetch,
  url: string,
  init: RequestInit
): Promise<JsonResult> {
  try {
    const response = await fetcher(url, init);
    if (response.status === 404) return { payload: null, failed: false };
    if (!response.ok || response.status !== 200) return { payload: null, failed: true };
    return { payload: await response.json() };
  } catch {
    return { payload: null, failed: true };
  }
}

function checkFailed(result: JsonResult): boolean {
  return 'failed' in result && result.failed;
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

  const block = await getJson(fetcher, `${base}/block/${normalized}`, {
    cache: 'no-store',
  });
  const blockNumber = blockNumberFor(block.payload, normalized);
  if (blockNumber !== null) return { kind: 'block', number: blockNumber };

  // No internal token here: every unknown hash costs a node call, so these
  // lookups stay inside the route's per-IP limit.
  const qns = await getJson(fetcher, `${base}/qns/node/${normalized}`, {
    cache: 'no-store',
  });
  const record = qnsRecordFor(qns.payload, normalized);
  if (record) return { kind: 'qns', record };

  if (checkFailed(block) || checkFailed(qns)) return { kind: 'unavailable' };
  return { kind: 'none' };
}
