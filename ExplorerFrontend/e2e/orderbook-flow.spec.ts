import { expect, test } from '@playwright/test';
import type { MarketFundFlowResponse } from '../app/lib/marketflow';
import type { MarketOrderBookResponse } from '../app/lib/orderbook';

const day = 86_400_000;
const end = Date.UTC(2026, 8, 22, 12);
const windows = ['15m', '30m', '1h', '2h', '4h', '1d', '7d', '30d'];
const venue = {
  id: 'mexc',
  name: 'MEXC',
  symbol: 'QRLUSDT',
  quoteAsset: 'USDT',
};
const totals = {
  bucket: 'total',
  buyQuantity: 200,
  sellQuantity: 100,
  netQuantity: 100,
  buyQuote: 128,
  sellQuote: 64,
  netQuote: 64,
  buyTradeCount: 2,
  sellTradeCount: 1,
};

function flow(window: string, empty = false): MarketFundFlowResponse {
  const durations: Record<string, [number, number]> = {
    '15m': [900_000, 60_000],
    '30m': [1_800_000, 120_000],
    '1h': [3_600_000, 300_000],
    '2h': [7_200_000, 600_000],
    '4h': [14_400_000, 900_000],
    '1d': [day, 3_600_000],
    '7d': [7 * day, 6 * 3_600_000],
    '30d': [30 * day, day],
  };
  const [duration, step] = durations[window];
  const start = end - duration;
  const dailyDays = window === '30d' ? 30 : window === '7d' ? 7 : 5;
  const point = (time: number, index: number) => ({
    time,
    buyQuantity: empty ? 0 : index % 2 ? 0 : 200,
    sellQuantity: empty ? 0 : index % 2 ? 100 : 0,
    netQuantity: empty ? 0 : index % 2 ? -100 : 200,
  });
  const dailyStart = Date.UTC(2026, 8, 23 - dailyDays);
  return {
    venue,
    venues: [venue],
    window,
    windows,
    windowStart: start,
    windowEnd: end,
    seriesStepMs: step,
    bands: { mediumFrom: 10, largeFrom: 100, quoteAsset: 'USDT' },
    buckets: [],
    totals: empty
      ? {
          ...totals,
          buyQuantity: 0,
          sellQuantity: 0,
          netQuantity: 0,
          buyTradeCount: 0,
          sellTradeCount: 0,
        }
      : totals,
    series: Array.from({ length: duration / step }, (_, i) => point(start + i * step, i)),
    daily: Array.from({ length: dailyDays }, (_, i) => point(dailyStart + i * day, i)),
    dailyDays,
    dailyStart,
    dailyEnd: end,
    coverage: {
      firstTradeAt: end - 10 * day,
      lastTradeAt: end - 60_000,
      tradeCount: 500,
      complete: duration <= 10 * day,
    },
  };
}

const book: MarketOrderBookResponse = {
  venue: 'MEXC',
  symbol: 'QRLUSDT',
  fetchedAt: new Date().toISOString(),
  lastUpdateId: 1,
  bids: Array.from({ length: 50 }, (_, i) => ({
    price: (0.635 - i * 0.001).toFixed(5),
    quantity: String(100 + i),
  })),
  asks: Array.from({ length: 50 }, (_, i) => ({
    price: (0.64 + i * 0.001).toFixed(5),
    quantity: String(150 + i),
  })),
  recentTrades: [
    {
      id: 'fixture-buy',
      price: '0.64',
      quantity: '12.5',
      quoteQuantity: '8',
      time: end,
      aggressorSide: 'buy',
    },
    {
      id: 'fixture-sell',
      price: '0.635',
      quantity: '20',
      quoteQuantity: '12.7',
      time: end - 60_000,
      aggressorSide: 'sell',
    },
  ],
  ticker: {
    high: '0.66',
    low: '0.62',
    last: '0.64',
    change: '0.01',
    changePercent: '0.01587',
    baseVolume: '20000',
    quoteVolume: '12800',
  },
};

