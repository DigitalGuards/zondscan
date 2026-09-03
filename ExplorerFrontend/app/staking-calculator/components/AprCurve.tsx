'use client';

import { useMemo } from 'react';
import { Group } from '@visx/group';
import { LinePath, AreaClosed, Line, Circle } from '@visx/shape';
import { AxisLeft, AxisBottom } from '@visx/axis';
import { scaleLinear } from '@visx/scale';
import { LinearGradient } from '@visx/gradient';
import { GridRows } from '@visx/grid';
import { curveMonotoneX } from '@visx/curve';
import { ParentSize } from '@visx/responsive';
import { palette, chartTheme } from '../../lib/theme';
import { MAX_EFFECTIVE_BALANCE_QUANTA, grossAprForValidators } from '../../lib/staking';

interface AprCurveProps {
  /** Validator count when 100% of supply is staked: the right edge. */
  maxValidators: number;
  circulatingSupply: number;
  /** Live network, drawn as a reference marker. */
  currentValidators: number;
  /** The user's modelled network size, drawn as the active marker. */
  scenarioValidators: number;
}

interface Point {
  ratio: number;
  apr: number;
}

const margin = { top: 12, right: 16, bottom: 50, left: 52 };
const SAMPLES = 140;

/**
 * Gross APR plotted against the share of circulating supply staked.
 *
 * The supply share is the honest x-axis: a validator locks a fixed
 * 40,000 Quanta, so the active set cannot grow past supply/40,000 and the
 * curve has a real right edge rather than trailing off to infinity.
 */
