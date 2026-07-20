'use client';

import { useMemo } from 'react';
import { NATIVE_UNIT } from '../../lib/helpers';
import { Group } from '@visx/group';
import { LinePath, AreaClosed } from '@visx/shape';
import { AxisLeft, AxisBottom } from '@visx/axis';
import { scaleTime, scaleLinear } from '@visx/scale';
import { LinearGradient } from '@visx/gradient';
import { curveMonotoneX } from '@visx/curve';
import { GridRows } from '@visx/grid';
import EmptyState from '../../components/EmptyState';
import { extent, max, bisector } from 'd3-array';
import { useTooltip, TooltipWithBounds, defaultStyles } from '@visx/tooltip';
import { localPoint } from '@visx/event';
import { palette, chartTheme } from '../../lib/theme';

interface HistoryRecord {
  epoch: string;
  timestamp: number;
  validatorsCount: number;
  activeCount: number;
  totalStaked: string;
}

interface ValidatorHistoryChartProps {
  data: HistoryRecord[];
  type: 'count' | 'staked';
  width?: number;
  height?: number;
}

const margin = { top: 20, right: 20, bottom: 40, left: 60 };
const axisColor = chartTheme.axis;
const accentColor = palette.accent;
const gridColor = chartTheme.grid;

const tooltipStyles = {
  ...defaultStyles,
  ...chartTheme.tooltip,
};

// Accessors
const getDate = (d: HistoryRecord) => new Date(d.timestamp * 1000);
const getCount = (d: HistoryRecord) => d.validatorsCount;
const getStaked = (d: HistoryRecord) => {
  const val = BigInt(d.totalStaked);
  // Convert from Shor to QRL (divide by 10^9, beacon chain uses Shor)
  return Number(val / BigInt(10 ** 9));
};

const bisectDate = bisector<HistoryRecord, Date>((d) => getDate(d)).left;

export default function ValidatorHistoryChart({
  data,
  type,
  width = 600,
  height = 300,
}: ValidatorHistoryChartProps) {
  const {
    showTooltip,
    hideTooltip,
    tooltipData,
    tooltipLeft,
    tooltipTop,
  } = useTooltip<HistoryRecord>();

  const innerWidth = width - margin.left - margin.right;
  const innerHeight = height - margin.top - margin.bottom;

  // Sort data by epoch for proper line rendering
  const sortedData = useMemo(() => {
    return [...data].sort((a, b) => parseInt(a.epoch) - parseInt(b.epoch));
  }, [data]);

  const getValue = type === 'count' ? getCount : getStaked;

  const xScale = useMemo(
    () =>
      scaleTime({
        range: [0, innerWidth],
        domain: extent(sortedData, getDate) as [Date, Date],
      }),
    [innerWidth, sortedData]
  );

  const yScale = useMemo(
    () =>
      scaleLinear({
        range: [innerHeight, 0],
        domain: [0, (max(sortedData, getValue) ?? 0) * 1.1],
        nice: true,
      }),
    [innerHeight, sortedData, getValue]
  );

  const handleTooltip = (event: React.TouchEvent<SVGRectElement> | React.MouseEvent<SVGRectElement>) => {
    const { x } = localPoint(event) || { x: 0 };
    const x0 = xScale.invert(x - margin.left);
    const index = bisectDate(sortedData, x0, 1);
    const d0 = sortedData[index - 1];
    const d1 = sortedData[index];
    let d = d0;
    if (d1 && getDate(d1)) {
      d = x0.valueOf() - getDate(d0).valueOf() > getDate(d1).valueOf() - x0.valueOf() ? d1 : d0;
    }
    showTooltip({
      tooltipData: d,
      tooltipLeft: xScale(getDate(d)) + margin.left,
      tooltipTop: yScale(getValue(d)) + margin.top,
    });
  };

  const formatYAxis = (value: number) => {
    if (type === 'staked') {
      if (value >= 1000000) return `${(value / 1000000).toFixed(1)}M`;
      if (value >= 1000) return `${(value / 1000).toFixed(1)}K`;
      return value.toFixed(0);
    }
    return value.toLocaleString();
  };

  const formatTooltipValue = (d: HistoryRecord) => {
    if (type === 'staked') {
      const val = getStaked(d);
      return `${val.toLocaleString(undefined, { maximumFractionDigits: 2 })} ${NATIVE_UNIT}`;
    }
    return d.validatorsCount.toLocaleString();
  };

  if (sortedData.length < 2) {
    return (
      <EmptyState
        title="Not enough data yet"
        description="Check back as more epochs complete."
      />
    );
  }

  return (
    <div className="relative">
      <svg width={width} height={height}>
        <LinearGradient
          id={`area-gradient-${type}`}
          from={accentColor}
          fromOpacity={0.4}
          to={accentColor}
          toOpacity={0}
        />
        <Group left={margin.left} top={margin.top}>
          <GridRows
            scale={yScale}
            width={innerWidth}
            stroke={gridColor}
            numTicks={5}
          />
          <AreaClosed
            data={sortedData}
            x={(d) => xScale(getDate(d)) ?? 0}
            y={(d) => yScale(getValue(d)) ?? 0}
            yScale={yScale}
            fill={`url(#area-gradient-${type})`}
            curve={curveMonotoneX}
          />
          <LinePath
            data={sortedData}
            x={(d) => xScale(getDate(d)) ?? 0}
            y={(d) => yScale(getValue(d)) ?? 0}
            stroke={accentColor}
            strokeWidth={2}
            curve={curveMonotoneX}
          />
          <AxisBottom
            top={innerHeight}
            scale={xScale}
            numTicks={width > 500 ? 6 : 3}
            stroke={axisColor}
            tickStroke={axisColor}
            tickLabelProps={{
              fill: chartTheme.tickLabel,
              fontSize: 11,
              fontFamily: chartTheme.fontFamily,
              textAnchor: 'middle',
            }}
          />
          <AxisLeft
            scale={yScale}
            numTicks={5}
            stroke={axisColor}
            tickStroke={axisColor}
            tickFormat={(v) => formatYAxis(v as number)}
            tickLabelProps={{
              fill: chartTheme.tickLabel,
              fontSize: 11,
              fontFamily: chartTheme.fontFamily,
              textAnchor: 'end',
              dx: '-0.25em',
              dy: '0.25em',
            }}
          />
          {/* Tooltip overlay */}
          <rect
            width={innerWidth}
            height={innerHeight}
            fill="transparent"
            onMouseMove={handleTooltip}
            onMouseLeave={hideTooltip}
            onTouchStart={handleTooltip}
            onTouchMove={handleTooltip}
          />
          {/* Tooltip dot */}
          {tooltipData && (
            <circle
              cx={xScale(getDate(tooltipData))}
              cy={yScale(getValue(tooltipData))}
              r={6}
              fill={accentColor}
              stroke={palette.textPrimary}
              strokeWidth={2}
              pointerEvents="none"
            />
          )}
        </Group>
      </svg>
      {tooltipData && (
        <TooltipWithBounds
          top={tooltipTop}
          left={tooltipLeft}
          style={tooltipStyles}
        >
          <div className="text-sm">
            <div className="text-text-secondary">
              Epoch {tooltipData.epoch}
            </div>
            <div className="text-text-primary font-semibold">
              {formatTooltipValue(tooltipData)}
            </div>
            <div className="text-text-secondary text-xs">
              {new Date(tooltipData.timestamp * 1000).toLocaleDateString()}
            </div>
          </div>
        </TooltipWithBounds>
      )}
    </div>
  );
}
