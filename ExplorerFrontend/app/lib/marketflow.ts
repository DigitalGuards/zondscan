/**
 * Types and pure helpers for the venue fund-flow panel.
 *
 * Net flow is signed base-asset quantity: buys positive, sells negative. The
 * API can never backfill (venues expose no historical trade tape), so every
 * response carries coverage and the UI must show it rather than let a partly
 * covered window read as a complete one.
 */

export type SizeBand = 'large' | 'medium' | 'small';

export const SIZE_BANDS: SizeBand[] = ['large', 'medium', 'small'];

export interface MarketFlowBucket {
  bucket: string;
  buyQuantity: number;
  sellQuantity: number;
  netQuantity: number;
  buyQuote: number;
  sellQuote: number;
  netQuote: number;
  buyTradeCount: number;
  sellTradeCount: number;
}

export interface MarketFlowPoint {
  time: number;
  buyQuantity: number;
  sellQuantity: number;
  netQuantity: number;
}

export interface MarketFlowCoverage {
  firstTradeAt: number | null;
  lastTradeAt: number | null;
  tradeCount: number;
  complete: boolean;
}

export interface MarketVenueInfo {
  id: string;
  name: string;
  symbol: string;
  quoteAsset: string;
}

export interface MarketSizeBands {
  mediumFrom: number;
  largeFrom: number;
  quoteAsset: string;
}

export interface MarketFundFlowResponse {
  venue: MarketVenueInfo;
  venues: MarketVenueInfo[];
  window: string;
  windows: string[];
  windowStart: number;
  windowEnd: number;
  seriesStepMs: number;
  bands: MarketSizeBands;
  buckets: MarketFlowBucket[];
  totals: MarketFlowBucket;
  series: MarketFlowPoint[];
  daily: MarketFlowPoint[];
  coverage: MarketFlowCoverage;
}

/** Vertical extent of a diverging chart. Zero is always inside the domain, so
 *  a column's direction from the baseline is a truthful sign indicator. */
export interface FlowScale {
  min: number;
  max: number;
}

export interface FlowColumn {
  y: number;
  height: number;
}

const finite = (value: number): number => (Number.isFinite(value) ? value : 0);

/**
 * Builds the vertical domain for a diverging column chart. Zero is forced
 * into the domain: a truncated axis would let a small negative bar sit above
 * the baseline and invert the reader's conclusion.
 */
export function flowScale(values: number[]): FlowScale {
  const usable = values.map(finite);
  const max = Math.max(0, ...usable);
  const min = Math.min(0, ...usable);
  if (max === 0 && min === 0) return { min: -1, max: 1 };
  return { min, max };
}

/** Y offset of the zero baseline within a plot of the given height. */
export function flowBaselineY(scale: FlowScale, plotHeight: number): number {
  const span = scale.max - scale.min;
  if (span <= 0) return plotHeight / 2;
  return (scale.max / span) * plotHeight;
}

/**
 * Geometry of one column, measured from the top of the plot. A positive
 * value grows upward from the baseline and a negative value downward, which
 * is the secondary encoding that keeps the chart readable when the
 * green/red pair is hard to tell apart (it sits in the 6-8 CVD floor band).
 */
export function flowColumn(value: number, scale: FlowScale, plotHeight: number): FlowColumn {
  const baseline = flowBaselineY(scale, plotHeight);
  const span = scale.max - scale.min;
  if (span <= 0) return { y: baseline, height: 0 };
  const magnitude = (Math.abs(finite(value)) / span) * plotHeight;
  if (finite(value) >= 0) return { y: baseline - magnitude, height: magnitude };
  return { y: baseline, height: magnitude };
}

/**
 * Path for one column: rounded on the data end, square where it meets the
 * baseline, so the mark reads as growing out of the axis. The radius
 * collapses on short columns, otherwise a near-zero value would render as a
 * lozenge that overstates it.
 */
