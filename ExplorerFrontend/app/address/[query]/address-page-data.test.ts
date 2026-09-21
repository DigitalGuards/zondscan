import { describe, expect, it } from '@jest/globals';

import type { ExplorerFetch } from './address-page-data';
import { loadAddressPageData, resolveQnsName } from './address-page-data';
import { classifyStoredVerification } from '../../lib/storedVerification';

const CHECKSUMMED_ADDRESS =
  'QaaaAAaaaAAaAaaAaAAAAaAAAaAaAaAAaAaaAaaaaAAAAAAAAaAAAAaAaAAaaAAaaaaAaAAAAaaAaAAaaaaaaAaAAaaaaAaAaaaaAaaaAAAAaAAAAaAaAaaAAAaAaaaAA';

function jsonResponse(status: number, payload: unknown) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: async () => payload,
  };
}

function aggregatePayload() {
  return {
    address: { balance: 12.5 },
    rank: 7,
    transactions_by_address: [],
    internal_transactions_by_address: [],
    contract_code: null,
    response: null,
  };
}

describe('QNS address page loading', () => {
  it('normalizes the name, resolves first, and aggregates only the returned canonical address', async () => {
    const calls: Array<{ input: string; init: Parameters<ExplorerFetch>[1] }> = [];
    const fetcher: ExplorerFetch = async (input, init) => {
      calls.push({ input, init });
      if (calls.length === 1) {
        return jsonResponse(200, {
          name: 'moscowchill.qrl',
          address: CHECKSUMMED_ADDRESS,
        });
      }
      return jsonResponse(200, aggregatePayload());
    };

    const result = await loadAddressPageData(
      'MoscowChill.QRL',
      'https://handler.example/',
      fetcher,
    );

    expect(calls.map((call) => call.input)).toEqual([
      'https://handler.example/qns/resolve/moscowchill.qrl',
      `https://handler.example/address/aggregate/${CHECKSUMMED_ADDRESS}`,
    ]);
    expect(calls[0]?.init).toEqual({ cache: 'no-store' });
    expect(calls[1]?.init).toEqual({ next: { revalidate: 10 } });
    expect(result).toMatchObject({
      ok: true,
      qnsName: 'moscowchill.qrl',
      address: CHECKSUMMED_ADDRESS,
    });
  });

  it.each(['café.qrl', 'alice..qrl', 'xn--name.qrl'])(
    'does not issue a request for invalid QNS input %s',
    async (input) => {
      let requests = 0;
      const fetcher: ExplorerFetch = async () => {
        requests += 1;
        return jsonResponse(500, {});
      };

      const result = await loadAddressPageData(input, 'https://handler.example', fetcher);

      expect(result).toMatchObject({ ok: false, kind: 'qns-invalid' });
      expect(requests).toBe(0);
    },
  );

  it('does not issue a request when the QNS backend URL is unconfigured', async () => {
    let requests = 0;
    const fetcher: ExplorerFetch = async () => {
      requests += 1;
      return jsonResponse(500, {});
    };

    const result = await loadAddressPageData('moscowchill.qrl', undefined, fetcher);

    expect(result).toEqual({
      ok: false,
      kind: 'qns-unconfigured',
      name: 'moscowchill.qrl',
      detail: 'QNS resolution is not configured',
    });
    expect(requests).toBe(0);
  });

  it.each([
    [
      400,
      { error: 'invalid QNS name' },
      'qns-invalid',
      'The QNS service rejected this name as invalid.',
    ],
    [
      503,
      { error: 'QNS resolution is not configured' },
      'qns-unconfigured',
      'QNS resolution is not configured',
    ],
    [
      404,
      { error: 'QNS name has no address record', name: 'moscowchill.qrl' },
      'qns-missing',
      'QNS name has no address record',
    ],
    [502, { error: 'QNS resolution failed' }, 'qns-upstream', 'QNS resolution failed upstream.'],
  ] as const)(
    'stops after a %i resolver response',
    async (status, payload, kind, detail) => {
      const calls: string[] = [];
      const fetcher: ExplorerFetch = async (input) => {
        calls.push(input);
        return jsonResponse(status, payload);
      };

      const result = await loadAddressPageData(
        'moscowchill.qrl',
        'https://handler.example',
        fetcher,
      );

      expect(result).toMatchObject({ ok: false, kind, detail });
      expect(calls).toEqual(['https://handler.example/qns/resolve/moscowchill.qrl']);
    },
  );

  it.each([
    {
      name: 'different.qrl',
      address: CHECKSUMMED_ADDRESS,
    },
    {
      name: 'moscowchill.qrl',
      address: `Q${'a'.repeat(128)}`,
    },
    {
      name: 'moscowchill.qrl',
      address: `Q${'Ab'.repeat(64)}`,
    },
  ])('rejects a malformed successful resolver payload before aggregation', async (payload) => {
    const calls: string[] = [];
    const fetcher: ExplorerFetch = async (input) => {
      calls.push(input);
      return jsonResponse(200, payload);
    };

    const result = await loadAddressPageData(
      'moscowchill.qrl',
      'https://handler.example',
      fetcher,
    );

    expect(result).toMatchObject({ ok: false, kind: 'qns-upstream' });
    expect(calls).toHaveLength(1);
  });

  it('keeps direct address lookups on the aggregate endpoint without invoking QNS', async () => {
    const calls: string[] = [];
    const fetcher: ExplorerFetch = async (input) => {
      calls.push(input);
      return jsonResponse(200, aggregatePayload());
    };

    const result = await loadAddressPageData(
      `0X${'A'.repeat(128)}`,
      'https://handler.example',
      fetcher,
    );

    expect(result).toMatchObject({ ok: true, address: CHECKSUMMED_ADDRESS });
    expect(calls).toEqual([
      `https://handler.example/address/aggregate/${CHECKSUMMED_ADDRESS}`,
    ]);
  });

  it('preserves explicit null verification fields as an invalid record', async () => {
    const payload = {
      ...aggregatePayload(),
      contract_code: {
        address: CHECKSUMMED_ADDRESS,
        contractCode: '',
        verified: true,
        verificationRecordSchema: null,
        compilerProvenance: null,
        sourceBundleDigest: null,
      },
    };
    const result = await loadAddressPageData(
      CHECKSUMMED_ADDRESS,
      'https://handler.example',
      async () => jsonResponse(200, payload),
    );

    expect(result.ok).toBe(true);
    if (!result.ok) throw new Error('expected address data');
    expect(result.addressData.contract_code).toMatchObject({
      verificationRecordSchema: null,
      compilerProvenance: null,
      sourceBundleDigest: null,
    });
    expect(
      classifyStoredVerification(result.addressData.contract_code ?? {}),
    ).toBe('invalid-recorded');
  });
});

describe('resolveQnsName error boundaries', () => {
  it('maps timeout and rate-limit responses to temporary upstream errors', async () => {
    const timeout = await resolveQnsName(
      'alice.qrl',
      'https://handler.example',
      async () => jsonResponse(504, { error: 'QNS resolution timed out' }),
    );
    const rateLimited = await resolveQnsName(
      'alice.qrl',
      'https://handler.example',
      async () => jsonResponse(429, { error: 'rate limit exceeded' }),
    );

    expect(timeout).toMatchObject({ kind: 'qns-upstream', detail: 'QNS resolution timed out.' });
    expect(rateLimited).toMatchObject({
      kind: 'qns-upstream',
      detail: 'Too many QNS lookups were requested. Try again shortly.',
    });
  });
});
