'use client';

import { useMemo, useState } from 'react';
import { useParentSize } from '@visx/responsive';
import axios from 'axios';
import { useQuery } from '@tanstack/react-query';
import config from '../../config';
import { palette } from '../lib/theme';
import {
  axisLabelPlacement,
  columnLabelPlacement,
  columnPath,
  coverageNotice,
  flowBaselineY,
  flowColumn,
  flowTicks,
  niceFlowScale,
  formatBandLabel,
  formatBandRange,
  formatCompactQuantity,
  formatFlowDay,
  formatFlowTime,
  formatSignedQuantity,
  formatWindowLabel,
  orderBuckets,
  type FlowScale,
  type MarketFlowPoint,
  type MarketFundFlowResponse,
} from '../lib/marketflow';

// The chart renders in pixel space: the SVG viewport and its viewBox are the
// measured width, so one unit is one pixel. A fixed viewBox scaled by w-full
// instead ties height AND type size to the container's width, which made the
// two charts render at different heights and different label sizes purely
// because one sits in a half-width grid column.
const CHART_HEIGHT = 220;
const PADDING = { top: 16, right: 12, bottom: 22, left: 46 };
const MIN_PLOT_WIDTH = 120;
const PLOT_HEIGHT = CHART_HEIGHT - PADDING.top - PADDING.bottom;

// Marks stay thin and the band's leftover width is air, so columns never
// fuse into a solid block at low point counts.
const MAX_BAR_WIDTH = 24;
const BAR_GAP = 2;
const CORNER_RADIUS = 4;

const GRID_COLOR = 'rgba(255, 255, 255, 0.07)';
const ZERO_LINE_COLOR = 'rgba(255, 255, 255, 0.2)';

interface ChartPoint {
  time: number;
  value: number;
  label: string;
}

