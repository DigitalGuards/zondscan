const now = Date.parse('2026-09-10T12:00:00Z');
const sourceRows = [
  { base: 'USD', quote: 'EUR', rate: 0.85 },
  { base: 'USD', quote: 'GBP', rate: 0.74 },
  { base: 'USD', quote: 'CHF', rate: 0.8 },
  { base: 'USD', quote: 'CAD', rate: 1.37 },
  { base: 'USD', quote: 'AUD', rate: 1.38 },
  { base: 'USD', quote: 'JPY', rate: 153 },
  { base: 'USD', quote: 'CNY', rate: 6.7 },
].map((row) => ({ ...row, date: '2026-09-09' }));

beforeEach(() => {
  jest.resetModules();
  jest.spyOn(Date, 'now').mockReturnValue(now);
});

afterEach(() => jest.restoreAllMocks());

it('coalesces concurrent requests and caches only validated rates for one hour', async () => {
  let finish!: (response: Response) => void;
  const pending = new Promise<Response>((resolve) => {
    finish = resolve;
  });
  const fetchMock = jest.spyOn(global, 'fetch').mockReturnValueOnce(pending);
  const { GET } = await import('./route');
  const requests = [GET(), GET(), GET()];
  expect(fetchMock).toHaveBeenCalledTimes(1);
  finish(Response.json(sourceRows));
  for (const response of await Promise.all(requests)) {
    expect(response.status).toBe(200);
    expect(response.headers.get('Cache-Control')).toContain('s-maxage=3600');
    expect((await response.json()).rates).toMatchObject({ USD: 1, EUR: 0.85 });
  }
  expect((await GET()).status).toBe(200);
  expect(fetchMock).toHaveBeenCalledTimes(1);

  jest.mocked(Date.now).mockReturnValue(now + 60 * 60 * 1000);
  fetchMock.mockResolvedValueOnce(Response.json(sourceRows));
  expect((await GET()).status).toBe(200);
  expect(fetchMock).toHaveBeenCalledTimes(2);
});

it('uses a fixed ECB source and bounds redirects and request duration', async () => {
  const fetchMock = jest.spyOn(global, 'fetch').mockResolvedValueOnce(Response.json(sourceRows));
  const { GET } = await import('./route');
  await GET();
  const [url, options] = fetchMock.mock.calls[0];
  expect(url).toBe(
    'https://api.frankfurter.dev/v2/rates?base=USD&quotes=EUR,GBP,CHF,CAD,AUD,JPY,CNY&providers=ECB'
  );
  expect(options).toMatchObject({
    redirect: 'error',
    cache: 'no-store',
    headers: { Accept: 'application/json' },
  });
  expect(options!.signal).toBeInstanceOf(AbortSignal);
});

it('returns an explicit unavailable response and a bounded failure cooldown', async () => {
  const fetchMock = jest
    .spyOn(global, 'fetch')
    .mockRejectedValueOnce(new Error('private upstream details'));
  const { GET } = await import('./route');
  const response = await GET();
  expect(response.status).toBe(503);
  expect(response.headers.get('Cache-Control')).toBe('no-store');
  expect(response.headers.get('Retry-After')).toBe('60');
  expect(await response.json()).toEqual({ error: 'Currency conversion temporarily unavailable' });

  jest.mocked(Date.now).mockReturnValue(now + 30_000);
  expect((await GET()).status).toBe(503);
  expect(fetchMock).toHaveBeenCalledTimes(1);

  // Requests during cooldown must not push recovery further into the future.
  jest.mocked(Date.now).mockReturnValue(now + 60_001);
  fetchMock.mockResolvedValueOnce(Response.json(sourceRows));
  expect((await GET()).status).toBe(200);
  expect(fetchMock).toHaveBeenCalledTimes(2);
});

it.each([
  ['missing currency', () => Response.json(sourceRows.slice(1))],
  ['invalid JSON', () => new Response('{')],
  ['empty body', () => new Response(null)],
  ['oversized body', () => new Response('x'.repeat(16_385))],
  ['provider HTTP error', () => new Response('unavailable', { status: 503 })],
] as const)('rejects %s without manufacturing rates', async (_label, createResponse) => {
  jest.spyOn(global, 'fetch').mockResolvedValueOnce(createResponse());
  const { GET } = await import('./route');
  const response = await GET();
  expect(response.status).toBe(503);
  expect(await response.json()).not.toHaveProperty('rates');
});
