import { renderToStaticMarkup } from 'react-dom/server';
import { notFound } from 'next/navigation';
import TransactionPage from './page';

jest.mock('next/navigation', () => ({
  notFound: jest.fn(() => {
    throw new Error('NEXT_NOT_FOUND');
  }),
  redirect: jest.fn(),
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
