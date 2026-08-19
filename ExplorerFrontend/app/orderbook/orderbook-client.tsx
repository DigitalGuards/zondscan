'use client';

import Image from 'next/image';
import { useMemo, useState } from 'react';
import axios from 'axios';
import { useQuery } from '@tanstack/react-query';
import {
  ArrowTopRightOnSquareIcon,
  PauseIcon,
  PlayIcon,
} from '@heroicons/react/24/outline';
import config from '../../config';
import FundFlowPanel from './fund-flow';
import { palette } from '../lib/theme';
import {
  buildHistoricalPlays,
  buildGroupedLadder,
  buildLadder,
  calculateBookStats,
  calculateYardValue,
  formatMarketPrice,
  formatQrlQuantity,
  formatQrlTradeSize,
  markerScale,
  marketTradeKey,
  PRICE_GROUPINGS,
  priceDeltaToYards,
  priceToFieldPosition,
  type LadderLevel,
  type MarketOrderBookResponse,
  type MarketPlay,
  type PriceGrouping,
  type MarketSide,
  type MarketTrade,
} from '../lib/orderbook';

const FIELD_WIDTH = 1000;
const FIELD_HEIGHT = 563;
// Markers on the arena field. Deliberately separate from the ladder's row
// count: the field crowds long before a table does, so showing more depth
// must not mean showing more squirrels.
const MAX_VISIBLE_LEVELS = 14;

// Rows per side in the depth ladder.
const LADDER_ROW_OPTIONS = [12, 25, 50] as const;
const DEFAULT_LADDER_ROWS = 12;

interface ArenaGameState {
  responseKey: string;
  fieldPosition: number;
  previousPosition: number;
  bullsScore: number;
  bearsScore: number;
  possession: 'bulls' | 'bears' | 'neutral';
  yardValue: number;
  lastPlayPrice: number;
  lastPlayTime: number;
  pendingQuantity: number;
  pendingExecutions: number;
  seenTradeKeys: string[];
  plays: MarketPlay[];
}

const newGameState = (): ArenaGameState => ({
  responseKey: '',
  fieldPosition: 50,
  previousPosition: 50,
  bullsScore: 0,
  bearsScore: 0,
  possession: 'neutral',
  yardValue: 0,
  lastPlayPrice: 0,
  lastPlayTime: 0,
  pendingQuantity: 0,
  pendingExecutions: 0,
  seenTradeKeys: [],
  plays: [],
});

const numeric = (value: string | number | null | undefined): number => {
  const parsed = Number(value);
  return Number.isFinite(parsed) ? parsed : 0;
};

const clamp = (value: number, min: number, max: number): number =>
  Math.min(max, Math.max(min, value));

/**
 * Stable per-bucket seed for staggering the arena's idle animation. Keyed off
 * the price so a bucket's timing survives a change in its rank; anything
 * derived from array position would be rewritten under a running animation.
 */
function seedFromPrice(price: string): number {
  let seed = 0;
  for (let index = 0; index < price.length; index += 1) {
    seed = (seed * 31 + price.charCodeAt(index)) % 100_003;
  }
  return seed;
}

function responseIdentity(data: MarketOrderBookResponse): string {
  const newestTrade = [...data.recentTrades].sort((a, b) => b.time - a.time)[0];
  return [data.fetchedAt, data.lastUpdateId, newestTrade ? marketTradeKey(newestTrade) : 'empty'].join(':');
}

