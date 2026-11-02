import { renderToStaticMarkup } from 'react-dom/server';
import { notFound, redirect } from 'next/navigation';
import TransactionPage from './page';

jest.mock('server-only', () => ({}));

jest.mock('next/navigation', () => ({
  notFound: jest.fn(() => {
    throw new Error('NEXT_NOT_FOUND');
  }),
  redirect: jest.fn((path: string) => {
    throw new Error(`NEXT_REDIRECT ${path}`);
  }),
}));

const hash = `0x${'a'.repeat(64)}`;
const originalFetch = global.fetch;

afterEach(() => {
  global.fetch = originalFetch;
  jest.restoreAllMocks();
});

it('keeps upstream failure distinct from transaction absence', async () => {
  jest.spyOn(console, 'error').mockImplementation(() => {});
  global.fetch = jest
    .fn()
    .mockResolvedValueOnce({ ok: false, status: 404 })
    .mockResolvedValueOnce({ ok: false, status: 503 });
  const html = renderToStaticMarkup(
    await TransactionPage({ params: Promise.resolve({ query: hash }) })
  );
  expect(html).toContain('Transaction Details Unavailable');
  expect(html).not.toContain('Transaction Not Found');
});

it('lets a confirmed 404 reach the framework not-found boundary', async () => {
  global.fetch = jest.fn().mockResolvedValue({ ok: false, status: 404 });
  await expect(TransactionPage({ params: Promise.resolve({ query: hash }) })).rejects.toThrow(
    'NEXT_NOT_FOUND'
  );
  expect(notFound).toHaveBeenCalled();
});

const OWNER =
  'QaaaAAaaaAAaAaaAaAAAAaAAAaAaAaAAaAaaAaaaaAAAAAAAAaAAAAaAaAAaaAAaaaaAaAAAAaaAaAAaaaaaaAaAAaaaaAaAaaaaAaaaAAAAaAAAAaAaAaaAAAaAaaaAA';

function json(status: number, payload: unknown) {
  return { ok: status >= 200 && status < 300, status, json: () => Promise.resolve(payload) };
}

// Request order: pending check, transaction, block by hash, QNS node.
function fetchByPath(block: unknown, qns: unknown) {
  return jest.fn((url: string) => {
    if (url.includes('/block/')) return Promise.resolve(block);
    if (url.includes('/qns/node/')) return Promise.resolve(qns);
    return Promise.resolve({ ok: false, status: 404 });
  });
}

it('redirects a block hash to the block page', async () => {
  global.fetch = fetchByPath(
    json(200, { block: { result: { hash, number: '0x2a' } } }),
    json(404, { error: 'QNS node has no owner' })
  ) as unknown as typeof fetch;
  await expect(TransactionPage({ params: Promise.resolve({ query: hash }) })).rejects.toThrow(
    'NEXT_REDIRECT /block/42'
  );
  expect(redirect).toHaveBeenCalledWith('/block/42');
});

it('explains a QNS name hash with its owner and name', async () => {
  global.fetch = fetchByPath(
    json(404, { error: 'block not found' }),
    json(200, { node: hash, owner: OWNER, resolver: null, name: 'alice.qrl' })
  ) as unknown as typeof fetch;
  const html = renderToStaticMarkup(
    await TransactionPage({ params: Promise.resolve({ query: hash }) })
  );
  expect(html).toContain('This is a QNS name hash');
  expect(html).toContain('alice.qrl');
  expect(html).toContain(`href="/address/${OWNER}"`);
  expect(html).toContain('No resolver set');
});

it('says when a QNS name hash has no published name', async () => {
  global.fetch = fetchByPath(
    json(404, { error: 'block not found' }),
    json(200, { node: hash, owner: OWNER, resolver: `Q${'2'.repeat(128)}`, name: null })
  ) as unknown as typeof fetch;
  const html = renderToStaticMarkup(
    await TransactionPage({ params: Promise.resolve({ query: hash }) })
  );
  expect(html).toContain('Name not published');
});

it('reaches not-found when the hash matches no block or QNS name', async () => {
  global.fetch = fetchByPath(
    json(404, { error: 'block not found' }),
    json(404, { error: 'QNS node has no owner' })
  ) as unknown as typeof fetch;
  await expect(TransactionPage({ params: Promise.resolve({ query: hash }) })).rejects.toThrow(
    'NEXT_NOT_FOUND'
  );
});
