import { afterEach, beforeEach, describe, expect, it } from '@jest/globals';
import { headers } from 'next/headers';
import { renderToStaticMarkup } from 'react-dom/server';

import Page, { generateMetadata } from './page';

jest.mock('server-only', () => ({}));
jest.mock('next/headers', () => ({ headers: jest.fn() }));
jest.mock('../../../config', () => ({ __esModule: true, default: { handlerUrl: '/handler' } }));
jest.mock('./address-view', () => ({ __esModule: true, default: () => null }));
jest.mock('./token-contract-view', () => ({ __esModule: true, default: () => null }));

const address = `Q${'4'.repeat(128)}`;
const primaryName = 'alice.qrl';
const addressTitle = 'Address Q44444444...44444444...44444444';

function response(payload: unknown, status = 200): Response {
  return new Response(JSON.stringify(payload), { status });
}

describe('address page server requests', () => {
  beforeEach(() => {
    jest.clearAllMocks();
    jest.replaceProperty(process, 'env', { ...process.env, QNS_INTERNAL_TOKEN: '' });
    jest.mocked(headers).mockImplementation(() => {
      throw new Error('Address routes must preserve rendering without request headers');
    });
  });

  afterEach(() => {
    jest.restoreAllMocks();
    jest.useRealTimers();
  });

  it.each([address, primaryName])(
    'preserves route copy and caching without reading visitor headers for %s',
    async (query) => {
      const fetcher = jest
        .spyOn(globalThis, 'fetch')
        .mockImplementation((input) =>
          Promise.resolve(
            typeof input === 'string' && input.includes('/qns/')
              ? response({ address, name: primaryName })
              : response({ address: { balance: 1 }, contract_code: null })
          )
        );

      const html = renderToStaticMarkup(await Page({ params: Promise.resolve({ query }) }));

      const qnsCall = fetcher.mock.calls.find(
        ([input]) => typeof input === 'string' && input.includes('/qns/')
      );
      expect(qnsCall).toBeDefined();
      expect(qnsCall?.[1]?.headers).toBeUndefined();
      expect(headers).not.toHaveBeenCalled();
      expect(html).toContain(
        query === address
          ? `Address ${address} with the verified QNS primary name ${primaryName}`
          : `${primaryName} resolves to ${address}`
      );
    }
  );

  it('generates address metadata while the aggregate would hang', async () => {
    const fetcher = jest
      .spyOn(globalThis, 'fetch')
      .mockImplementation((input) =>
        typeof input === 'string' && input.includes('/qns/reverse/')
          ? Promise.resolve(response({ address, name: primaryName }))
          : new Promise(() => {})
      );

    const metadata = await generateMetadata({ params: Promise.resolve({ query: address }) });

    expect(metadata.title).toBe(`${addressTitle} (${primaryName}) | ZondScan`);
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(fetcher).toHaveBeenCalledWith(`/handler/qns/reverse/${address}`, {
      next: { revalidate: 60 },
      signal: expect.any(AbortSignal),
    });
    expect(headers).not.toHaveBeenCalled();
  });

  it('bounds metadata to 1500 ms when reverse lookup hangs', async () => {
    jest.useFakeTimers();
    const fetcher = jest.spyOn(globalThis, 'fetch').mockImplementation(() => new Promise(() => {}));
    let settled = false;
    const loading = generateMetadata({ params: Promise.resolve({ query: address }) }).then(
      (metadata) => {
        settled = true;
        return metadata;
      }
    );

    await jest.advanceTimersByTimeAsync(1499);
    expect(settled).toBe(false);
    await jest.advanceTimersByTimeAsync(1);

    await expect(loading).resolves.toMatchObject({ title: `${addressTitle} | ZondScan` });
    expect(fetcher.mock.calls.map(([input]) => input)).toEqual([`/handler/qns/reverse/${address}`]);
    expect(fetcher.mock.calls[0]?.[1]?.signal?.aborted).toBe(true);
  });

  it('generates name-route metadata directly from the parameter', async () => {
    const fetcher = jest.spyOn(globalThis, 'fetch').mockImplementation(() => new Promise(() => {}));

    const metadata = await generateMetadata({ params: Promise.resolve({ query: 'ALICE.QRL' }) });

    expect(metadata.title).toBe('alice.qrl QNS Address | ZondScan');
    expect(metadata.description).toContain('QRL address resolved by QNS name alice.qrl');
    expect(fetcher).not.toHaveBeenCalled();
    expect(headers).not.toHaveBeenCalled();
  });
});