function advanceGame(
  game: ArenaGameState,
  data: MarketOrderBookResponse,
  scaleMultiplier: number,
  playWindowMs: number,
): ArenaGameState {
  const responseKey = responseIdentity(data);
  if (responseKey === game.responseKey) return game;

  const stats = calculateBookStats(data);
  const orderedTrades = [...data.recentTrades].sort((a, b) => a.time - b.time);
  const newestTrade = orderedTrades[orderedTrades.length - 1];

  if (!game.responseKey) {
    const yardValue = calculateYardValue(stats) * scaleMultiplier;
    const plays = buildHistoricalPlays(orderedTrades, yardValue, playWindowMs);
    const initialPrice = newestTrade ? numeric(newestTrade.price) : stats.last;
    return {
      ...game,
      responseKey,
      yardValue,
      lastPlayPrice: initialPrice,
      lastPlayTime: newestTrade?.time ?? Date.parse(data.fetchedAt),
      possession:
        plays[0]?.direction === 'bulls'
          ? 'bulls'
          : plays[0]?.direction === 'bears'
            ? 'bears'
            : 'neutral',
      seenTradeKeys: orderedTrades.map(marketTradeKey).slice(-400),
      plays,
    };
  }

  const seen = new Set(game.seenTradeKeys);
  const unseen = orderedTrades.filter((trade) => !seen.has(marketTradeKey(trade)));
  const nextSeen = [...game.seenTradeKeys, ...unseen.map(marketTradeKey)].slice(-400);
  if (unseen.length === 0) return { ...game, responseKey, seenTradeKeys: nextSeen };

  const endTrade = unseen[unseen.length - 1];
  const endPrice = numeric(endTrade.price);
  const pendingQuantity =
    game.pendingQuantity + unseen.reduce((sum, trade) => sum + numeric(trade.quantity), 0);
  const pendingExecutions = game.pendingExecutions + unseen.length;
  const yards = priceDeltaToYards(endPrice - game.lastPlayPrice, game.yardValue);
  const elapsed = Math.max(0, endTrade.time - game.lastPlayTime);

  // Keep accumulating dust prints until the move is visible or the selected
  // play window closes. Exact executions still appear immediately in the tape.
  if (Math.abs(yards) < 0.5 && elapsed < playWindowMs) {
    return {
      ...game,
      responseKey,
      pendingQuantity,
      pendingExecutions,
      seenTradeKeys: nextSeen,
    };
  }

  const rawPosition = game.fieldPosition + yards;
  let fieldPosition = clamp(rawPosition, 3, 97);
  let bullsScore = game.bullsScore;
  let bearsScore = game.bearsScore;
  if (rawPosition >= 97) {
    bullsScore += 7;
    fieldPosition = 50;
  } else if (rawPosition <= 3) {
    bearsScore += 7;
    fieldPosition = 50;
  }

  const direction = yards > 0.05 ? 'bulls' : yards < -0.05 ? 'bears' : 'flat';
  const play: MarketPlay = {
    id: `live:${endTrade.time}:${marketTradeKey(endTrade)}`,
    occurredAt: endTrade.time,
    startPrice: game.lastPlayPrice,
    endPrice,
    yards,
    direction,
    quantity: pendingQuantity,
    executionCount: pendingExecutions,
  };

  return {
    ...game,
    responseKey,
    previousPosition: game.fieldPosition,
    fieldPosition,
    bullsScore,
    bearsScore,
    possession: direction === 'flat' ? game.possession : direction,
    lastPlayPrice: endPrice,
    lastPlayTime: endTrade.time,
    pendingQuantity: 0,
    pendingExecutions: 0,
    seenTradeKeys: nextSeen,
    plays: [play, ...game.plays].slice(0, 8),
  };
}

function formatSignedPercent(fraction: string): string {
  const value = numeric(fraction) * 100;
  if (!Number.isFinite(value)) return '...';
  return `${value > 0 ? '+' : ''}${value.toFixed(2)}%`;
}

function formatTimestamp(value: number | string): string {
  const date = typeof value === 'number' ? new Date(value) : new Date(value);
  if (Number.isNaN(date.getTime())) return '...';
  return date.toLocaleTimeString('en-GB', {
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    timeZone: 'UTC',
  });
}

function StatCard({
  label,
  value,
  detail,
  tone = 'neutral',
}: {
  label: string;
  value: string;
  detail: string;
  tone?: 'bid' | 'ask' | 'accent' | 'neutral';
}): JSX.Element {
  const valueTone =
    tone === 'bid'
      ? 'text-success'
      : tone === 'ask'
        ? 'text-error'
        : tone === 'accent'
          ? 'text-accent'
          : 'text-text-primary';
  return (
    <div className="card p-3 sm:p-4 min-w-0">
      <p className="eyebrow mb-1.5">{label}</p>
      <p className={`font-mono text-lg sm:text-xl font-medium truncate ${valueTone}`}>{value}</p>
      <p className="text-xs text-text-muted mt-1 truncate">{detail}</p>
    </div>
  );
}

function StatusBadge({
  isFetching,
  isError,
}: {
  isFetching: boolean;
  isError: boolean;
}): JSX.Element {
  if (isError) {
    return (
      <span className="inline-flex items-center gap-2 rounded-full border border-error/30 bg-error/10 px-3 py-1.5 text-xs font-medium text-error">
        <span className="h-2 w-2 rounded-full bg-error" /> Feed delayed
      </span>
    );
  }
  return (
    <span className="inline-flex items-center gap-2 rounded-full border border-success/30 bg-success/10 px-3 py-1.5 text-xs font-medium text-success">
      <span className={`h-2 w-2 rounded-full bg-success ${isFetching ? 'animate-pulse' : ''}`} />
      {isFetching ? 'Updating' : 'Live snapshot'}
    </span>
  );
}

function fieldY(position: number): number {
  return 505 - clamp(position, 0, 100) * 3.84;
}

function fieldHalfWidth(position: number): number {
  return 476 - clamp(position, 0, 100) * 2.55;
}

function levelX(position: number, index: number, side: MarketSide): number {
  const lanes = [-0.72, -0.36, 0, 0.36, 0.72];
  const lane = lanes[(index * 2 + (side === 'sell' ? 1 : 0)) % lanes.length];
  return FIELD_WIDTH / 2 + lane * fieldHalfWidth(position) * 0.78;
}

