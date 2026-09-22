import { renderToStaticMarkup } from 'react-dom/server';
import type { MarketFundFlowResponse } from '../lib/marketflow';
import FundFlowPanel from './fund-flow';

let mockQueryState: {
  data: MarketFundFlowResponse | undefined;
  isError: boolean;
  isFetching: boolean;
  isPlaceholderData: boolean;
  refetch: jest.Mock;
};

jest.mock('@tanstack/react-query', () => ({ useQuery: () => mockQueryState }));
jest.mock('@visx/responsive', () => ({
  useParentSize: () => ({ width: 600, parentRef: null }),
}));

const day = 86_400_000;
const end = Date.UTC(2026, 8, 22, 12);
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
const data: MarketFundFlowResponse = {
  venue,
  venues: [venue],
  window: '1d',
  windows: ['15m', '30m', '1h', '2h', '4h', '1d', '7d', '30d'],
  windowStart: end - day,
  windowEnd: end,
  seriesStepMs: 3_600_000,
  bands: { mediumFrom: 10, largeFrom: 100, quoteAsset: 'USDT' },
  buckets: [],
  totals,
  series: [{ time: end - day, buyQuantity: 200, sellQuantity: 100, netQuantity: 100 }],
  daily: [{ time: end - day, buyQuantity: 200, sellQuantity: 100, netQuantity: 100 }],
  dailyDays: 5,
  dailyStart: end - 5 * day,
  dailyEnd: end,
  coverage: {
    firstTradeAt: end - 10 * day,
    lastTradeAt: end - 60_000,
    tradeCount: 50,
    complete: true,
  },
};

beforeEach(() => {
  mockQueryState = {
    data,
    isError: false,
    isFetching: false,
    isPlaceholderData: false,
    refetch: jest.fn(),
  };
});

describe('fund flow analysis', () => {
  it('offers longer timeframes and describes execution-derived volume in plain terms', () => {
    const html = renderToStaticMarkup(<FundFlowPanel />);
    expect(html).toContain('Analysis timeframe');
    expect(html).toContain('>24H</button>');
    expect(html).toContain('>7D</button>');
    expect(html).toContain('>30D</button>');
    expect(html).toContain('Net buy volume, 24H');
    expect(html).toContain('taker buy volume minus taker sell volume');
    expect(html).toContain('Deposits and withdrawals are excluded');
    expect(html).toContain('collection gaps may exist');
    expect(html).not.toMatch(/net inflow/i);
  });

  it('uses declared long daily ranges and date-aware accessible values', () => {
    mockQueryState.data = {
      ...data,
      window: '30d',
      windowStart: end - 30 * day,
      seriesStepMs: day,
      dailyDays: 30,
      dailyStart: end - 30 * day,
    };
    const html = renderToStaticMarkup(<FundFlowPanel />);
    expect(html).toContain('Daily net buy volume, last 30 days');
    expect(html).toContain('Net QRL buy volume per day over 30D');
    expect(html).toContain('2026-09-21 12:00 UTC');
    expect(html).toContain('Partial history:');
  });

  it('identifies an empty selected window even when earlier history exists', () => {
    mockQueryState.data = {
      ...data,
      totals: { ...totals, buyTradeCount: 0, sellTradeCount: 0 },
      series: [],
    };
    const html = renderToStaticMarkup(<FundFlowPanel />);
    expect(html).toContain('No trades recorded for this period');
    expect(html).not.toContain('aria-label="Net QRL buy volume per hour over 24H"');
    expect(html).toContain('aria-label="Daily net QRL buy volume over the last 5 UTC days"');
  });

  it('keeps existing data visible when a refresh fails', () => {
    mockQueryState.isError = true;
    const html = renderToStaticMarkup(<FundFlowPanel />);
    expect(html).toContain('Refresh failed. Showing the last loaded data.');
    expect(html).toContain('Net buy volume, 24H');
  });

  it('provides a retry when initial data is unavailable', () => {
    mockQueryState.data = undefined;
    mockQueryState.isError = true;
    const html = renderToStaticMarkup(<FundFlowPanel />);
    expect(html).toContain('Fund flow unavailable');
    expect(html).toContain('Retry');
  });

  it('keeps a loading state and supports older responses without daily metadata', () => {
    mockQueryState.data = undefined;
    expect(renderToStaticMarkup(<FundFlowPanel />)).toContain('skeleton');
    mockQueryState.data = {
      ...data,
      dailyDays: undefined,
      dailyStart: undefined,
      dailyEnd: undefined,
    };
    expect(renderToStaticMarkup(<FundFlowPanel />)).toContain('Daily net buy volume, last 1 days');
  });
});