export function columnPath(
  x: number,
  width: number,
  y: number,
  height: number,
  positive: boolean,
  cornerRadius = 4,
): string {
  const radius = Math.max(0, Math.min(cornerRadius, width / 2, height));
  const top = y;
  const bottom = y + height;
  if (positive) {
    return [
      `M ${x} ${bottom}`,
      `L ${x} ${top + radius}`,
      `Q ${x} ${top} ${x + radius} ${top}`,
      `L ${x + width - radius} ${top}`,
      `Q ${x + width} ${top} ${x + width} ${top + radius}`,
      `L ${x + width} ${bottom}`,
      'Z',
    ].join(' ');
  }
  return [
    `M ${x} ${top}`,
    `L ${x} ${bottom - radius}`,
    `Q ${x} ${bottom} ${x + radius} ${bottom}`,
    `L ${x + width - radius} ${bottom}`,
    `Q ${x + width} ${bottom} ${x + width} ${bottom - radius}`,
    `L ${x + width} ${top}`,
    'Z',
  ].join(' ');
}

function niceCeil(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 0;
  const magnitude = 10 ** Math.floor(Math.log10(value));
  const normalized = value / magnitude;
  let nice = 1;
  if (normalized > 5) nice = 10;
  else if (normalized > 2) nice = 5;
  else if (normalized > 1) nice = 2;
  return nice * magnitude;
}

/**
 * Expands a domain to round bounds.
 *
 * The plot must be scaled to the same rounded numbers the axis prints,
 * otherwise a tick rounded outward from the data lands outside the plot and
 * its gridline draws over the axis labels below the chart.
 */
export function niceFlowScale(values: number[]): FlowScale {
  const raw = flowScale(values);
  const max = niceCeil(raw.max);
  const min = -niceCeil(Math.abs(raw.min));
  if (max === 0 && min === 0) return { min: -1, max: 1 };
  return { min, max };
}

/**
 * Axis ticks for a diverging chart: the rounded extremes plus zero. Ticks
 * carry the values that are not directly labelled, so they round to numbers
 * a reader can hold in their head. Pass a domain from niceFlowScale so every
 * tick falls inside the plot.
 */
export function flowTicks(scale: FlowScale): number[] {
  const top = niceCeil(scale.max);
  const bottom = -niceCeil(Math.abs(scale.min));
  const ticks = [top, 0, bottom].filter(
    (value, index, all) => all.indexOf(value) === index,
  );
  return ticks.sort((a, b) => b - a);
}

export interface AxisLabelPlacement {
  x: number;
  anchor: 'start' | 'middle' | 'end';
}

/** Approximate half-width of a monospace label, for overflow tests. */
const MONO_CHAR_WIDTH = 6;

/**
 * Places an axis label under its band.
 *
 * The label stays centred on its own band, which is what ties it to its
 * column, and only shifts to an edge anchor when the centred text would
 * actually spill outside the plot. Anchoring by index instead detaches the
 * first and last labels from their columns whenever bands are wide, which is
 * every low-count chart such as the five daily bars.
 */
export function axisLabelPlacement(
  centerX: number,
  labelLength: number,
  plotWidth: number,
  charWidth = MONO_CHAR_WIDTH,
): AxisLabelPlacement {
  const halfText = (labelLength * charWidth) / 2;
  if (centerX - halfText < 0) return { x: 0, anchor: 'start' };
  if (centerX + halfText > plotWidth) return { x: plotWidth, anchor: 'end' };
  return { x: centerX, anchor: 'middle' };
}

export interface ColumnLabelPlacement {
  y: number;
  /** True when the label sits on the fill and needs ink that clears it. */
  inside: boolean;
  visible: boolean;
}

/**
 * Places a direct label on a column without letting it escape the plot.
 *
 * The natural position is just beyond the data end. A column that reaches
 * the top or bottom of the plot leaves no room there, and a label drawn
 * anyway lands in the axis band and collides with the tick labels. In that
 * case the label moves inside the column, and when the column is too short
 * to hold it the label is dropped: the value stays reachable through the
 * tooltip and the table twin.
 */
export function columnLabelPlacement(
  column: FlowColumn,
  positive: boolean,
  plotHeight: number,
): ColumnLabelPlacement {
  const outside = positive ? column.y - 6 : column.y + column.height + 12;
  if (outside >= 10 && outside <= plotHeight - 2) {
    return { y: outside, inside: false, visible: true };
  }
  const inside = positive ? column.y + 13 : column.y + column.height - 5;
  return { y: inside, inside: true, visible: column.height >= 18 };
}

