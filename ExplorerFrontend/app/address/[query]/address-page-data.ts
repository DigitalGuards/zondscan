import type { AddressData } from '@/app/types';
import { decodeToHex, formatAddress } from '../../lib/helpers';
import { canonicalizeQrlAddress } from '../../lib/qrlAddress';
import { hasQnsSuffix, normalizeQnsName } from '../../lib/qns';

interface ExplorerFetchResponse {
  ok: boolean;
  status: number;
  json(): Promise<unknown>;
}

type ExplorerRequestInit = RequestInit & {
  next?: { revalidate?: number };
};

export type ExplorerFetch = (
  input: string,
  init?: ExplorerRequestInit,
) => Promise<ExplorerFetchResponse>;

export type QnsResolutionErrorKind =
  | 'qns-invalid'
  | 'qns-unconfigured'
  | 'qns-missing'
  | 'qns-upstream';

export type QnsResolutionResult =
  | { ok: true; name: string; address: string }
  | {
      ok: false;
      kind: QnsResolutionErrorKind;
      name?: string;
      detail: string;
    };

export type AddressPageLoadResult =
  | {
      ok: true;
      address: string;
      qnsName?: string;
      addressData: AddressData;
    }
  | {
      ok: false;
      kind: QnsResolutionErrorKind | 'invalid-query' | 'address-data';
      name?: string;
      detail: string;
    };

const QNS_NOT_CONFIGURED = 'QNS resolution is not configured';
const QNS_CHAIN_MISMATCH = 'QNS deployment does not match the connected chain';
const QNS_REGISTRY_UNAVAILABLE = 'QNS registry is unavailable on the connected chain';
const QNS_NO_RESOLVER = 'QNS name has no resolver';
const QNS_NO_ADDRESS = 'QNS name has no address record';

function defaultFetch(input: string, init?: ExplorerRequestInit): Promise<ExplorerFetchResponse> {
  return fetch(input, init);
}

