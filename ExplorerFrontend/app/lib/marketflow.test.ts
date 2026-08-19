import {
  axisLabelPlacement,
  columnLabelPlacement,
  coverageNotice,
  flowBaselineY,
  flowColumn,
  flowScale,
  flowTicks,
  niceFlowScale,
  formatBandRange,
  formatCompactQuantity,
  formatSignedQuantity,
  formatWindowLabel,
  orderBuckets,
  type MarketFlowBucket,
  type MarketFlowCoverage,
} from './marketflow';

describe('flowScale', () => {
  it('always contains zero so a column direction cannot mislead', () => {
    expect(flowScale([5, 10, 20])).toEqual({ min: 0, max: 20 });
    expect(flowScale([-5, -10])).toEqual({ min: -10, max: 0 });
    expect(flowScale([-4, 9])).toEqual({ min: -4, max: 9 });
  });

  it('falls back to a symmetric domain when everything is zero', () => {
    expect(flowScale([])).toEqual({ min: -1, max: 1 });
    expect(flowScale([0, 0])).toEqual({ min: -1, max: 1 });
  });

  it('ignores non-finite values rather than collapsing the domain', () => {
    expect(flowScale([Number.NaN, 5, Number.POSITIVE_INFINITY])).toEqual({ min: 0, max: 5 });
  });
});

describe('flowColumn', () => {
  const scale = { min: -100, max: 100 };

  it('puts the baseline where zero falls in the domain', () => {
    expect(flowBaselineY(scale, 200)).toBe(100);
    expect(flowBaselineY({ min: 0, max: 50 }, 200)).toBe(200);
    expect(flowBaselineY({ min: -50, max: 0 }, 200)).toBe(0);
  });

  it('grows a positive value up from the baseline and a negative value down', () => {
    const positive = flowColumn(50, scale, 200);
    expect(positive.height).toBe(50);
    expect(positive.y).toBe(50);
    expect(positive.y + positive.height).toBe(flowBaselineY(scale, 200));

    const negative = flowColumn(-50, scale, 200);
    expect(negative.height).toBe(50);
    expect(negative.y).toBe(flowBaselineY(scale, 200));
  });

  it('renders a zero value as a flat mark on the baseline', () => {
    const zero = flowColumn(0, scale, 200);
    expect(zero.height).toBe(0);
    expect(zero.y).toBe(100);
  });

  it('keeps equal magnitudes the same length in both directions', () => {
    const up = flowColumn(30, { min: -10, max: 90 }, 100);
    const down = flowColumn(-30, { min: -10, max: 90 }, 100);
    expect(up.height).toBeCloseTo(down.height);
  });
});

describe('flowTicks', () => {
  it('rounds the extremes and always includes zero', () => {
    expect(flowTicks({ min: -1234, max: 4321 })).toEqual([5000, 0, -2000]);
  });

  it('does not duplicate zero when a side is empty', () => {
    expect(flowTicks({ min: 0, max: 800 })).toEqual([1000, 0]);
    expect(flowTicks({ min: -800, max: 0 })).toEqual([0, -1000]);
  });
});

describe('niceFlowScale', () => {
  it('rounds the domain outward to the numbers the axis prints', () => {
    expect(niceFlowScale([1234, -400])).toEqual({ min: -500, max: 2000 });
    expect(niceFlowScale([-1104])).toEqual({ min: -2000, max: 0 });
  });

  it('is idempotent, so an already-round domain is left alone', () => {
    const once = niceFlowScale([1234, -400]);
    expect(niceFlowScale([once.min, once.max])).toEqual(once);
  });

  it('keeps every tick inside the plot', () => {
    // The regression: ticks rounded outward from a raw domain landed below
    // the plot and their gridlines drew over the x-axis labels.
    const plotHeight = 182;
    const samples = [[1234, -400], [-1104], [500], [0, 0], [12, -9000]];
    for (const values of samples) {
      const scale = niceFlowScale(values);
      for (const tick of flowTicks(scale)) {
        const column = flowColumn(tick, scale, plotHeight);
        const y = tick >= 0 ? column.y : column.y + column.height;
        expect(y).toBeGreaterThanOrEqual(0);
        expect(y).toBeLessThanOrEqual(plotHeight);
      }
    }
  });
});

describe('axisLabelPlacement', () => {
  const plotWidth = 340;

  it('keeps a label centred on its own band when it fits', () => {
    // Five wide bands: every label has room, so none should be pulled to an
    // edge. Anchoring by index instead detaches the first and last labels
    // from the columns they name.
    const bandWidth = plotWidth / 5;
    expect(axisLabelPlacement(bandWidth / 2, 6, plotWidth)).toEqual({
      x: bandWidth / 2,
      anchor: 'middle',
    });
    expect(axisLabelPlacement(plotWidth - bandWidth / 2, 6, plotWidth)).toEqual({
      x: plotWidth - bandWidth / 2,
      anchor: 'middle',
    });
  });

  it('anchors to the edge only when the centred text would spill out', () => {
    // 24 narrow bands: the first band's centre is a few pixels in, so a
    // centred label would run back over the y-axis ticks.
    const bandWidth = plotWidth / 24;
    expect(axisLabelPlacement(bandWidth / 2, 5, plotWidth)).toEqual({ x: 0, anchor: 'start' });
    expect(axisLabelPlacement(plotWidth - bandWidth / 2, 5, plotWidth)).toEqual({
      x: plotWidth,
      anchor: 'end',
    });
  });

  it('treats a longer label as needing more room', () => {
    expect(axisLabelPlacement(20, 4, plotWidth).anchor).toBe('middle');
    expect(axisLabelPlacement(20, 12, plotWidth).anchor).toBe('start');
  });
});

