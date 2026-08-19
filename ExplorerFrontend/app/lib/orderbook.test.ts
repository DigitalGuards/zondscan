import {
  buildHistoricalPlays,
  formatQrlQuantity,
  formatQrlTradeSize,
  buildGroupedLadder,
  buildLadder,
  calculateBookStats,
  calculateYardValue,
  markerScale,
  PRICE_GROUPINGS,
  priceDeltaToYards,
  type MarketOrderBookResponse,
} from './orderbook';

const fixture: MarketOrderBookResponse = {
  venue: 'MEXC',
  symbol: 'QRLUSDT',
  fetchedAt: '2026-08-09T12:00:00Z',
  lastUpdateId: 42,
  bids: [
    { price: '0.74', quantity: '20' },
    { price: '0.75', quantity: '10' },
    { price: 'bad', quantity: '5' },
    { price: '0.73', quantity: '0' },
  ],
  asks: [
    { price: '0.78', quantity: '30' },
    { price: '0.77', quantity: '15' },
  ],
  recentTrades: [],
  ticker: {
    high: '0.82',
    low: '0.70',
    last: '0.76',
    change: '0.01',
    changePercent: '1.3',
    baseVolume: '1200',
    quoteVolume: '910',
  },
};

describe('order-book calculations', () => {
  it('sorts, filters, and accumulates ladder levels', () => {
    const bids = buildLadder(fixture.bids, 'buy');
    expect(bids.map((level) => level.price)).toEqual([0.75, 0.74]);
    expect(bids.map((level) => level.cumulativeQuantity)).toEqual([10, 30]);

    const asks = buildLadder(fixture.asks, 'sell');
    expect(asks.map((level) => level.price)).toEqual([0.77, 0.78]);
    expect(asks[0].notional).toBeCloseTo(11.55);
  });

  it('offers the same broad price groupings as the exchange ladder', () => {
    expect(PRICE_GROUPINGS).toEqual(['0.00001', '0.0001', '0.001', '0.01']);
  });

  it('rounds asks up and aggregates their quantity and original notional', () => {
    const asks = buildGroupedLadder(
      [
        { price: '0.77644', quantity: '2' },
        { price: '0.77996', quantity: '3' },
        { price: '0.78001', quantity: '4' },
      ],
      'sell',
      '0.01',
    );

    expect(asks.map((level) => level.raw.price)).toEqual(['0.78', '0.79']);
    expect(asks.map((level) => level.quantity)).toEqual([5, 4]);
    expect(asks[0].notional).toBeCloseTo(0.77644 * 2 + 0.77996 * 3);
    expect(asks.map((level) => level.cumulativeQuantity)).toEqual([5, 9]);
  });

  it('rounds bids down using decimal-safe buckets', () => {
    const bids = buildGroupedLadder(
      [
        { price: '0.76936', quantity: '4' },
        { price: '0.76200', quantity: '6' },
        { price: '0.75999', quantity: '2' },
        { price: '0.699999999999999999', quantity: '1' },
      ],
      'buy',
      '0.01',
    );

    expect(bids.map((level) => level.raw.price)).toEqual(['0.76', '0.75', '0.69']);
    expect(bids[0].quantity).toBe(10);
  });

  it('groups the complete snapshot before applying the visible limit', () => {
    const bids = buildGroupedLadder(
      [
        { price: '0.744', quantity: '2' },
        { price: '0.749', quantity: '3' },
        { price: '0.739', quantity: '7' },
      ],
      'buy',
      '0.01',
      1,
    );

    expect(bids).toHaveLength(1);
    expect(bids[0].raw.price).toBe('0.74');
    expect(bids[0].quantity).toBe(5);
  });

  it('derives midpoint, spread, and symmetric-band depth', () => {
    const stats = calculateBookStats(fixture, 1.5);
    expect(stats.bestBid).toBe(0.75);
    expect(stats.bestAsk).toBe(0.77);
    expect(stats.midpoint).toBeCloseTo(0.76);
    expect(stats.spread).toBeCloseTo(0.02);
    expect(stats.spreadBps).toBeCloseTo(263.1578, 3);
    expect(stats.bidQuantityInBand).toBe(10);
    expect(stats.askQuantityInBand).toBe(15);
  });

  it('selects a positive readable price-per-yard scale', () => {
    const stats = calculateBookStats(fixture);
    const yardValue = calculateYardValue(stats);
    expect(yardValue).toBeGreaterThan(0);
    expect(priceDeltaToYards(yardValue * 4, yardValue)).toBeCloseTo(4);
    expect(priceDeltaToYards(-yardValue * 2, yardValue)).toBeCloseTo(-2);
  });

  it('uses marker area scaling without letting large levels dominate', () => {
    const small = markerScale(1, [1, 100, 10_000]);
    const medium = markerScale(100, [1, 100, 10_000]);
    const large = markerScale(10_000, [1, 100, 10_000]);
    expect(small).toBeLessThan(medium);
    expect(medium).toBeLessThan(large);
    expect(large).toBeLessThanOrEqual(1.44);
  });

  it('batches micro executions into timed plays', () => {
    const plays = buildHistoricalPlays(
      [
        { id: '1', price: '0.7500', quantity: '1', quoteQuantity: '0.75', time: 1_000, aggressorSide: 'buy' },
        { id: '2', price: '0.7501', quantity: '2', quoteQuantity: '1.50', time: 5_000, aggressorSide: 'buy' },
        { id: '3', price: '0.7498', quantity: '3', quoteQuantity: '2.25', time: 45_000, aggressorSide: 'sell' },
      ],
      0.0001,
      30_000,
    );

    expect(plays).toHaveLength(2);
    expect(plays[0].direction).toBe('bears');
    expect(plays[0].yards).toBeCloseTo(-3);
    expect(plays[1].quantity).toBe(3);
    expect(plays[1].executionCount).toBe(2);
  });
});