for (const width of [320, 390, 1440]) {
  test(`standard orderbook and longer flow analysis at ${width}px`, async ({
    page,
    baseURL,
  }, testInfo) => {
    expect(new URL(baseURL!).hostname).toBe('127.0.0.1');
    const errors: string[] = [];
    const gameAssets: string[] = [];
    const requestedWindows: string[] = [];
    let bookRequests = 0;
    page.on('pageerror', (error) => errors.push(error.message));
    page.on('request', (request) => {
      if (/squirrel|stadium/i.test(request.url())) gameAssets.push(request.url());
    });
    await page.route('**/*', (route) => {
      const url = new URL(route.request().url());
      if (!['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname)) return route.abort();
      if (url.pathname.endsWith('/market/orderbook')) {
        bookRequests += 1;
        return route.fulfill({ json: { ...book, fetchedAt: new Date().toISOString() } });
      }
      if (url.pathname.endsWith('/market/fundflow')) {
        const window = url.searchParams.get('window') ?? '1d';
        requestedWindows.push(window);
        return route.fulfill({ json: flow(window, window === '15m') });
      }
      return route.continue();
    });
    await page.setViewportSize({ width, height: 1000 });
    await page.goto('/orderbook');
    await expect(page).toHaveTitle('QRL Order Book | ZondScan');
    const panel = page.getByRole('region', { name: 'Fund flow analysis' });
    await expect(panel.getByText('Net buy volume, 24H', { exact: true })).toBeVisible();
    expect(await page.locator('#main-content').innerText()).not.toMatch(
      /squirrel|arena|play by play|yard|score|team|formation/i
    );
    await expect(page.getByRole('heading', { name: 'Recent trades', exact: true })).toBeVisible();
    await page.getByLabel('Price grouping in USDT').selectOption('0.0001');
    await page.getByLabel('Order book rows per side').selectOption('25');
    await expect(
      page.getByRole('table', { name: 'Bids QRL USDT order book' }).locator('tbody tr')
    ).toHaveCount(25);
    await expect(
      page.getByRole('table', { name: 'Asks QRL USDT order book' }).locator('tbody tr')
    ).toHaveCount(25);
    await page.getByLabel('Order book rows per side').selectOption('12');
    await page.getByRole('button', { name: 'Pause order book', exact: true }).click();
    await expect(
      page.getByRole('button', { name: 'Resume order book', exact: true })
    ).toBeVisible();
    const pausedRequests = bookRequests;
    if (width === 1440) await page.waitForTimeout(3300);
    expect(bookRequests).toBe(pausedRequests);
    await page.getByRole('button', { name: 'Resume order book', exact: true }).click();
    await expect.poll(() => bookRequests).toBeGreaterThan(pausedRequests);
    for (const [label, count] of [
      ['7D', 28],
      ['30D', 30],
    ] as const) {
      await panel.getByRole('button', { name: label, exact: true }).click();
      await expect(panel.getByText(`Net buy volume, ${label}`, { exact: true })).toBeVisible();
      const chart = panel.getByRole('group', {
        name: new RegExp(`Net QRL buy volume per .* over ${label}$`),
      });
      await expect(chart.locator('g[role="img"]')).toHaveCount(count);
      const first = chart.locator('g[role="img"]').first();
      await first.focus();
      await expect(panel.getByRole('status')).toContainText('UTC');
      await expect(first).toHaveAttribute('aria-label', /2026-.* UTC: net buy volume/);
      await first.blur();
      await expect(panel.getByRole('button', { name: label, exact: true })).toHaveAttribute(
        'aria-pressed',
        'true'
      );
      expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(
        true
      );
    }
    await expect(panel.getByText(/^Partial history:/).first()).toBeVisible();
    await expect(
      panel.getByRole('heading', {
        name: 'Daily net buy volume, last 30 days',
      })
    ).toBeVisible();
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({
      path: testInfo.outputPath(`orderbook-${width}-dark.png`),
      fullPage: true,
    });
    await page.getByRole('button', { name: /^Appearance:/ }).click();
    await page.getByRole('menuitem', { name: /^Light/ }).click();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
    await page.evaluate(() => window.scrollTo(0, 0));
    await page.screenshot({
      path: testInfo.outputPath(`orderbook-${width}-light.png`),
      fullPage: true,
    });
    await panel.getByRole('button', { name: '15m', exact: true }).click();
    await expect(
      panel
        .getByText(
          'No trades recorded for this period. Values reflect the available collected history.'
        )
        .first()
    ).toBeVisible();
    expect(requestedWindows).toEqual(expect.arrayContaining(['1d', '7d', '30d', '15m']));
    expect(errors).toEqual([]);
    expect(gameAssets).toEqual([]);
  });
}

test('a slower previous timeframe never replaces the newly selected range', async ({ page }) => {
  let releasePrevious: () => void = () => undefined;
  const previousGate = new Promise<void>((resolve) => {
    releasePrevious = resolve;
  });
  await page.route('**/*', async (route) => {
    const url = new URL(route.request().url());
    if (!['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname)) return route.abort();
    if (url.pathname.endsWith('/market/orderbook')) return route.fulfill({ json: book });
    if (url.pathname.endsWith('/market/fundflow')) {
      const window = url.searchParams.get('window') ?? '1d';
      if (window === '7d') await previousGate;
      return route.fulfill({ json: flow(window) });
    }
    return route.continue();
  });
  try {
    await page.goto('/orderbook');
    const panel = page.getByRole('region', { name: 'Fund flow analysis' });
    await expect(panel.getByText('Net buy volume, 24H', { exact: true })).toBeVisible();
    const previousRequest = page.waitForRequest((request) =>
      request.url().includes('/market/fundflow?window=7d')
    );
    await panel.getByRole('button', { name: '7D', exact: true }).click();
    await previousRequest;
    await expect(panel.getByRole('status')).toContainText(
      'Loading 7D data. Showing the previous 24H view'
    );
    await panel.getByRole('button', { name: '30D', exact: true }).click();
    await expect(panel.getByText('Net buy volume, 30D', { exact: true })).toBeVisible();
    const previousResponse = page.waitForResponse((response) =>
      response.url().includes('/market/fundflow?window=7d')
    );
    releasePrevious();
    await previousResponse;
    await expect(panel.getByText('Net buy volume, 30D', { exact: true })).toBeVisible();
    await expect(panel.getByRole('button', { name: '30D', exact: true })).toHaveAttribute(
      'aria-pressed',
      'true'
    );
  } finally {
    releasePrevious();
  }
});
