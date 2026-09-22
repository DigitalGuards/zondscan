import { renderToStaticMarkup } from 'react-dom/server';
import { useQuery } from '@tanstack/react-query';
import axios from 'axios';
import type { MarketOrderBookResponse } from '../lib/orderbook';
import OrderBookClient from './orderbook-client';

jest.mock('@tanstack/react-query', () => ({
  useQuery: jest.fn(),
  useQueryClient: () => ({ cancelQueries: jest.fn() }),
}));
jest.mock('axios');
jest.mock('./fund-flow', () => ({
  __esModule: true,
  default: () => <section aria-label="Fund flow analysis">Fund flow analysis</section>,
}));
jest.mock('../components/TimeDisplay', () => ({
  __esModule: true,
  default: ({ timestamp }: { timestamp: number }) => <time>{timestamp}</time>,
}));

const now = Date.parse('2026-09-22T12:00:00Z');
const fixture: MarketOrderBookResponse = {
  venue: 'MEXC',
  symbol: 'QRLUSDT',
  fetchedAt: new Date(now).toISOString(),
  lastUpdateId: 42,
  bids: [{ price: '0.7501', quantity: '20' }],
  asks: [{ price: '0.7599', quantity: '15' }],
  recentTrades: [
    {
      id: 'buy',
      price: '0.76',
      quantity: '7.75',
      quoteQuantity: '5.89',
      time: now,
      aggressorSide: 'buy',
    },
    {
      id: 'sell',
      price: '0.75',
      quantity: '2.50',
      quoteQuantity: '1.875',
      time: now - 1_000,
      aggressorSide: 'sell',
    },
  ],
  ticker: {
    high: '0.82',
    low: '0.70',
    last: '0.76',
    change: '0.01',
    changePercent: '0.013',
    baseVolume: '1200',
    quoteVolume: '910',
  },
};

function render(data: MarketOrderBookResponse | undefined = fixture, options = {}) {
  jest.mocked(useQuery).mockReturnValue({
    data,
    dataUpdatedAt: now,
    isFetching: false,
    isError: false,
    refetch: jest.fn(),
    ...options,
  } as unknown as ReturnType<typeof useQuery>);
  return renderToStaticMarkup(<OrderBookClient />);
}

beforeEach(() => jest.clearAllMocks());

it('renders conventional market labels, explicit units and the independent analysis panel', () => {
  const html = render();
  for (const label of [
    'QRL / USDT',
    'MEXC Spot',
    'Order book',
    'Recent trades',
    'Price (USDT)',
    'Amount (QRL)',
    'Total (QRL)',
    'Bids',
    'Asks',
    '>Buy<',
    '>Sell<',
    'Spread',
    '24h volume',
    '+1.30% in 24h',
    'Visible depth within 2% of midpoint',
    'Fund flow analysis',
    '7.75',
  ])
    expect(html).toContain(label);
  expect(html).not.toMatch(/squirrel|stadium|arena|yard|play by play|team|score/i);
  expect(html).not.toContain('/orderbook/');
  expect(html).toContain('aria-label="Price grouping in USDT"');
  expect(html).toContain('aria-label="Order book rows per side"');
  expect(html).toContain('value="0.001" selected');
  expect(html).toContain('value="12" selected');
  expect(html).toContain('Pause order book');
  expect(html).toContain('>Live</span>');
});

it('keeps a cached snapshot visible when refresh fails', () => {
  const html = render(fixture, { isError: true });
  expect(html).toContain('Data delayed');
  expect(html).toContain('Showing the last available snapshot.');
  expect(html).toContain('Retry');
  expect(html).toContain('Recent trades');
});

it('labels old and invalid snapshot timestamps as delayed', () => {
  for (const fetchedAt of [new Date(now - 20_000).toISOString(), 'invalid']) {
    const html = render({ ...fixture, fetchedAt });
    expect(html).toContain('Data delayed');
    expect(html).not.toContain('>Live</span>');
  }
});

it('shows explicit empty book and trade states', () => {
  const html = render({ ...fixture, bids: [], asks: [], recentTrades: [] });
  expect(html).toContain('No bids in this snapshot.');
  expect(html).toContain('No asks in this snapshot.');
  expect(html).toContain('No recent trades available.');
  expect(html).not.toContain('50.0% bid depth');
});

it('labels missing quote volume without treating it as zero', () => {
  expect(render({ ...fixture, ticker: { ...fixture.ticker, quoteVolume: null } })).toContain(
    'Quote volume unavailable'
  );
});

it('renders a loading status while the initial snapshot is pending', () => {
  const html = render(fixture, { data: undefined });
  expect(html).toContain('role="status"');
  expect(html).toContain('Loading market data');
  expect(html).not.toContain('>Live</span>');
});

it('provides an initial error and retry without inventing market values', () => {
  const html = render(fixture, { data: undefined, isError: true });
  expect(html).toContain('role="alert"');
  expect(html).toContain('Market feed unavailable');
  expect(html).toContain('Retry');
  expect(html).not.toContain('24h volume');
});

it('keeps bounded polling and forwards cancellation to the data request', async () => {
  render();
  const options = jest.mocked(useQuery).mock.calls[0][0];
  expect(options.enabled).toBe(true);
  expect(options.refetchInterval).toBe(3_000);
  expect(options.refetchIntervalInBackground).toBe(false);
  expect(options.retry).toBe(2);
  jest.mocked(axios.get).mockResolvedValueOnce({ data: fixture });
  const signal = new AbortController().signal;
  const queryFn = options.queryFn as (context: {
    signal: AbortSignal;
  }) => Promise<MarketOrderBookResponse>;
  await expect(queryFn({ signal })).resolves.toEqual(fixture);
  expect(axios.get).toHaveBeenCalledWith(expect.stringContaining('/market/orderbook'), {
    timeout: 8_000,
    signal,
  });
});
