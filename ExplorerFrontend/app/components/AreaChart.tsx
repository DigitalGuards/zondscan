import type { ReactNode } from 'react';
import { Group } from '@visx/group';
import { AreaClosed } from '@visx/shape';
import { AxisLeft, AxisBottom } from '@visx/axis';
import type { AxisScale } from '@visx/axis';
import { LinearGradient } from '@visx/gradient';
import { curveMonotoneX } from '@visx/curve';
import { chartTheme } from '../lib/theme';

export interface Block {
  result: {
    timestamp: string;
    size: number;
  }
}
// Initialize some variables
const axisColor = chartTheme.axis;
const axisBottomTickLabelProps = {
  textAnchor: 'middle' as const,
  fontFamily: chartTheme.fontFamily,
  fontSize: 10,
  fill: chartTheme.tickLabel,
};
const axisLeftTickLabelProps = {
  dx: '-0.25em',
  dy: '0.25em',
  fontFamily: chartTheme.fontFamily,
  fontSize: 10,
  textAnchor: 'end' as const,
  fill: chartTheme.tickLabel,
};

// accessors
const getDate = (d: Block): Date => new Date(parseInt(d.result.timestamp.slice(2), 16) * 1000);
const getStockValue = (d: Block): number => d.result.size;

export default function AreaChart({
  data,
  gradientColor,
  gradientId = 'area-gradient',
  width,
  yMax,
  margin,
  xScale,
  yScale,
  hideBottomAxis = false,
  hideLeftAxis = false,
  top,
  left,
  children,
}: {
  data: Block[];
  gradientColor: string;
  gradientId?: string;
  xScale: AxisScale<number>;
  yScale: AxisScale<number>;
  width: number;
  yMax: number;
  margin: { top: number; right: number; bottom: number; left: number };
  hideBottomAxis?: boolean;
  hideLeftAxis?: boolean;
  top?: number;
  left?: number;
  children?: ReactNode;
}): JSX.Element | null {
  if (width < 10) return null;
  return (
    <Group left={left || margin.left} top={top || margin.top}>
      <LinearGradient
        id={gradientId}
        from={gradientColor}
        fromOpacity={1}
        to={gradientColor}
        toOpacity={0.2}
      />
      <AreaClosed<Block>
        data={data}
        x={(d) => xScale(getDate(d)) || 0}
        y={(d) => yScale(getStockValue(d)) || 0}
        yScale={yScale}
        strokeWidth={1}
        stroke={`url(#${gradientId})`}
        fill={`url(#${gradientId})`}
        curve={curveMonotoneX}
      />
      {!hideBottomAxis && (
        <AxisBottom
          top={yMax}
          scale={xScale}
          numTicks={width > 520 ? 10 : 5}
          stroke={axisColor}
          tickStroke={axisColor}
          tickLabelProps={axisBottomTickLabelProps}
        />
      )}
      {!hideLeftAxis && (
        <AxisLeft
          scale={yScale}
          numTicks={5}
          stroke={axisColor}
          tickStroke={axisColor}
          tickLabelProps={axisLeftTickLabelProps}
        />
      )}
      {children}
    </Group>
  );
}