describe('columnLabelPlacement', () => {
  const plotHeight = 182;

  it('sits just beyond the data end when there is room', () => {
    const placement = columnLabelPlacement({ y: 80, height: 20 }, true, plotHeight);
    expect(placement).toEqual({ y: 74, inside: false, visible: true });
  });

  it('moves inside when the column reaches the bottom of the plot', () => {
    // A full-height negative column leaves no room below: a label there
    // lands in the axis band and collides with the tick labels.
    const placement = columnLabelPlacement({ y: 0, height: plotHeight }, false, plotHeight);
    expect(placement.inside).toBe(true);
    expect(placement.visible).toBe(true);
    expect(placement.y).toBeLessThan(plotHeight);
  });

  it('moves inside when a positive column reaches the top', () => {
    const placement = columnLabelPlacement({ y: 2, height: 180 }, true, plotHeight);
    expect(placement.inside).toBe(true);
    expect(placement.y).toBeGreaterThan(2);
  });

  it('keeps the label outside a short column that has room below it', () => {
    const placement = columnLabelPlacement({ y: 0, height: 4 }, false, plotHeight);
    expect(placement).toEqual({ y: 16, inside: false, visible: true });
  });

  it('drops the label when the column reaches the edge and is too short to hold it', () => {
    const placement = columnLabelPlacement(
      { y: plotHeight - 4, height: 4 },
      false,
      plotHeight,
    );
    expect(placement.visible).toBe(false);
  });
});

describe('quantity formatting', () => {
  it('compacts by magnitude', () => {
    expect(formatCompactQuantity(0)).toBe('0');
    expect(formatCompactQuantity(4.25)).toBe('4.3');
    expect(formatCompactQuantity(320)).toBe('320');
    expect(formatCompactQuantity(9532)).toBe('9.53K');
    expect(formatCompactQuantity(2_500_000)).toBe('2.50M');
  });

  it('carries an explicit sign so polarity survives without colour', () => {
    expect(formatSignedQuantity(1001.4)).toBe('+1.00K');
    expect(formatSignedQuantity(-4297)).toBe('-4.30K');
    expect(formatSignedQuantity(0)).toBe('0');
  });

  it('labels windows without touching the minute unit', () => {
    expect(formatWindowLabel('15m')).toBe('15m');
    expect(formatWindowLabel('1h')).toBe('1H');
    expect(formatWindowLabel('1d')).toBe('1D');
  });
});

describe('formatBandRange', () => {
  const bands = { mediumFrom: 10, largeFrom: 100, quoteAsset: 'USDT' };

  it('states our own thresholds outright', () => {
    expect(formatBandRange('large', bands)).toBe('>= 100 USDT');
    expect(formatBandRange('medium', bands)).toBe('10.0 to 100 USDT');
    expect(formatBandRange('small', bands)).toBe('< 10.0 USDT');
  });
});

describe('orderBuckets', () => {
  const bucket = (name: string): MarketFlowBucket => ({
    bucket: name,
    buyQuantity: 1,
    sellQuantity: 1,
    netQuantity: 0,
    buyQuote: 1,
    sellQuote: 1,
    netQuote: 0,
    buyTradeCount: 1,
    sellTradeCount: 1,
  });

  it('orders largest first regardless of API order', () => {
    const ordered = orderBuckets([bucket('small'), bucket('large'), bucket('medium')]);
    expect(ordered.map((row) => row.bucket)).toEqual(['large', 'medium', 'small']);
  });

  it('zero-fills a band the API omitted so the table keeps its shape', () => {
    const ordered = orderBuckets([bucket('large')]);
    expect(ordered).toHaveLength(3);
    expect(ordered[2]).toMatchObject({ bucket: 'small', buyQuantity: 0, sellQuantity: 0 });
  });
});

describe('coverageNotice', () => {
  const coverage = (overrides: Partial<MarketFlowCoverage>): MarketFlowCoverage => ({
    firstTradeAt: 1_000,
    lastTradeAt: 5_000,
    tradeCount: 10,
    complete: true,
    ...overrides,
  });

  it('says nothing when the window is fully covered', () => {
    expect(coverageNotice(coverage({}), 2_000)).toBeNull();
  });

  it('explains an empty store rather than implying zero flow', () => {
    const notice = coverageNotice(coverage({ tradeCount: 0, firstTradeAt: null }), 0);
    expect(notice).toMatch(/cannot be backfilled/);
  });

  it('flags a partially covered window', () => {
    const notice = coverageNotice(
      coverage({ complete: false, firstTradeAt: Date.UTC(2026, 7, 15, 9, 30) }),
      Date.UTC(2026, 7, 14),
    );
    expect(notice).toMatch(/^Partial window/);
    expect(notice).toContain('15 Aug');
  });
});