function ArenaMarker({
  level,
  side,
  index,
  currentPrice,
  fieldPosition,
  yardValue,
  scale,
  isBest,
  motionEnabled,
}: {
  level: LadderLevel;
  side: MarketSide;
  index: number;
  currentPrice: number;
  fieldPosition: number;
  yardValue: number;
  scale: number;
  isBest: boolean;
  motionEnabled: boolean;
}): JSX.Element | null {
  const position = priceToFieldPosition(
    level.price,
    currentPrice,
    fieldPosition,
    yardValue,
  );
  if (position <= 2 || position >= 98) return null;
  const x = levelX(position, index, side);
  const y = fieldY(position);
  // Geometry is drawn at a fixed base size and scaled through the CSS
  // transform below. Sizing via attributes instead would snap between polls,
  // because an attribute change is not something CSS can transition.
  const radius = 12;
  const spriteSize = radius * 3.8;
  const halo = radius + (isBest ? 4 : 2);
  const label = `${side === 'buy' ? 'Bid' : 'Ask'} price bucket at ${formatMarketPrice(level.price)} USDT, ${formatQrlQuantity(level.quantity)} QRL`;
  // Desynchronise the idle loop so the field breathes instead of pulsing in
  // lockstep. Seeded from the price rather than the array index: a bucket
  // keeps its React key across polls, so its animation keeps running, and
  // rewriting delay/duration mid-flight (which an index does whenever a
  // bucket's rank shifts) reinterprets elapsed time against the new duration
  // and jumps the marker a frame.
  const priceSeed = seedFromPrice(level.raw.price);
  const idleDelay = (priceSeed % 1600) / 1000;
  const idleDuration = 2.6 + (priceSeed % 9) / 10;
  return (
    <g
      style={{
        transform: `translate(${x.toFixed(2)}px, ${y.toFixed(2)}px) scale(${scale.toFixed(3)})`,
        // SVG elements default to transform-box: view-box with a 50% 50%
        // origin, which resolves to the centre of the viewBox rather than the
        // element. Translate alone does not care, but scale does: left at the
        // default every marker would grow towards the middle of the field.
        transformOrigin: '0 0',
        transition: motionEnabled
          ? 'transform 900ms cubic-bezier(0.16, 1, 0.3, 1)'
          : 'none',
      }}
      role="img"
      aria-label={label}
      tabIndex={0}
      className="cursor-help outline-none"
    >
      <title>{label}</title>
      {/* Stagger the entrance so the first paint arrives as a sweep rather
          than 28 markers appearing on the same frame. Capped so a marker
          that mounts mid-session is not left invisible for long. */}
      <g
        className={motionEnabled ? 'arena-enter' : undefined}
        style={
          motionEnabled ? { animationDelay: `${Math.min(index, 13) * 28}ms` } : undefined
        }
      >
        <g
          className={motionEnabled ? 'arena-idle' : undefined}
          style={
            motionEnabled
              ? { animationDelay: `${idleDelay}s`, animationDuration: `${idleDuration}s` }
              : undefined
          }
        >
          {side === 'buy' ? (
            <>
              <circle r={halo} fill="url(#bid-halo)" opacity={isBest ? 0.95 : 0.72} />
              <image
                href="/orderbook/bid-squirrel.webp"
                x={-spriteSize / 2}
                y={-spriteSize * 0.56}
                width={spriteSize}
                height={spriteSize}
                preserveAspectRatio="xMidYMid meet"
              />
            </>
          ) : (
            <>
              <rect
                x={-halo}
                y={-halo}
                width={halo * 2}
                height={halo * 2}
                rx={radius * 0.62}
                fill="url(#ask-halo)"
                opacity={isBest ? 0.95 : 0.72}
              />
              <image
                href="/orderbook/ask-squirrel.webp"
                x={-spriteSize / 2}
                y={-spriteSize * 0.56}
                width={spriteSize}
                height={spriteSize}
                preserveAspectRatio="xMidYMid meet"
              />
            </>
          )}
        </g>
      </g>
    </g>
  );
}

