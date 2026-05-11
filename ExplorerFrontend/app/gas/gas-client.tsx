'use client';

import { useEffect, useMemo, useState } from 'react';
import axios from 'axios';
import config from '../../config';
import Badge from '../components/Badge';
import { formatGasPrice, hexToBigInt, hexToNumber, formatBigGas } from '../lib/helpers';

interface GasSummary {
  avgGasPriceHex: string;
  avgGasUsedHex: string;
  avgGasLimitHex: string;
  avgBlockTimeSec: number;
  pendingCount: number;
  lastBlockNumberHex: string;
  lastGasUsedHex: string;
  lastGasLimitHex: string;
  gasPriceHistogram: { shorLow: number; shorHigh: number; count: number }[];
}

interface GasHistoryRow {
  blockNumber: number;
  timestamp: number;
  gasUsed: string;
  gasLimit: string;
  baseFeePerGas: string;
  txCount: number;
}

interface GasHistoryResp {
  range: '24h' | '7d';
  data: GasHistoryRow[];
}

function StatCard({ label, value, sub }: { label: string; value: string; sub?: string }): JSX.Element {
  return (
    <div className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-lg p-4">
      <p className="text-xs uppercase tracking-wider text-gray-400 mb-1">{label}</p>
      <p className="text-xl font-semibold text-[#ffa729] truncate">{value}</p>
      {sub && <p className="text-xs text-gray-500 mt-1">{sub}</p>}
    </div>
  );
}

function formatAxisTick(unixSec: number, range: '24h' | '7d'): string {
  const d = new Date(unixSec * 1000);
  if (range === '7d') {
    return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', timeZone: 'UTC' });
  }
  return d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', timeZone: 'UTC', hour12: false });
}

function GasUsedChart({ rows, range }: { rows: GasHistoryRow[]; range: '24h' | '7d' }): JSX.Element {
  // Inline SVG line+area chart. Self-contained so we don't fight with
  // AreaChart's block-shape coupling.
  const points = useMemo(() => {
    return rows
      .map((r) => ({ x: r.timestamp, y: hexToNumber(r.gasUsed) }))
      .filter((p) => p.x > 0);
  }, [rows]);

  if (points.length < 2) {
    return (
      <div className="text-center text-gray-500 text-sm py-12">
        Not enough gas history yet — check back after the syncer has run its periodic task.
      </div>
    );
  }

  const w = 800;
  const h = 240;
  const pad = { l: 56, r: 16, t: 12, b: 28 };
  const innerW = w - pad.l - pad.r;
  const innerH = h - pad.t - pad.b;
  const xs = points.map((p) => p.x);
  const ys = points.map((p) => p.y);
  const xMin = Math.min(...xs);
  const xMax = Math.max(...xs);
  const yMin = 0;
  const yMax = Math.max(...ys) * 1.05 || 1;
  const sx = (x: number) => pad.l + ((x - xMin) / (xMax - xMin || 1)) * innerW;
  const sy = (y: number) => pad.t + innerH - ((y - yMin) / (yMax - yMin || 1)) * innerH;

  const linePath = points.map((p, i) => `${i === 0 ? 'M' : 'L'}${sx(p.x).toFixed(1)},${sy(p.y).toFixed(1)}`).join(' ');
  const areaPath = `${linePath} L${sx(points[points.length - 1].x).toFixed(1)},${sy(0).toFixed(1)} L${sx(points[0].x).toFixed(1)},${sy(0).toFixed(1)} Z`;

  // Y-axis tick labels (4 ticks)
  const yTicks = [0.25, 0.5, 0.75, 1].map((f) => yMax * f);
  // X-axis tick labels (5 ticks)
  const xTicks = Array.from({ length: 5 }, (_, i) => xMin + ((xMax - xMin) * i) / 4);

  return (
    <div className="w-full overflow-x-auto">
      <svg viewBox={`0 0 ${w} ${h}`} className="w-full h-auto" preserveAspectRatio="xMidYMid meet">
        <defs>
          <linearGradient id="gasGradient" x1="0" x2="0" y1="0" y2="1">
            <stop offset="0%" stopColor="#ffa729" stopOpacity="0.7" />
            <stop offset="100%" stopColor="#ffa729" stopOpacity="0.05" />
          </linearGradient>
        </defs>
        {/* Grid */}
        {yTicks.map((t, i) => (
          <line key={i} x1={pad.l} x2={w - pad.r} y1={sy(t)} y2={sy(t)} stroke="#3d3d3d" strokeWidth="0.5" />
        ))}
        <path d={areaPath} fill="url(#gasGradient)" />
        <path d={linePath} fill="none" stroke="#ffa729" strokeWidth="1.5" />
        {/* Y labels */}
        {yTicks.map((t, i) => (
          <text key={i} x={pad.l - 6} y={sy(t)} dy="0.32em" textAnchor="end" fontSize="10" fill="#9ca3af">
            {Math.round(t).toLocaleString()}
          </text>
        ))}
        {/* X labels */}
        {xTicks.map((t, i) => (
          <text key={i} x={sx(t)} y={h - 8} textAnchor="middle" fontSize="10" fill="#9ca3af">
            {formatAxisTick(t, range)}
          </text>
        ))}
      </svg>
    </div>
  );
}