function handlerBase(handlerUrl: string | undefined): string | null {
  const base = handlerUrl?.trim().replace(/\/+$/, '');
  return base || null;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

async function readJson(response: ExplorerFetchResponse): Promise<unknown> {
  try {
    return await response.json();
  } catch {
    return null;
  }
}

function payloadError(payload: unknown): string | null {
  if (!isRecord(payload)) return null;
  return typeof payload.error === 'string' ? payload.error : null;
}

/**
 * Resolve one already-routable QNS input through the configured backend.
 * Success is accepted only when both returned fields match the public route
 * contract exactly.
 */
export async function resolveQnsName(
  input: string,
  handlerUrl: string | undefined,
  fetcher: ExplorerFetch = defaultFetch,
): Promise<QnsResolutionResult> {
  const name = normalizeQnsName(input);
  if (!name) {
    return {
      ok: false,
      kind: 'qns-invalid',
      detail: 'Invalid QNS name. Use the conservative ASCII .qrl name format.',
    };
  }

  const base = handlerBase(handlerUrl);
  if (!base) {
    return {
      ok: false,
      kind: 'qns-unconfigured',
      name,
      detail: QNS_NOT_CONFIGURED,
    };
  }

  let response: ExplorerFetchResponse;
  try {
    response = await fetcher(`${base}/qns/resolve/${encodeURIComponent(name)}`, {
      cache: 'no-store',
    });
  } catch {
    return {
      ok: false,
      kind: 'qns-upstream',
      name,
      detail: 'QNS resolution failed before the explorer received a response.',
    };
  }

  const payload = await readJson(response);
  if (response.ok && response.status === 200) {
    if (!isRecord(payload) || typeof payload.name !== 'string' || typeof payload.address !== 'string') {
      return {
        ok: false,
        kind: 'qns-upstream',
        name,
        detail: 'QNS resolution returned a malformed response.',
      };
    }

    const returnedName = normalizeQnsName(payload.name);
    const canonicalAddress = canonicalizeQrlAddress(payload.address);
    if (
      returnedName !== name ||
      payload.name !== name ||
      canonicalAddress === null ||
      payload.address !== canonicalAddress
    ) {
      return {
        ok: false,
        kind: 'qns-upstream',
        name,
        detail: 'QNS resolution returned an invalid name or address.',
      };
    }

    return { ok: true, name, address: canonicalAddress };
  }

  const error = payloadError(payload);
  if (response.status === 400) {
    return {
      ok: false,
      kind: 'qns-invalid',
      name,
      detail: 'The QNS service rejected this name as invalid.',
    };
  }
  if (response.status === 404) {
    return {
      ok: false,
      kind: 'qns-missing',
      name,
      detail:
        error === QNS_NO_RESOLVER || error === QNS_NO_ADDRESS
          ? error
          : 'QNS name has no resolver or address record',
    };
  }
  if (response.status === 503) {
    return {
      ok: false,
      kind: 'qns-unconfigured',
      name,
      detail:
        error === QNS_NOT_CONFIGURED ||
        error === QNS_CHAIN_MISMATCH ||
        error === QNS_REGISTRY_UNAVAILABLE
          ? error
          : 'QNS resolution is unavailable on this explorer deployment',
    };
  }

  let detail = 'QNS resolution failed upstream.';
  if (response.status === 429) {
    detail = 'Too many QNS lookups were requested. Try again shortly.';
  } else if (response.status === 504) {
    detail = 'QNS resolution timed out.';
  }
  return { ok: false, kind: 'qns-upstream', name, detail };
}

async function fetchAddressData(
  address: string,
  handlerUrl: string | undefined,
  fetcher: ExplorerFetch,
): Promise<AddressData | null> {
  const base = handlerBase(handlerUrl);
  if (!base) return null;

  try {
    const response = await fetcher(`${base}/address/aggregate/${address}`, {
      next: { revalidate: 10 },
    });
    if (!response.ok) return null;

    const payload = await response.json();
    if (!isRecord(payload)) return null;

    const transactions = payload.transactions_by_address;
    if (Array.isArray(transactions)) {
      payload.transactions_by_address = transactions.map((transaction) => {
        if (!isRecord(transaction)) return transaction;
        const gasUsed = transaction.gasUsed;
        const gasPrice = transaction.gasPrice;
        return {
          ...transaction,
          gasUsedStr:
            transaction.gasUsedStr ||
            (gasUsed
              ? `0x${typeof gasUsed === 'number' ? gasUsed.toString(16) : String(gasUsed)}`
              : '0x0'),
          gasPriceStr:
            transaction.gasPriceStr ||
            (gasPrice
              ? `0x${typeof gasPrice === 'number' ? gasPrice.toString(16) : String(gasPrice)}`
              : '0x0'),
        };
      });
    }

    const contractCode = payload.contract_code;
    if (
      isRecord(contractCode) &&
      typeof contractCode.contractCode === 'string' &&
      contractCode.contractCode
    ) {
      const creatorBytes = contractCode.contractCreatorAddress;
      const addressBytes = contractCode.contractAddress;
      const rawCreatorAddress =
        typeof creatorBytes === 'string' ? `0x${decodeToHex(creatorBytes)}` : '0x0';

      payload.contract_code = {
        ...contractCode,
        decodedCreatorAddress: formatAddress(rawCreatorAddress),
        decodedContractAddress:
          typeof addressBytes === 'string'
            ? formatAddress(`0x${decodeToHex(addressBytes)}`)
            : '0x0',
        contractSize: Math.ceil((contractCode.contractCode.length * 3) / 4),
      };
    }

    return payload as unknown as AddressData;
  } catch (error) {
    console.error('Error fetching address data:', error);
    return null;
  }
}

/**
 * Resolve a route query, then load its aggregate. QNS names cannot reach the
 * aggregate endpoint until the resolver returns an independently validated
 * canonical address.
 */
export async function loadAddressPageData(
  query: string,
  handlerUrl: string | undefined,
  fetcher: ExplorerFetch = defaultFetch,
): Promise<AddressPageLoadResult> {
  const normalizedName = normalizeQnsName(query);
  let address: string;
  let qnsName: string | undefined;

  if (normalizedName) {
    const resolution = await resolveQnsName(normalizedName, handlerUrl, fetcher);
    if (!resolution.ok) return resolution;
    address = resolution.address;
    qnsName = resolution.name;
  } else {
    if (hasQnsSuffix(query)) {
      return {
        ok: false,
        kind: 'qns-invalid',
        detail: 'Invalid QNS name. Use the conservative ASCII .qrl name format.',
      };
    }
    const canonicalAddress = canonicalizeQrlAddress(query);
    if (!canonicalAddress) {
      return {
        ok: false,
        kind: 'invalid-query',
        detail: 'Invalid or unsupported QRL address or QNS name.',
      };
    }
    address = canonicalAddress;
  }

  const addressData = await fetchAddressData(address, handlerUrl, fetcher);
  if (!addressData) {
    return {
      ok: false,
      kind: 'address-data',
      name: qnsName,
      detail: 'Failed to load address data.',
    };
  }

  return { ok: true, address, qnsName, addressData };
}