describe('formatQrlQuantity', () => {
  it('renders a whole number at every size a bucket realistically takes', () => {
    // Reported from the live ladder: one sub-100 bucket grew two decimals
    // while its neighbours stayed whole, so the column read as broken.
    expect(formatQrlQuantity(13.93)).toBe('14');
    expect(formatQrlQuantity(92.06)).toBe('92');
    expect(formatQrlQuantity(377)).toBe('377');
    expect(formatQrlQuantity(1152.4)).toBe('1,152');
    expect(formatQrlQuantity(13454)).toBe('13,454');
  });

  it('formats a whole column at one precision', () => {
    const column = [13.93, 92.06, 377, 1152.4, 13454].map(formatQrlQuantity);
    for (const cell of column) {
      expect(cell).not.toContain('.');
    }
  });

  it('keeps the fraction below 1 QRL, where it is the only information there is', () => {
    expect(formatQrlQuantity(0.5)).toBe('0.5');
    expect(formatQrlQuantity(0.42)).toBe('0.42');
    expect(formatQrlQuantity(1)).toBe('1');
  });

  it('labels dust rather than rounding it to a misleading zero', () => {
    expect(formatQrlQuantity(0.004)).toBe('<0.01');
    expect(formatQrlQuantity(0)).toBe('0');
  });

  it('degrades to an ellipsis on a non-finite value', () => {
    expect(formatQrlQuantity(Number.NaN)).toBe('...');
    expect(formatQrlQuantity(Number.POSITIVE_INFINITY)).toBe('...');
  });
});

describe('formatQrlTradeSize', () => {
  it('keeps two decimals on an individual print, where the fraction is real', () => {
    // Rounding one execution to a whole number misstates it: 7.75 is not 8.
    expect(formatQrlTradeSize(7.75)).toBe('7.75');
    expect(formatQrlTradeSize(41.88)).toBe('41.88');
  });

  it('holds the same precision at every magnitude so the column is one shape', () => {
    const column = [7.75, 41.88, 250, 1195.5].map(formatQrlTradeSize);
    expect(column).toEqual(['7.75', '41.88', '250.00', '1,195.50']);
  });

  it('degrades to an ellipsis on a non-finite value', () => {
    expect(formatQrlTradeSize(Number.NaN)).toBe('...');
  });
});