function Chart({
  width,
  height,
  maxValidators,
  circulatingSupply,
  currentValidators,
  scenarioValidators,
}: AprCurveProps & { width: number; height: number }): JSX.Element | null {
  const innerWidth = width - margin.left - margin.right;
  const innerHeight = height - margin.top - margin.bottom;

  const data = useMemo<Point[]>(() => {
    if (circulatingSupply <= 0 || maxValidators < 1) return [];
    const points: Point[] = [];
    // Start above zero: the curve is asymptotic at the origin and a first
    // sample at one validator would flatten everything else against the axis.
    const first = Math.max(1, Math.round(maxValidators * 0.02));
    for (let i = 0; i <= SAMPLES; i += 1) {
      const v = Math.round(first + ((maxValidators - first) * i) / SAMPLES);
      points.push({
        ratio: (v * MAX_EFFECTIVE_BALANCE_QUANTA) / circulatingSupply,
        apr: grossAprForValidators(v),
      });
    }
    return points;
  }, [maxValidators, circulatingSupply]);

  const scales = useMemo(() => {
    if (data.length === 0) return null;
    const maxApr = Math.max(...data.map((d) => d.apr));
    return {
      x: scaleLinear<number>({
        domain: [data[0].ratio, data[data.length - 1].ratio],
        range: [0, innerWidth],
      }),
      y: scaleLinear<number>({
        domain: [0, maxApr * 1.08],
        range: [innerHeight, 0],
        nice: true,
      }),
    };
  }, [data, innerWidth, innerHeight]);

  if (!scales || innerWidth <= 0 || innerHeight <= 0) return null;

  const { x, y } = scales;
  const ratioOf = (validators: number): number =>
    (validators * MAX_EFFECTIVE_BALANCE_QUANTA) / circulatingSupply;

  const [lo, hi] = x.domain();
  const inDomain = (m: { ratio: number }): boolean => m.ratio >= lo && m.ratio <= hi;

  const nowMarker = {
    key: 'current',
    label: 'now',
    ratio: ratioOf(currentValidators),
    apr: grossAprForValidators(currentValidators),
    color: palette.textMuted,
    dashed: true,
  };
  const scenarioMarker = {
    key: 'scenario',
    label: 'scenario',
    ratio: ratioOf(scenarioValidators),
    apr: grossAprForValidators(scenarioValidators),
    color: palette.accent,
    dashed: false,
  };

  const visible = [nowMarker, scenarioMarker].filter(inDomain);
  // The scenario starts on the live network, so both markers sit on the same
  // pixel and their labels would overprint. Collapse to the active one and
  // call it what it is.
  const collides =
    visible.length === 2 && Math.abs(x(nowMarker.ratio) - x(scenarioMarker.ratio)) < 48;
  const markers = collides ? [{ ...scenarioMarker, label: 'now' }] : visible;

  return (
    <svg width={width} height={height} role="img" aria-label="Gross APR against share of supply staked">
      <LinearGradient
        id="apr-curve-fill"
        from={palette.accent}
        to={palette.accent}
        fromOpacity={0.22}
        toOpacity={0}
      />
      <Group left={margin.left} top={margin.top}>
        <GridRows scale={y} width={innerWidth} stroke={chartTheme.grid} strokeWidth={1} numTicks={5} />

        <AreaClosed<Point>
          data={data}
          x={(d) => x(d.ratio)}
          y={(d) => y(d.apr)}
          yScale={y}
          fill="url(#apr-curve-fill)"
          curve={curveMonotoneX}
        />
        <LinePath<Point>
          data={data}
          x={(d) => x(d.ratio)}
          y={(d) => y(d.apr)}
          stroke={palette.accent}
          strokeWidth={2}
          curve={curveMonotoneX}
        />

        {markers.map((m) => (
          <Group key={m.key}>
            <Line
              from={{ x: x(m.ratio), y: innerHeight }}
              to={{ x: x(m.ratio), y: y(m.apr) }}
              stroke={m.color}
              strokeWidth={1}
              strokeDasharray={m.dashed ? '3,3' : undefined}
              opacity={m.dashed ? 0.7 : 1}
            />
            <Circle cx={x(m.ratio)} cy={y(m.apr)} r={4} fill={m.color} />
            <text
              x={Math.min(x(m.ratio) + 8, innerWidth - 4)}
              y={Math.max(y(m.apr) - 8, 12)}
              textAnchor={x(m.ratio) > innerWidth - 60 ? 'end' : 'start'}
              fill={m.color}
              fontSize={10}
              fontFamily={chartTheme.fontFamily}
            >
              {m.label} {(m.apr * 100).toFixed(2)}%
            </text>
          </Group>
        ))}

        <AxisLeft
          scale={y}
          numTicks={5}
          stroke={chartTheme.axis}
          tickStroke={chartTheme.axis}
          tickFormat={(v) => `${(Number(v) * 100).toFixed(0)}%`}
          tickLabelProps={() => ({
            fill: chartTheme.tickLabel,
            fontSize: 10,
            fontFamily: chartTheme.fontFamily,
            textAnchor: 'end',
            dx: -4,
            dy: 3,
          })}
        />
        <AxisBottom
          top={innerHeight}
          scale={x}
          numTicks={Math.max(3, Math.min(6, Math.floor(innerWidth / 90)))}
          stroke={chartTheme.axis}
          tickStroke={chartTheme.axis}
          tickFormat={(v) => `${(Number(v) * 100).toFixed(0)}%`}
          tickLabelProps={() => ({
            fill: chartTheme.tickLabel,
            fontSize: 10,
            fontFamily: chartTheme.fontFamily,
            textAnchor: 'middle',
            dy: 4,
          })}
          label="Share of circulating supply staked"
          labelProps={{
            fill: chartTheme.tickLabel,
            fontSize: 10,
            fontFamily: chartTheme.fontFamily,
            textAnchor: 'middle',
          }}
          labelOffset={14}
        />
      </Group>
    </svg>
  );
}

export default function AprCurve(props: AprCurveProps): JSX.Element {
  return (
    <div className="h-64 w-full">
      <ParentSize>
        {({ width, height }) =>
          width > 0 && height > 0 ? <Chart {...props} width={width} height={height} /> : null
        }
      </ParentSize>
    </div>
  );
}
