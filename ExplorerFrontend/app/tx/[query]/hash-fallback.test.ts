import { afterEach, beforeEach, describe, expect, it, jest } from '@jest/globals';

jest.mock('server-only', () => ({}));

import { resolveUnknownHash, type FallbackFetch } from './hash-fallback';

const NODE = '0x2874b520ad0679a7c812be276fb0a234ca05373d48a02119afa825be09845e99';
const OWNER =
  'QaaaAAaaaAAaAaaAaAAAAaAAAaAaAaAAaAaaAaaaaAAAAAAAAaAAAAaAaAAaaAAaaaaAaAAAAaaAaAAaaaaaaAaAAaaaaAaAaaaaAaaaAAAAaAAAAaAaAaaAAAaAaaaAA';
const RESOLVER = `Q${'2'.repeat(128)}`;
const HANDLER = 'http://handler.test/';

function respond(status: number, payload?: unknown) {
  return {
    ok: status >= 200 && status < 300,
    status,
    json: () => Promise.resolve(payload),
  };
}

function fetcherFor(routes: Record<string, ReturnType<typeof respond> | Error>) {
  const calls: Array<{ url: string; init?: RequestInit }> = [];
  const fetcher: FallbackFetch = (url, init) => {
    calls.push({ url, init });
    if (!Object.hasOwn(routes, url)) return Promise.resolve(respond(404, { error: 'not found' }));
    const route = routes[url];
    return route instanceof Error ? Promise.reject(route) : Promise.resolve(route);
  };
  return { fetcher, calls };
}

describe('resolveUnknownHash', () => {
  beforeEach(() => {
    jest.replaceProperty(process, 'env', { ...process.env, QNS_INTERNAL_TOKEN: '' });
  });
  afterEach(() => {
    jest.restoreAllMocks();
  });

  it('returns the block number for a stored block hash', async () => {
    const { fetcher, calls } = fetcherFor({
      [`http://handler.test/block/${NODE}`]: respond(200, {
        block: { result: { hash: NODE, number: '0xb6a4' } },
      }),
    });
    await expect(resolveUnknownHash(NODE.toUpperCase().replace('0X', '0x'), HANDLER, fetcher)).resolves.toEqual({
      kind: 'block',
      number: 46756,
    });
    expect(calls).toHaveLength(1);
  });

  it('ignores a block payload whose hash differs', async () => {
    const { fetcher } = fetcherFor({
      [`http://handler.test/block/${NODE}`]: respond(200, {
        block: { result: { hash: `0x${'1'.repeat(64)}`, number: '0x1' } },
      }),
    });
    await expect(resolveUnknownHash(NODE, HANDLER, fetcher)).resolves.toEqual({ kind: 'none' });
  });

  it('returns a QNS node with its published name', async () => {
    const { fetcher } = fetcherFor({
      [`http://handler.test/qns/node/${NODE}`]: respond(200, {
        node: NODE,
        owner: OWNER,
        resolver: RESOLVER,
        name: 'alice.qrl',
      }),
    });
    await expect(resolveUnknownHash(NODE, HANDLER, fetcher)).resolves.toEqual({
      kind: 'qns',
      record: { node: NODE, owner: OWNER, resolver: RESOLVER, name: 'alice.qrl' },
    });
  });

  it('accepts a QNS node without a name or resolver', async () => {
    const { fetcher } = fetcherFor({
      [`http://handler.test/qns/node/${NODE}`]: respond(200, {
        node: NODE,
        owner: OWNER,
        resolver: null,
        name: null,
      }),
    });
    await expect(resolveUnknownHash(NODE, HANDLER, fetcher)).resolves.toEqual({
      kind: 'qns',
      record: { node: NODE, owner: OWNER, resolver: null, name: null },
    });
  });

  it.each([
    ['a different node', { node: `0x${'1'.repeat(64)}`, owner: OWNER, resolver: null, name: null }],
    ['a missing owner', { node: NODE, owner: null, resolver: null, name: null }],
    ['a non-canonical owner', { node: NODE, owner: OWNER.toLowerCase(), resolver: null, name: null }],
    ['an unnormalized name', { node: NODE, owner: OWNER, resolver: RESOLVER, name: 'Alice.qrl' }],
    ['a malformed resolver', { node: NODE, owner: OWNER, resolver: 'Q12', name: null }],
  ])('rejects a QNS payload with %s', async (_label, payload) => {
    const { fetcher } = fetcherFor({ [`http://handler.test/qns/node/${NODE}`]: respond(200, payload) });
    await expect(resolveUnknownHash(NODE, HANDLER, fetcher)).resolves.toEqual({ kind: 'none' });
  });

  it('reports a check that could not run', async () => {
    const { fetcher, calls } = fetcherFor({
      [`http://handler.test/block/${NODE}`]: new Error('offline'),
      [`http://handler.test/qns/node/${NODE}`]: respond(503, { error: 'QNS resolution is not configured' }),
    });
    await expect(resolveUnknownHash(NODE, HANDLER, fetcher)).resolves.toEqual({ kind: 'unavailable' });
    expect(calls.map((call) => call.url)).toEqual([
      `http://handler.test/block/${NODE}`,
      `http://handler.test/qns/node/${NODE}`,
    ]);
  });

  it('treats a QNS 503 alone as unavailable', async () => {
    const { fetcher } = fetcherFor({
      [`http://handler.test/block/${NODE}`]: respond(404, { error: 'not found' }),
      [`http://handler.test/qns/node/${NODE}`]: respond(503, { error: 'QNS resolution is not configured' }),
    });
    await expect(resolveUnknownHash(NODE, HANDLER, fetcher)).resolves.toEqual({ kind: 'unavailable' });
  });

  it('treats two 404 answers as no match', async () => {
    const { fetcher } = fetcherFor({
      [`http://handler.test/block/${NODE}`]: respond(404, { error: 'not found' }),
      [`http://handler.test/qns/node/${NODE}`]: respond(404, { error: 'QNS node has no owner' }),
    });
    await expect(resolveUnknownHash(NODE, HANDLER, fetcher)).resolves.toEqual({ kind: 'none' });
  });

  it('sends no internal QNS header, so the per-IP limit applies', async () => {
    const token = 'test-internal-value';
    jest.replaceProperty(process, 'env', { ...process.env, QNS_INTERNAL_TOKEN: token });
    const { fetcher, calls } = fetcherFor({});
    await resolveUnknownHash(NODE, HANDLER, fetcher);
    expect(calls[0].init?.headers).toBeUndefined();
    expect(calls[1].init?.headers).toBeUndefined();
  });

  it('does nothing without a handler URL', async () => {
    const { fetcher, calls } = fetcherFor({});
    await expect(resolveUnknownHash(NODE, '  ', fetcher)).resolves.toEqual({ kind: 'none' });
    expect(calls).toHaveLength(0);
  });
});