function DivergingColumnChart({
  points,
  title,
  emphasise,
  valueUnit,
}: {
  points: ChartPoint[];
  title: string;
  /** Indices to direct-label. Labelling every column reads as noise, so the
   *  caller picks the few that carry the story. */
  emphasise: number[];
  valueUnit: string;
}): JSX.Element {
  const [active, setActive] = useState<number | null>(null);
  const { parentRef, width } = useParentSize({ debounceTime: 40 });

  const chartWidth = Math.max(MIN_PLOT_WIDTH + PADDING.left + PADDING.right, width);
  const plotWidth = chartWidth - PADDING.left - PADDING.right;

  const scale: FlowScale = useMemo(
    () => niceFlowScale(points.map((point) => point.value)),
    [points],
  );
  const baseline = flowBaselineY(scale, PLOT_HEIGHT);
  const ticks = flowTicks(scale);
  const bandWidth = points.length > 0 ? plotWidth / points.length : plotWidth;
  const barWidth = Math.max(3, Math.min(MAX_BAR_WIDTH, bandWidth - BAR_GAP));
  const labelEvery = points.length <= 6 ? 1 : Math.ceil(points.length / 6);
  const emphasised = new Set(emphasise);
  const activePoint = active === null ? null : points[active];

  return (
    <div ref={parentRef} className="relative mt-2 w-full" style={{ height: CHART_HEIGHT }}>
      <svg
        width={chartWidth}
        height={CHART_HEIGHT}
        viewBox={`0 0 ${chartWidth} ${CHART_HEIGHT}`}
        role="group"
        aria-label={title}
      >
        <g transform={`translate(${PADDING.left} ${PADDING.top})`}>
          {ticks.map((tick) => {
            const y = flowColumn(tick, scale, PLOT_HEIGHT);
            const tickY = tick >= 0 ? y.y : y.y + y.height;
            return (
              <g key={tick}>
                <line
                  x1={0}
                  x2={plotWidth}
                  y1={tickY}
                  y2={tickY}
                  stroke={tick === 0 ? ZERO_LINE_COLOR : GRID_COLOR}
                  strokeWidth={1}
                />
                <text
                  x={-8}
                  y={tickY + 3}
                  textAnchor="end"
                  fill={palette.textMuted}
                  fontSize={10}
                  fontFamily="IBM Plex Mono, ui-monospace, monospace"
                >
                  {formatCompactQuantity(tick)}
                </text>
              </g>
            );
          })}

          {points.map((point, index) => {
            const column = flowColumn(point.value, scale, PLOT_HEIGHT);
            const x = index * bandWidth + (bandWidth - barWidth) / 2;
            const positive = point.value >= 0;
            const isActive = active === index;
            const describedValue = `${formatSignedQuantity(point.value)} ${valueUnit}`;
            return (
              <g
                key={point.time}
                role="img"
                aria-label={`${point.label}: net ${describedValue}`}
                tabIndex={0}
                className="cursor-help outline-none"
                onMouseEnter={() => setActive(index)}
                onMouseLeave={() => setActive((current) => (current === index ? null : current))}
                onFocus={() => setActive(index)}
                onBlur={() => setActive((current) => (current === index ? null : current))}
              >
                {/* Full-height hit area so the pointer target is far larger
                    than the mark, which can be only a few pixels tall. */}
                <rect
                  x={index * bandWidth}
                  y={0}
                  width={bandWidth}
                  height={PLOT_HEIGHT}
                  fill={isActive ? 'rgba(255,255,255,0.04)' : 'transparent'}
                />
                {column.height > 0.15 && (
                  <path
                    d={columnPath(x, barWidth, column.y, column.height, positive, CORNER_RADIUS)}
                    fill={positive ? palette.success : palette.error}
                    opacity={active === null || isActive ? 1 : 0.55}
                  />
                )}
                {emphasised.has(index) &&
                  (() => {
                    const placement = columnLabelPlacement(column, positive, PLOT_HEIGHT);
                    if (!placement.visible) return null;
                    return (
                      <text
                        x={x + barWidth / 2}
                        y={placement.y}
                        textAnchor="middle"
                        // Ink on the fill when the label sits inside the
                        // column, so it clears the mark's light hue.
                        fill={placement.inside ? palette.background : palette.textSecondary}
                        fontSize={10}
                        fontWeight={placement.inside ? 600 : 400}
                        fontFamily="IBM Plex Mono, ui-monospace, monospace"
                      >
                        {formatSignedQuantity(point.value)}
                      </text>
                    );
                  })()}
              </g>
            );
          })}

          {points.map((point, index) => {
            if (index % labelEvery !== 0) return null;
            const placement = axisLabelPlacement(
              index * bandWidth + bandWidth / 2,
              point.label.length,
              plotWidth,
            );
            return (
              <text
                key={`label:${point.time}`}
                x={placement.x}
                y={PLOT_HEIGHT + 14}
                textAnchor={placement.anchor}
                fill={palette.textMuted}
                fontSize={10}
                fontFamily="IBM Plex Mono, ui-monospace, monospace"
              >
                {point.label}
              </text>
            );
          })}
        </g>
      </svg>

      {activePoint && (
        <div
          className="pointer-events-none absolute z-10 -translate-x-1/2 -translate-y-full rounded-lg border border-border bg-background/95 px-2.5 py-1.5 text-xs shadow-lg backdrop-blur-sm"
          style={{
            // Pixel space, same as the SVG, so the tooltip tracks its column
            // exactly at any container width.
            left: PADDING.left + (active as number) * bandWidth + bandWidth / 2,
            top: PADDING.top + baseline,
          }}
          role="status"
        >
          <p className="font-mono text-[11px] text-text-muted">{activePoint.label} UTC</p>
          <p className="mt-0.5 font-mono text-text-primary">
            Net {formatSignedQuantity(activePoint.value)} {valueUnit}
          </p>
        </div>
      )}

      {/* Table twin: every plotted value stays reachable without relying on
          the hover layer or on telling the two mark colours apart. */}
      <table className="sr-only">
        <caption>{title}</caption>
        <thead>
          <tr>
            <th scope="col">Period</th>
            <th scope="col">Net flow ({valueUnit})</th>
          </tr>
        </thead>
        <tbody>
          {points.map((point) => (
            <tr key={`row:${point.time}`}>
              <th scope="row">{point.label}</th>
              <td>{formatSignedQuantity(point.value)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function extremeIndex(points: ChartPoint[]): number[] {
  if (points.length === 0) return [];
  let index = 0;
  for (let i = 1; i < points.length; i += 1) {
    if (Math.abs(points[i].value) > Math.abs(points[index].value)) index = i;
  }
  return Math.abs(points[index].value) > 0 ? [index] : [];
}

export default function FundFlowPanel(): JSX.Element {
  const [windowId, setWindowId] = useState('1d');
  const [venueId, setVenueId] = useState<string>('');

  const query = useQuery<MarketFundFlowResponse>({
    queryKey: ['market-fundflow', venueId || 'default', windowId],
    queryFn: async () => {
      const params = new URLSearchParams({ window: windowId });
      if (venueId) params.set('venue', venueId);
      const response = await axios.get<MarketFundFlowResponse>(
        `${config.handlerUrl}/market/fundflow?${params.toString()}`,
        { timeout: 8_000 },
      );
      return response.data;
    },
    refetchInterval: 30_000,
    refetchIntervalInBackground: false,
    staleTime: 15_000,
    // Hold the previous rollup while a new one loads: a skeleton on every
    // refetch would make the panel jump on a 30s cycle.
    placeholderData: (previous) => previous,
    retry: 2,
  });

  const data = query.data;

  const seriesPoints = useMemo<ChartPoint[]>(
    () =>
      (data?.series ?? []).map((point: MarketFlowPoint) => ({
        time: point.time,
        value: point.netQuantity,
        label: formatFlowTime(point.time),
      })),
    [data],
  );

  const dailyPoints = useMemo<ChartPoint[]>(
    () =>
      (data?.daily ?? []).map((point: MarketFlowPoint) => ({
        time: point.time,
        value: point.netQuantity,
        label: formatFlowDay(point.time),
      })),
    [data],
  );

  if (query.isError) {
    return (
      <section className="card p-6 text-center" aria-label="Fund flow">
        <h2 className="font-display text-xl font-semibold text-text-primary">Fund flow unavailable</h2>
        <p className="mt-2 text-sm text-text-secondary">
          ZondScan could not load the collected trade rollup.
        </p>
        <button
          type="button"
          onClick={() => void query.refetch()}
          className="mt-4 rounded-lg bg-accent px-4 py-2 text-sm font-semibold text-background hover:bg-accent-hover"
        >
          Retry
        </button>
      </section>
    );
  }

  if (!data) {
    return (
      <section className="card p-4" aria-label="Fund flow">
        <div className="h-5 w-40 skeleton rounded mb-3" />
        <div className="h-48 skeleton rounded-lg" />
      </section>
    );
  }

  const buckets = orderBuckets(data.buckets);
  const notice = coverageNotice(data.coverage, data.windowStart);
  const hasFlow = data.coverage.tradeCount > 0;
  const dailyEmphasis = (() => {
    const marks = new Set(extremeIndex(dailyPoints));
    if (dailyPoints.length > 0) marks.add(dailyPoints.length - 1);
    return [...marks];
  })();

  return (
    <section className="card overflow-hidden" aria-label="Fund flow analysis">
      {/* One filter row, above everything it scopes: both charts and the
          band table re-render against the same slice. */}
      <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border px-4 py-3">
        <div>
          <h2 className="font-display text-sm font-semibold text-text-primary">Fund flow analysis</h2>
          <p className="text-xs text-text-muted mt-0.5">
            {data.venue.name} {data.venue.symbol} executions grouped by trade size
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-2">
          {data.venues.length > 1 && (
            <label className="flex items-center gap-2 text-xs text-text-muted">
              <span className="hidden sm:inline">Venue</span>
              <select
                value={venueId || data.venue.id}
                onChange={(event) => setVenueId(event.target.value)}
                className="rounded-md border border-border bg-surface-2 px-2 py-1.5 text-xs text-text-primary outline-none focus:border-accent"
                aria-label="Trading venue"
              >
                {data.venues.map((venue) => (
                  <option key={venue.id} value={venue.id}>
                    {venue.name}
                  </option>
                ))}
              </select>
            </label>
          )}
          <div className="inline-flex rounded-lg border border-border bg-surface p-0.5" role="group" aria-label="Rollup window">
            {data.windows.map((option) => (
              <button
                key={option}
                type="button"
                onClick={() => setWindowId(option)}
                aria-pressed={option === data.window}
                className={`rounded-md px-2.5 py-1.5 text-[11px] font-medium transition-colors ${
                  option === data.window
                    ? 'bg-surface-3 text-text-primary'
                    : 'text-text-muted hover:text-text-primary'
                }`}
              >
                {formatWindowLabel(option)}
              </button>
            ))}
          </div>
        </div>
      </div>

      <div
        className={`transition-opacity ${query.isFetching ? 'opacity-60' : 'opacity-100'}`}
      >
        {notice && (
          <p className="border-b border-border bg-surface-2 px-4 py-2 text-xs text-text-secondary">
            {notice}
          </p>
        )}

        <div className="grid gap-4 p-4 xl:grid-cols-[minmax(0,1fr)_minmax(0,1.1fr)]">
          <div>
            <div className="flex items-baseline justify-between gap-3">
              <h3 className="eyebrow">Net inflow, {formatWindowLabel(data.window)}</h3>
              <p
                className={`num text-lg sm:text-xl font-medium ${
                  data.totals.netQuantity >= 0 ? 'text-success' : 'text-error'
                }`}
              >
                {formatSignedQuantity(data.totals.netQuantity)} QRL
              </p>
            </div>

            <table className="mt-3 w-full table-fixed">
              <thead>
                <tr className="text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted">
                  <th className="py-1.5 pr-2 text-left font-medium">Size band</th>
                  <th className="py-1.5 px-2 text-right font-medium">Buy</th>
                  <th className="py-1.5 px-2 text-right font-medium">Sell</th>
                  <th className="py-1.5 pl-2 text-right font-medium">Net</th>
                </tr>
              </thead>
              <tbody className="font-mono text-xs">
                {buckets.map((bucket) => (
                  <tr key={bucket.bucket} className="border-t border-border/50">
                    <th scope="row" className="py-2 pr-2 text-left font-normal">
                      <span className="block text-text-primary">{formatBandLabel(bucket.bucket)}</span>
                      <span className="block text-[10px] text-text-muted">
                        {formatBandRange(bucket.bucket, data.bands)}
                      </span>
                    </th>
                    <td className="py-2 px-2 text-right text-text-primary">
                      <span
                        className="mr-1.5 inline-block h-1.5 w-1.5 rounded-full align-middle"
                        style={{ backgroundColor: palette.success }}
                        aria-hidden="true"
                      />
                      {formatCompactQuantity(bucket.buyQuantity)}
                    </td>
                    <td className="py-2 px-2 text-right text-text-primary">
                      <span
                        className="mr-1.5 inline-block h-1.5 w-1.5 rounded-full align-middle"
                        style={{ backgroundColor: palette.error }}
                        aria-hidden="true"
                      />
                      {formatCompactQuantity(bucket.sellQuantity)}
                    </td>
                    <td className="py-2 pl-2 text-right text-text-primary">
                      {formatSignedQuantity(bucket.netQuantity)}
                    </td>
                  </tr>
                ))}
                <tr className="border-t border-border">
                  <th scope="row" className="py-2 pr-2 text-left font-normal text-text-secondary">
                    Total
                  </th>
                  <td className="py-2 px-2 text-right text-text-primary">
                    {formatCompactQuantity(data.totals.buyQuantity)}
                  </td>
                  <td className="py-2 px-2 text-right text-text-primary">
                    {formatCompactQuantity(data.totals.sellQuantity)}
                  </td>
                  <td className="py-2 pl-2 text-right text-text-primary">
                    {formatSignedQuantity(data.totals.netQuantity)}
                  </td>
                </tr>
              </tbody>
            </table>
            <p className="mt-2 text-[11px] text-text-muted">
              Size bands are ZondScan&apos;s own classification by {data.bands.quoteAsset} notional.
              Quantities are QRL.
            </p>
          </div>

          <div>
            <h3 className="eyebrow">Net inflow per step</h3>
            {hasFlow ? (
              <DivergingColumnChart
                points={seriesPoints}
                title={`Net QRL inflow per ${Math.round(data.seriesStepMs / 60000)} minute step over ${formatWindowLabel(data.window)}`}
                emphasise={extremeIndex(seriesPoints)}
                valueUnit="QRL"
              />
            ) : (
              <EmptyChart />
            )}
          </div>
        </div>

        <div className="border-t border-border p-4">
          <div className="flex items-baseline justify-between gap-3">
            <h3 className="eyebrow">Daily net inflow, last 5 days</h3>
            <p className="text-[11px] text-text-muted">UTC days</p>
          </div>
          {hasFlow ? (
            <DivergingColumnChart
              points={dailyPoints}
              title="Daily net QRL inflow over the last five UTC days"
              emphasise={dailyEmphasis}
              valueUnit="QRL"
            />
          ) : (
            <EmptyChart />
          )}
        </div>
      </div>
    </section>
  );
}

function EmptyChart(): JSX.Element {
  return (
    <div className="mt-2 flex h-40 items-center justify-center rounded-lg border border-dashed border-border px-6 text-center">
      <p className="text-xs text-text-muted">
        Collection has not recorded any trades yet. This chart fills in as trades arrive.
      </p>
    </div>
  );
}