function MarketArena({
  bids,
  asks,
  currentPrice,
  game,
  motionEnabled,
}: {
  bids: LadderLevel[];
  asks: LadderLevel[];
  currentPrice: number;
  game: ArenaGameState;
  motionEnabled: boolean;
}): JSX.Element {
  const quantities = [...bids, ...asks].map((level) => level.quantity);
  const currentY = fieldY(game.fieldPosition);
  const previousY = fieldY(game.previousPosition);
  const latestPlay = game.plays[0];
  const currentHalfWidth = fieldHalfWidth(game.fieldPosition);
  const furthestVisiblePrice = Math.max(
    ...bids.map((level) => Math.abs(level.price - currentPrice)),
    ...asks.map((level) => Math.abs(level.price - currentPrice)),
    game.yardValue,
  );
  const availableBookYards = Math.max(
    20,
    Math.min(game.fieldPosition - 4, 96 - game.fieldPosition),
  );
  const bookYardValue = furthestVisiblePrice / availableBookYards;

  return (
    <div className="relative aspect-[16/9] min-h-[245px] overflow-hidden rounded-xl bg-background">
      <Image
        src="/orderbook/stadium-field-v2.webp"
        alt=""
        fill
        priority
        sizes="100vw"
        className="object-cover"
      />
      <div className="absolute inset-0 bg-gradient-to-b from-background/5 via-transparent to-background/20" />
      <svg
        viewBox={`0 0 ${FIELD_WIDTH} ${FIELD_HEIGHT}`}
        className="absolute inset-0 h-full w-full"
        role="group"
        aria-label="Market arena. Orange ask squirrels and green bid squirrels represent grouped price buckets."
      >
        <defs>
          <radialGradient id="bid-halo">
            <stop offset="0%" stopColor={palette.quantum} stopOpacity="0.48" />
            <stop offset="100%" stopColor={palette.quantum} stopOpacity="0" />
          </radialGradient>
          <radialGradient id="ask-halo">
            <stop offset="0%" stopColor={palette.error} stopOpacity="0.5" />
            <stop offset="100%" stopColor={palette.error} stopOpacity="0" />
          </radialGradient>
          <filter id="ball-glow" x="-100%" y="-100%" width="300%" height="300%">
            <feDropShadow dx="0" dy="0" stdDeviation="5" floodColor={palette.info} />
          </filter>
        </defs>

        <text
          x="500"
          y="92"
          textAnchor="middle"
          fill={palette.error}
          fontSize="17"
          fontWeight="600"
          letterSpacing="3"
        >
          ASK SQUIRRELS
        </text>
        <text
          x="500"
          y="536"
          textAnchor="middle"
          fill={palette.quantum}
          fontSize="17"
          fontWeight="600"
          letterSpacing="3"
        >
          BID SQUIRRELS
        </text>

        <line
          x1={FIELD_WIDTH / 2}
          x2={FIELD_WIDTH / 2}
          y1={previousY}
          y2={currentY}
          stroke={game.possession === 'bears' ? palette.error : palette.quantum}
          strokeWidth="4"
          strokeLinecap="round"
          opacity="0.78"
          strokeDasharray="6 8"
        />
        <line
          x1={FIELD_WIDTH / 2 - currentHalfWidth * 0.92}
          x2={FIELD_WIDTH / 2 + currentHalfWidth * 0.92}
          y1={currentY}
          y2={currentY}
          stroke={palette.info}
          strokeWidth="2"
          opacity="0.82"
        />

        {asks.map((level, index) => (
          <ArenaMarker
            key={`ask:${level.raw.price}`}
            level={level}
            side="sell"
            index={index}
            currentPrice={currentPrice}
            fieldPosition={game.fieldPosition}
            yardValue={bookYardValue}
            scale={markerScale(level.quantity, quantities)}
            isBest={index === 0}
            motionEnabled={motionEnabled}
          />
        ))}
        {bids.map((level, index) => (
          <ArenaMarker
            key={`bid:${level.raw.price}`}
            level={level}
            side="buy"
            index={index}
            currentPrice={currentPrice}
            fieldPosition={game.fieldPosition}
            yardValue={bookYardValue}
            scale={markerScale(level.quantity, quantities)}
            isBest={index === 0}
            motionEnabled={motionEnabled}
          />
        ))}

        <g
          style={{
            transform: `translate(${FIELD_WIDTH / 2}px, ${currentY}px)`,
            transition: motionEnabled ? 'transform 900ms cubic-bezier(0.16, 1, 0.3, 1)' : 'none',
          }}
          filter="url(#ball-glow)"
          role="img"
          aria-label={`Market ball at the ${game.fieldPosition.toFixed(1)} yard line, last trade ${formatMarketPrice(currentPrice)} USDT`}
        >
          <title>
            {`Last trade ${formatMarketPrice(currentPrice)} USDT at field position ${game.fieldPosition.toFixed(1)}`}
          </title>
          {/* Impact ripple. Keyed on the newest play so React remounts it per
              play, which is what restarts the animation; a CSS animation on a
              persistent element would only ever run once. */}
          {motionEnabled && latestPlay && (
            <circle
              key={latestPlay.id}
              r="12"
              fill="none"
              stroke={
                latestPlay.direction === 'flat'
                  ? palette.textMuted
                  : latestPlay.direction === 'bears'
                    ? palette.error
                    : palette.success
              }
              strokeWidth="2"
              className="arena-ripple"
              aria-hidden="true"
            />
          )}
          <rect
            x="-12"
            y="-12"
            width="24"
            height="24"
            rx="6"
            transform="rotate(45)"
            fill={palette.background}
            stroke={palette.info}
            strokeWidth="3"
          />
          {/* QRL omega mark. Drawn as a stroked path rather than the Unicode
              Ω glyph so it keeps the brand outline (open ring on two feet)
              at any zoom and never depends on a font fallback. Geometry is
              centered on the origin so it stays inside the rotated badge. */}
          <path
            d="M -5 5.9 L -1.8 5.9 L -1.8 3.1 A 4.7 4.7 0 1 1 1.8 3.1 L 1.8 5.9 L 5 5.9"
            fill="none"
            stroke={palette.textPrimary}
            strokeWidth="2"
            strokeLinecap="round"
            strokeLinejoin="round"
          />
        </g>

        <g transform={`translate(${FIELD_WIDTH / 2 + currentHalfWidth * 0.56} ${currentY - 12})`}>
          <rect x="-64" y="-17" width="128" height="27" rx="8" fill={palette.background} opacity="0.9" />
          <text
            x="0"
            y="2"
            textAnchor="middle"
            fill={palette.textPrimary}
            fontSize="13"
            fontFamily="IBM Plex Mono, ui-monospace, monospace"
          >
            {formatMarketPrice(currentPrice)}
          </text>
        </g>
      </svg>

      <div className="absolute left-2 top-2 sm:left-3 sm:top-3 rounded-lg border border-border bg-background/80 px-2.5 py-2 backdrop-blur-md">
        <p className="eyebrow">Arena score</p>
        <p className="mt-1 font-display text-xs sm:text-sm font-semibold">
          <span className="text-success">Bids {game.bullsScore}</span>
          <span className="mx-2 text-text-muted">:</span>
          <span className="text-error">{game.bearsScore} Asks</span>
        </p>
      </div>
    </div>
  );
}

