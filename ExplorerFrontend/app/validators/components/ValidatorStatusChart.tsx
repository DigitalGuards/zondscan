'use client';

import { useState, useMemo } from 'react';
import { Group } from '@visx/group';
import { Pie } from '@visx/shape';
import { scaleOrdinal } from '@visx/scale';

interface StatusData {
  label: string;
  value: number;
  color: string;
}

interface ValidatorStatusChartProps {
  activeCount: number;
  pendingCount: number;
  exitedCount: number;
  slashedCount: number;
  width?: number;
  height?: number;
}

const defaultMargin = { top: 20, right: 20, bottom: 20, left: 20 };

export default function ValidatorStatusChart({
  activeCount,
  pendingCount,
  exitedCount,
  slashedCount,
  width = 300,
  height = 300,
}: ValidatorStatusChartProps) {
  const [active, setActive] = useState<StatusData | null>(null);

  const data: StatusData[] = useMemo(() => {
    const items = [
      { label: 'Active', value: activeCount, color: '#22c55e' },
      { label: 'Pending', value: pendingCount, color: '#eab308' },
      { label: 'Exited', value: exitedCount, color: '#6b7280' },
      { label: 'Slashed', value: slashedCount, color: '#ef4444' },
    ];
    return items.filter(item => item.value > 0);
  }, [activeCount, pendingCount, exitedCount, slashedCount]);

  const total = activeCount + pendingCount + exitedCount + slashedCount;

  const innerWidth = width - defaultMargin.left - defaultMargin.right;
  const innerHeight = height - defaultMargin.top - defaultMargin.bottom;
  const radius = Math.min(innerWidth, innerHeight) / 2;
  const centerY = innerHeight / 2;
  const centerX = innerWidth / 2;
  const donutThickness = 40;

  const getColor = scaleOrdinal({
    domain: data.map((d) => d.label),
    range: data.map((d) => d.color),
  });

  if (total === 0) {
    return (
      <div className="flex items-center justify-center h-full text-gray-400">
        No validator data available
      </div>
    );
  }

  return (
    <div className="relative">
      <svg width={width} height={height}>
        <Group top={centerY + defaultMargin.top} left={centerX + defaultMargin.left}>
          <Pie
            data={data}
            pieValue={(d) => d.value}
            outerRadius={radius}
            innerRadius={radius - donutThickness}
            cornerRadius={3}
            padAngle={0.02}
          >
            {(pie) =>
              pie.arcs.map((arc, index) => {
                const [centroidX, centroidY] = pie.path.centroid(arc);
                const hasSpaceForLabel = arc.endAngle - arc.startAngle >= 0.3;
                const arcPath = pie.path(arc) ?? '';
                const arcFill = getColor(arc.data.label);
                const isActive = active?.label === arc.data.label;

                return (
                  <g key={`arc-${arc.data.label}-${index}`}>
                    <path
                      d={arcPath}
                      fill={arcFill}
                      opacity={active && !isActive ? 0.5 : 1}
                      onMouseEnter={() => setActive(arc.data)}
                      onMouseLeave={() => setActive(null)}
                      style={{
                        cursor: 'pointer',
                        transition: 'opacity 0.2s',
                        transform: isActive ? 'scale(1.02)' : 'scale(1)',
                        transformOrigin: 'center',
                      }}
                    />
                    {hasSpaceForLabel && (
                      <text
                        x={centroidX}
                        y={centroidY}
                        dy=".33em"
                        fill="#fff"
                        fontSize={12}
                        textAnchor="middle"
                        pointerEvents="none"
                      >
                        {Math.round((arc.data.value / total) * 100)}%
                      </text>
                    )}
                  </g>
                );
              })
            }
          </Pie>
          {/* Center text */}
          <text
            textAnchor="middle"
            fill="#ffa729"
            fontSize={24}
            fontWeight="bold"
            dy="-0.2em"
          >
            {total.toLocaleString()}
          </text>
          <text
            textAnchor="middle"
            fill="#9ca3af"
            fontSize={12}
            dy="1.2em"
          >
            validators
          </text>
        </Group>
      </svg>

      {/* Legend */}
      <div className="flex flex-wrap justify-center gap-4 mt-4">
        {data.map((item) => (
          <div
            key={item.label}
            className={`flex items-center gap-2 cursor-pointer transition-opacity ${
              active && active.label !== item.label ? 'opacity-50' : ''
            }`}
            onMouseEnter={() => setActive(item)}
            onMouseLeave={() => setActive(null)}
          >
            <div
              className="w-3 h-3 rounded-full"
              style={{ backgroundColor: item.color }}
            />
            <span className="text-sm text-gray-300">
              {item.label}: {item.value.toLocaleString()}
            </span>
          </div>
        ))}
      </div>

      {/* Tooltip */}
      {active && (
        <div className="absolute top-4 right-4 bg-[#1f1f1f] border border-[#3d3d3d] rounded-lg p-3 shadow-lg">
          <div className="flex items-center gap-2">
            <div
              className="w-3 h-3 rounded-full"
              style={{ backgroundColor: active.color }}
            />
            <span className="text-gray-300 font-medium">{active.label}</span>
          </div>
          <div className="text-xl font-semibold text-white mt-1">
            {active.value.toLocaleString()}
          </div>
          <div className="text-sm text-gray-400">
            {((active.value / total) * 100).toFixed(1)}% of total
          </div>
        </div>
      )}
    </div>
  );
}