function HistogramChart({ buckets }: { buckets: GasSummary['gasPriceHistogram'] }): JSX.Element {
  if (!buckets || buckets.length === 0) {
    return <div className="text-center text-gray-500 text-sm py-12">Mempool is empty.</div>;
  }
  const w = 800;
  const h = 220;
  const pad = { l: 40, r: 16, t: 12, b: 36 };
  const innerW = w - pad.l - pad.r;
  const innerH = h - pad.t - pad.b;
  const maxCount = Math.max(...buckets.map((b) => b.count), 1);
  const barW = innerW / buckets.length;
  return (
    <div className="w-full overflow-x-auto">
      <svg viewBox={`0 0 ${w} ${h}`} className="w-full h-auto" preserveAspectRatio="xMidYMid meet">
        {buckets.map((b, i) => {
          const barH = (b.count / maxCount) * innerH;
          const x = pad.l + i * barW + 2;
          const y = pad.t + innerH - barH;
          return (
            <g key={i}>
              <rect x={x} y={y} width={barW - 4} height={barH} fill="#ffa729" opacity="0.8" />
              <text
                x={x + (barW - 4) / 2}
                y={h - 18}
                fontSize="9"
                fill="#9ca3af"
                textAnchor="middle"
              >
                {b.shorLow === b.shorHigh ? b.shorLow.toFixed(0) : `${b.shorLow.toFixed(0)}–${b.shorHigh.toFixed(0)}`}
              </text>
              {b.count > 0 && (
                <text x={x + (barW - 4) / 2} y={y - 3} fontSize="9" fill="#d1d5db" textAnchor="middle">
                  {b.count}
                </text>
              )}
            </g>
          );
        })}
        <text x={w / 2} y={h - 4} fontSize="10" fill="#6b7280" textAnchor="middle">
          gas price (Shor)
        </text>
      </svg>
    </div>
  );
}

export default function GasClient(): JSX.Element {
  const [summary, setSummary] = useState<GasSummary | null>(null);
  const [history, setHistory] = useState<GasHistoryRow[]>([]);
  const [range, setRange] = useState<'24h' | '7d'>('24h');
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const s = await axios.get<GasSummary>(`${config.handlerUrl}/gas/summary`);
        if (!cancelled) setSummary(s.data);
      } catch (e: unknown) {
        if (!cancelled) setErr('Failed to load gas summary');
        console.error(e);
      }
    };
    load();
    const id = setInterval(load, 10000);
    return () => {
      cancelled = true;
      clearInterval(id);
    };
  }, []);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      try {
        const r = await axios.get<GasHistoryResp>(`${config.handlerUrl}/gas/history?range=${range}`);
        if (!cancelled) setHistory(r.data?.data ?? []);
      } catch (e) {
        console.error(e);
      }
    };
    load();
  }, [range]);

  const utilization = useMemo(() => {
    if (!summary) return null;
    const used = hexToBigInt(summary.lastGasUsedHex);
    const limit = hexToBigInt(summary.lastGasLimitHex);
    if (limit === BigInt(0)) return null;
    // percentage with 1 decimal
    const pct = Number((used * BigInt(1000)) / limit) / 10;
    return pct;
  }, [summary]);

  return (
    <div className="p-4 sm:p-8 max-w-6xl mx-auto">
      <div className="flex items-center justify-between mb-6">
        <div>
          <h2 className="text-2xl font-bold text-[#ffa729]">Network Gas</h2>
          <p className="text-sm text-gray-400 mt-1">
            Live mempool + block stats from the QRL Zond network. Updated every 10s.
          </p>
        </div>
        <div className="flex items-center gap-2">
          {(['24h', '7d'] as const).map((r) => (
            <button
              key={r}
              onClick={() => setRange(r)}
              className={`text-xs px-3 py-1 rounded-full border transition-colors ${
                range === r
                  ? 'bg-[#ffa729]/20 text-[#ffa729] border-[#ffa729]/40'
                  : 'bg-[#1f1f1f] text-gray-400 border-[#3d3d3d] hover:text-gray-200'
              }`}
            >
              {r}
            </button>
          ))}
        </div>
      </div>

      {err && (
        <div className="bg-red-900/20 border border-red-500/40 rounded-lg p-3 mb-4 text-sm text-red-400">{err}</div>
      )}

      <div className="grid grid-cols-2 md:grid-cols-3 gap-3 mb-6">
        <StatCard
          label="Avg Gas Price"
          value={summary ? `${formatGasPrice(summary.avgGasPriceHex)} Shor` : '—'}
          sub="mempool median"
        />
        <StatCard
          label="Avg Block Time"
          value={summary ? `${summary.avgBlockTimeSec.toFixed(1)}s` : '—'}
          sub="last 30 blocks"
        />
        <StatCard
          label="Mempool Size"
          value={summary ? summary.pendingCount.toString() : '—'}
          sub="pending transactions"
        />
        <StatCard
          label="Avg Gas Used"
          value={summary ? formatBigGas(summary.avgGasUsedHex) : '—'}
          sub="per block, last 30"
        />
        <StatCard
          label="Last Block"
          value={summary ? hexToNumber(summary.lastBlockNumberHex).toLocaleString() : '—'}
          sub={summary ? `${formatBigGas(summary.lastGasUsedHex)} / ${formatBigGas(summary.lastGasLimitHex)} gas` : ''}
        />
        <StatCard
          label="Gas Limit Utilization"
          value={utilization !== null ? `${utilization.toFixed(1)}%` : '—'}
          sub="last block"
        />
      </div>

      <div className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-lg p-4 sm:p-6 mb-6">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-semibold text-[#ffa729]">Gas Used per Block</h3>
          <Badge variant="neutral">{range}</Badge>
        </div>
        <GasUsedChart rows={history} range={range} />
      </div>

      <div className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-lg p-4 sm:p-6">
        <div className="flex items-center justify-between mb-3">
          <h3 className="text-sm font-semibold text-[#ffa729]">Mempool Gas Price Distribution</h3>
          {summary && <Badge variant="neutral">{summary.pendingCount} tx</Badge>}
        </div>
        {summary && <HistogramChart buckets={summary.gasPriceHistogram} />}
      </div>
    </div>
  );
}