function DepthLadder({
  bids,
  asks,
  midpoint,
  grouping,
  onGroupingChange,
  rowsPerSide,
  onRowsChange,
}: {
  bids: LadderLevel[];
  asks: LadderLevel[];
  midpoint: number;
  grouping: PriceGrouping;
  onGroupingChange: (grouping: PriceGrouping) => void;
  rowsPerSide: number;
  onRowsChange: (rows: number) => void;
}): JSX.Element {
  // Already limited to `rows` per side upstream, where the grouping runs.
  const displayBids = bids;
  const displayAsks = asks;
  const maxCumulative = Math.max(
    displayBids[displayBids.length - 1]?.cumulativeQuantity ?? 0,
    displayAsks[displayAsks.length - 1]?.cumulativeQuantity ?? 0,
    1,
  );

  // The cumulative-depth bar is a row background gradient anchored to the
  // right edge. It must not be an extra spanning <td>: that adds column slots
  // the <thead> does not have, and table-fixed then lays the table out with
  // more columns than there are headers, so every header drifts left of the
  // values it labels. The `1a` suffix is the palette color at 10% alpha.
  const depthBarStyle = (side: MarketSide, width: string): string => {
    const color = side === 'buy' ? `${palette.success}1a` : `${palette.error}1a`;
    return `linear-gradient(to left, ${color} ${width}, transparent ${width})`;
  };

  const rows = (levels: LadderLevel[], side: MarketSide): JSX.Element[] =>
    levels.map((level) => {
      const width = `${Math.min(100, (level.cumulativeQuantity / maxCumulative) * 100)}%`;
      return (
        <tr
          key={`${side}:${level.raw.price}`}
          className="font-mono text-xs"
          style={{ backgroundImage: depthBarStyle(side, width) }}
        >
          <td className={`py-1.5 pr-3 ${side === 'buy' ? 'text-success' : 'text-error'}`}>
            {formatMarketPrice(level.price)}
          </td>
          <td className="py-1.5 px-2 text-right text-text-primary">
            {formatQrlQuantity(level.quantity)}
          </td>
          <td className="py-1.5 pl-2 text-right text-text-secondary">
            {formatQrlQuantity(level.cumulativeQuantity)}
          </td>
        </tr>
      );
    });

  return (
    <div className="card overflow-hidden">
      <div className="flex items-center justify-between gap-3 border-b border-border px-4 py-3">
        <div>
          <h3 className="font-display text-sm font-semibold text-text-primary">Grouped depth ladder</h3>
          <p className="text-xs text-text-muted mt-0.5">MEXC levels combined into price buckets</p>
        </div>
        <div className="flex shrink-0 items-center gap-3">
          <label className="flex items-center gap-2 text-xs text-text-muted">
            <span className="hidden sm:inline">Group</span>
            <select
              value={grouping}
              onChange={(event) => onGroupingChange(event.target.value as PriceGrouping)}
              className="rounded-md border border-border bg-surface-2 px-2 py-1.5 font-mono text-xs text-text-primary outline-none focus:border-accent"
              aria-label="Price grouping in USDT"
            >
              {PRICE_GROUPINGS.map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </select>
          </label>
          <label className="flex items-center gap-2 text-xs text-text-muted">
            <span className="hidden sm:inline">Rows</span>
            <select
              value={rowsPerSide}
              onChange={(event) => onRowsChange(Number(event.target.value))}
              className="rounded-md border border-border bg-surface-2 px-2 py-1.5 font-mono text-xs text-text-primary outline-none focus:border-accent"
              aria-label="Ladder rows per side"
            >
              {LADDER_ROW_OPTIONS.map((option) => (
                <option key={option} value={option}>
                  {option}
                </option>
              ))}
            </select>
          </label>
        </div>
      </div>
      <div className="px-4 py-2">
        <table className="w-full table-fixed" aria-label="MEXC QRL USDT order book levels">
          <thead>
            <tr className="text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted">
              {/* Horizontal padding mirrors the body cells so each right-aligned
                  header sits exactly over its column of values. */}
              <th className="py-1.5 pr-3 text-left font-medium">Price</th>
              <th className="py-1.5 px-2 text-right font-medium">QRL</th>
              <th className="py-1.5 pl-2 text-right font-medium">Cumulative</th>
            </tr>
          </thead>
          <tbody>{rows([...displayAsks].reverse(), 'sell')}</tbody>
          <tbody>
            <tr>
              <td colSpan={3} className="border-y border-info/20 bg-info/5 py-2 text-center font-mono text-sm text-info">
                midpoint {formatMarketPrice(midpoint)}
              </td>
            </tr>
          </tbody>
          <tbody>{rows(displayBids, 'buy')}</tbody>
        </table>
      </div>
    </div>
  );
}

function RecentTrades({ trades }: { trades: MarketTrade[] }): JSX.Element {
  const ordered = [...trades].sort((a, b) => b.time - a.time).slice(0, 9);
  return (
    <div className="card overflow-hidden">
      <div className="border-b border-border px-4 py-3">
        <h3 className="font-display text-sm font-semibold text-text-primary">Recent executions</h3>
        <p className="text-xs text-text-muted mt-0.5">Aggressor side from the MEXC trade tape</p>
      </div>
      <div className="overflow-x-auto px-4 py-2">
        <table className="w-full min-w-[330px] font-mono text-xs" aria-label="Recent QRL USDT trades">
          <thead>
            <tr className="text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted">
              <th className="py-1.5 text-left font-medium">UTC</th>
              <th className="py-1.5 text-left font-medium">Side</th>
              <th className="py-1.5 text-right font-medium">Price</th>
              <th className="py-1.5 text-right font-medium">QRL</th>
            </tr>
          </thead>
          <tbody>
            {ordered.map((trade) => (
              <tr key={marketTradeKey(trade)} className="border-t border-border/50">
                <td className="py-1.5 text-text-muted">{formatTimestamp(trade.time)}</td>
                <td className={`py-1.5 uppercase ${trade.aggressorSide === 'buy' ? 'text-success' : 'text-error'}`}>
                  {trade.aggressorSide}
                </td>
                <td className="py-1.5 text-right text-text-primary">{formatMarketPrice(numeric(trade.price))}</td>
                <td className="py-1.5 text-right text-text-secondary">{formatQrlTradeSize(numeric(trade.quantity))}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function PlayFeed({ plays }: { plays: MarketPlay[] }): JSX.Element {
  return (
    <div className="card overflow-hidden">
      <div className="border-b border-border px-4 py-3">
        <h3 className="font-display text-sm font-semibold text-text-primary">Play by play</h3>
        <p className="text-xs text-text-muted mt-0.5">Price outcomes from batched executions</p>
      </div>
      <ol className="divide-y divide-border/60 px-4">
        {plays.length === 0 && (
          <li className="py-8 text-center text-sm text-text-muted">Waiting for a market print.</li>
        )}
        {plays.map((play) => {
          const outcome =
            play.direction === 'bulls'
              ? 'Bid team advances'
              : play.direction === 'bears'
                ? 'Ask team advances'
                : 'No gain';
          const tone =
            play.direction === 'bulls'
              ? 'text-success'
              : play.direction === 'bears'
                ? 'text-error'
                : 'text-text-secondary';
          return (
            <li key={play.id} className="flex items-start gap-3 py-3">
              <span
                className={`mt-1 h-2.5 w-2.5 flex-none ${play.direction === 'bears' ? 'rounded-sm bg-error' : 'rounded-full bg-success'} ${play.direction === 'flat' ? 'bg-text-muted' : ''}`}
                aria-hidden="true"
              />
              <div className="min-w-0 flex-1">
                <div className="flex items-baseline justify-between gap-3">
                  <p className={`text-sm font-medium ${tone}`}>{outcome}</p>
                  <time className="font-mono text-[11px] text-text-muted" dateTime={new Date(play.occurredAt).toISOString()}>
                    {formatTimestamp(play.occurredAt)}
                  </time>
                </div>
                <p className="mt-0.5 text-xs text-text-secondary">
                  {Math.abs(play.yards) < 0.05
                    ? 'Price unchanged'
                    : `${Math.abs(play.yards).toFixed(1)} yd to ${formatMarketPrice(play.endPrice)} USDT`}
                </p>
                <p className="mt-0.5 text-[11px] text-text-muted">
                  {play.executionCount} {play.executionCount === 1 ? 'execution' : 'executions'} · {formatQrlQuantity(play.quantity)} QRL
                </p>
              </div>
            </li>
          );
        })}
      </ol>
    </div>
  );
}

function LoadingState(): JSX.Element {
  return (
    <div className="page-content py-4 sm:py-6 lg:py-8" aria-live="polite">
      <div className="h-8 w-64 skeleton rounded-lg mb-3" />
      <div className="h-4 w-96 max-w-full skeleton rounded mb-6" />
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 mb-4">
        {Array.from({ length: 4 }, (_, index) => (
          <div key={index} className="card h-24 skeleton" />
        ))}
      </div>
      <div className="card aspect-video skeleton" />
    </div>
  );
}

export default function OrderBookClient(): JSX.Element {
  const [motionEnabled, setMotionEnabled] = useState(true);
  const [playWindowMs, setPlayWindowMs] = useState(30_000);
  const [scaleMultiplier, setScaleMultiplier] = useState(1);
  const [priceGrouping, setPriceGrouping] = useState<PriceGrouping>('0.01');
  const [ladderRows, setLadderRows] = useState<number>(DEFAULT_LADDER_ROWS);
  const [game, setGame] = useState<ArenaGameState>(newGameState);

  const orderBookQuery = useQuery<MarketOrderBookResponse>({
    queryKey: ['market-orderbook', 'QRLUSDT'],
    queryFn: async () => {
      const response = await axios.get<MarketOrderBookResponse>(
        `${config.handlerUrl}/market/orderbook`,
        { timeout: 8_000 },
      );
      return response.data;
    },
    refetchInterval: 3_000,
    refetchIntervalInBackground: false,
    staleTime: 1_500,
    retry: 2,
  });

  const data = orderBookQuery.data;
  const stats = useMemo(() => (data ? calculateBookStats(data) : null), [data]);
  const exactBids = useMemo(
    () => (data ? buildLadder(data.bids, 'buy', MAX_VISIBLE_LEVELS) : []),
    [data],
  );
  const exactAsks = useMemo(
    () => (data ? buildLadder(data.asks, 'sell', MAX_VISIBLE_LEVELS) : []),
    [data],
  );
  const bids = useMemo(
    () =>
      data
        ? buildGroupedLadder(data.bids, 'buy', priceGrouping, MAX_VISIBLE_LEVELS)
        : [],
    [data, priceGrouping],
  );
  const asks = useMemo(
    () =>
      data
        ? buildGroupedLadder(data.asks, 'sell', priceGrouping, MAX_VISIBLE_LEVELS)
        : [],
    [data, priceGrouping],
  );

  // The ladder is built separately from the arena's levels so it can run
  // deeper into the book without adding markers to the field.
  const ladderBids = useMemo(
    () => (data ? buildGroupedLadder(data.bids, 'buy', priceGrouping, ladderRows) : []),
    [data, priceGrouping, ladderRows],
  );
  const ladderAsks = useMemo(
    () => (data ? buildGroupedLadder(data.asks, 'sell', priceGrouping, ladderRows) : []),
    [data, priceGrouping, ladderRows],
  );

  if (data) {
    const nextGame = advanceGame(game, data, scaleMultiplier, playWindowMs);
    if (nextGame !== game) setGame(nextGame);
  }

  const resetGame = (options?: { scale?: number; windowMs?: number }): void => {
    if (options?.scale !== undefined) setScaleMultiplier(options.scale);
    if (options?.windowMs !== undefined) setPlayWindowMs(options.windowMs);
    setGame(newGameState());
  };

  if (!data || !stats) {
    if (orderBookQuery.isError) {
      return (
        <div className="page-content py-10">
          <div className="card mx-auto max-w-xl p-6 text-center">
            <h2 className="font-display text-xl font-semibold text-text-primary">Market feed unavailable</h2>
            <p className="mt-2 text-sm text-text-secondary">
              ZondScan could not reach the public MEXC QRL/USDT feed. Try again in a moment.
            </p>
            <button
              type="button"
              onClick={() => void orderBookQuery.refetch()}
              className="mt-5 rounded-lg bg-accent px-4 py-2 text-sm font-semibold text-background hover:bg-accent-hover"
            >
              Retry
            </button>
          </div>
        </div>
      );
    }
    return <LoadingState />;
  }

  const depthTotal = stats.bidQuantityInBand + stats.askQuantityInBand;
  const bidShare = depthTotal > 0 ? (stats.bidQuantityInBand / depthTotal) * 100 : 50;
  const newestTrade = [...data.recentTrades].sort((a, b) => b.time - a.time)[0];

  return (
    <div className="page-content py-4 sm:py-6 lg:py-8">
      <header className="mb-6 flex flex-col gap-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <div className="flex flex-wrap items-center gap-2.5">
            <h2 className="section-title">QRL Order Book Arena</h2>
            <StatusBadge isFetching={orderBookQuery.isFetching} isError={orderBookQuery.isError} />
          </div>
          <p className="mt-1.5 max-w-3xl text-sm text-text-secondary">
            Resting MEXC QRL/USDT depth forms the teams. Executed trades move the market ball.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-xs text-text-muted">
          <span>Book {formatTimestamp(data.fetchedAt)} UTC</span>
          <span>Last print {newestTrade ? `${formatTimestamp(newestTrade.time)} UTC` : 'waiting'}</span>
          <a
            href="https://www.mexc.com/exchange/QRL_USDT"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-1 text-accent hover:text-accent-hover"
          >
            MEXC Spot <ArrowTopRightOnSquareIcon className="h-3.5 w-3.5" aria-hidden="true" />
          </a>
        </div>
      </header>

      {orderBookQuery.isError && (
        <div className="mb-4 rounded-xl border border-warning/30 bg-warning/10 px-4 py-3 text-sm text-warning" role="status">
          The latest refresh failed. The last successful snapshot remains visible.
        </div>
      )}

      <section className="mb-6 grid grid-cols-2 gap-3 lg:grid-cols-4" aria-label="Market summary">
        <StatCard
          label="Last trade"
          value={`${formatMarketPrice(stats.last)} USDT`}
          detail={`${formatSignedPercent(data.ticker.changePercent)} in 24h`}
          tone="accent"
        />
        <StatCard
          label="Best bid"
          value={`${formatMarketPrice(stats.bestBid)} USDT`}
          detail={`${formatQrlQuantity(exactBids[0]?.quantity ?? 0)} QRL at best price`}
          tone="bid"
        />
        <StatCard
          label="Best ask"
          value={`${formatMarketPrice(stats.bestAsk)} USDT`}
          detail={`${formatQrlQuantity(exactAsks[0]?.quantity ?? 0)} QRL at best price`}
          tone="ask"
        />
        <StatCard
          label="Spread"
          value={`${formatMarketPrice(stats.spread)} USDT`}
          detail={`${stats.spreadBps.toFixed(1)} basis points`}
        />
      </section>

      <section className="card mb-6 overflow-hidden p-0" aria-labelledby="arena-heading">
        {/* Same header strip as every other card on the page: bordered,
            px-4 py-3. Kept as a responsive flex rather than the panel-header
            utility because the controls stack under the title on mobile. */}
        <div className="flex flex-col gap-3 border-b border-border px-4 py-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h3 id="arena-heading" className="font-display text-sm font-semibold text-text-primary">
              Live market drive
            </h3>
            <p className="mt-0.5 text-xs text-text-muted">
              1 yard = {formatMarketPrice(game.yardValue)} USDT · formations use {priceGrouping} USDT buckets
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            <div className="inline-flex rounded-lg border border-border bg-surface p-0.5" aria-label="Arena price scale">
              {[
                { label: 'Tight', value: 0.5 },
                { label: 'Auto', value: 1 },
                { label: 'Wide', value: 2 },
              ].map((option) => (
                <button
                  key={option.label}
                  type="button"
                  onClick={() => resetGame({ scale: option.value })}
                  className={`rounded-md px-2.5 py-1.5 text-[11px] font-medium transition-colors ${
                    scaleMultiplier === option.value
                      ? 'bg-accent text-background'
                      : 'text-text-muted hover:text-text-primary'
                  }`}
                  aria-pressed={scaleMultiplier === option.value}
                >
                  {option.label}
                </button>
              ))}
            </div>
            <div className="inline-flex rounded-lg border border-border bg-surface p-0.5" aria-label="Play aggregation window">
              {[15_000, 30_000, 60_000].map((windowMs) => (
                <button
                  key={windowMs}
                  type="button"
                  onClick={() => resetGame({ windowMs })}
                  className={`rounded-md px-2.5 py-1.5 text-[11px] font-medium transition-colors ${
                    playWindowMs === windowMs
                      ? 'bg-surface-3 text-text-primary'
                      : 'text-text-muted hover:text-text-primary'
                  }`}
                  aria-pressed={playWindowMs === windowMs}
                >
                  {windowMs / 1000}s
                </button>
              ))}
            </div>
            <button
              type="button"
              onClick={() => setMotionEnabled((enabled) => !enabled)}
              className="inline-flex items-center gap-1.5 rounded-lg border border-border bg-surface px-2.5 py-1.5 text-[11px] font-medium text-text-secondary hover:text-text-primary"
              aria-pressed={motionEnabled}
            >
              {motionEnabled ? <PauseIcon className="h-3.5 w-3.5" /> : <PlayIcon className="h-3.5 w-3.5" />}
              Motion {motionEnabled ? 'on' : 'off'}
            </button>
          </div>
        </div>

        <div className="p-3 sm:p-4">
          <MarketArena
            bids={bids}
            asks={asks}
            currentPrice={stats.last}
            game={game}
            motionEnabled={motionEnabled}
          />

          <div className="grid gap-3 pt-3 sm:grid-cols-[1fr_auto_1fr] sm:items-center">
            <div>
              <div className="flex items-center justify-between text-xs">
                <span className="font-medium text-success">Bids within {stats.bandPercent}%</span>
                <span className="font-mono text-text-primary">{formatQrlQuantity(stats.bidQuantityInBand)} QRL</span>
              </div>
            </div>
            <div className="hidden h-5 w-px bg-border sm:block" />
            <div>
              <div className="flex items-center justify-between text-xs">
                <span className="font-medium text-error">Asks within {stats.bandPercent}%</span>
                <span className="font-mono text-text-primary">{formatQrlQuantity(stats.askQuantityInBand)} QRL</span>
              </div>
            </div>
            <div className="sm:col-span-3 h-2 overflow-hidden rounded-full bg-surface-3" aria-label={`${bidShare.toFixed(0)} percent bid depth inside the selected band`}>
              <div className="h-full bg-success transition-[width] duration-500" style={{ width: `${bidShare}%` }} />
            </div>
          </div>
        </div>
      </section>

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.2fr)_minmax(320px,0.8fr)]">
        <DepthLadder
          bids={ladderBids}
          asks={ladderAsks}
          midpoint={stats.midpoint}
          grouping={priceGrouping}
          onGroupingChange={setPriceGrouping}
          rowsPerSide={ladderRows}
          onRowsChange={setLadderRows}
        />
        <PlayFeed plays={game.plays} />
        <div className="xl:col-span-2">
          <RecentTrades trades={data.recentTrades} />
        </div>
      </div>

      <div className="mt-6">
        <FundFlowPanel />
      </div>

      <aside className="mt-6 rounded-xl border border-border bg-surface px-4 py-3 text-xs leading-relaxed text-text-muted">
        Public MEXC spot L2 data, aggregated by price. One arena squirrel represents one selected price bucket and its total visible QRL quantity. Resting orders can change or disappear at any time, so displayed depth is not guaranteed executable liquidity. The arena is a visualization of one centralized venue and does not represent the QRL network mempool. Fund-flow rollups are built from collected public executions and start accumulating when collection begins, so they cannot be backfilled.
      </aside>
    </div>
  );
}