/** Compact quantity for axis ticks and column labels: 1.2K, 34.5K, 1.1M. */
export function formatCompactQuantity(value: number): string {
  const safe = finite(value);
  const magnitude = Math.abs(safe);
  if (magnitude >= 1_000_000) return `${(safe / 1_000_000).toFixed(2)}M`;
  if (magnitude >= 1_000) return `${(safe / 1_000).toFixed(2)}K`;
  if (magnitude >= 100) return safe.toFixed(0);
  if (magnitude >= 1) return safe.toFixed(1);
  if (magnitude === 0) return '0';
  return safe.toFixed(2);
}

/**
 * Signed compact quantity. The explicit sign is a second, non-colour signal
 * of direction, so polarity survives for a reader who cannot separate the
 * green and red marks.
 */
export function formatSignedQuantity(value: number): string {
  const safe = finite(value);
  if (safe === 0) return '0';
  const sign = safe > 0 ? '+' : '-';
  return `${sign}${formatCompactQuantity(Math.abs(safe))}`;
}

/** Human label for a window id, e.g. "1h" renders as "1H". */
export function formatWindowLabel(window: string): string {
  return window.replace(/h$/, 'H').replace(/d$/, 'D');
}

/** Title-case band name for table rows and legends. */
export function formatBandLabel(bucket: string): string {
  return bucket.charAt(0).toUpperCase() + bucket.slice(1);
}

/**
 * Describes what a band covers in quote terms. These thresholds are
 * ZondScan's own, so the UI states them outright rather than implying the
 * venue publishes this split.
 */
export function formatBandRange(bucket: string, bands: MarketSizeBands): string {
  const quote = bands.quoteAsset;
  switch (bucket) {
    case 'large':
      return `>= ${formatCompactQuantity(bands.largeFrom)} ${quote}`;
    case 'medium':
      return `${formatCompactQuantity(bands.mediumFrom)} to ${formatCompactQuantity(bands.largeFrom)} ${quote}`;
    case 'small':
      return `< ${formatCompactQuantity(bands.mediumFrom)} ${quote}`;
    default:
      return '';
  }
}

/** Orders the API's bands largest first and zero-fills any the API omitted. */
export function orderBuckets(buckets: MarketFlowBucket[]): MarketFlowBucket[] {
  const byBucket = new Map(buckets.map((bucket) => [bucket.bucket, bucket]));
  return SIZE_BANDS.map(
    (band) =>
      byBucket.get(band) ?? {
        bucket: band,
        buyQuantity: 0,
        sellQuantity: 0,
        netQuantity: 0,
        buyQuote: 0,
        sellQuote: 0,
        netQuote: 0,
        buyTradeCount: 0,
        sellTradeCount: 0,
      },
  );
}

const timeFormatter = new Intl.DateTimeFormat('en-GB', {
  hour: '2-digit',
  minute: '2-digit',
  timeZone: 'UTC',
});

const dayFormatter = new Intl.DateTimeFormat('en-GB', {
  day: '2-digit',
  month: 'short',
  timeZone: 'UTC',
});

export function formatFlowTime(time: number): string {
  return timeFormatter.format(new Date(time));
}

export function formatFlowDay(time: number): string {
  return dayFormatter.format(new Date(time));
}

/**
 * Sentence explaining how much of a window is actually backed by collected
 * data. Returns null when the window is fully covered and nothing needs
 * saying.
 */
export function coverageNotice(
  coverage: MarketFlowCoverage,
  windowStart: number,
): string | null {
  if (coverage.tradeCount === 0 || coverage.firstTradeAt === null) {
    return 'No collected trades yet. Flow history begins when collection starts and cannot be backfilled.';
  }
  if (coverage.complete || coverage.firstTradeAt <= windowStart) return null;
  return `Partial window: collection began ${formatFlowDay(coverage.firstTradeAt)} ${formatFlowTime(
    coverage.firstTradeAt,
  )} UTC, so earlier flow in this window is not counted.`;
}
