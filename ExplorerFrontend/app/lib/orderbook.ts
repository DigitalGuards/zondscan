export type MarketSide = 'buy' | 'sell';

export const PRICE_GROUPINGS = ['0.00001', '0.0001', '0.001', '0.01', '0.1'] as const;

export type PriceGrouping = (typeof PRICE_GROUPINGS)[number];

export interface MarketOrderBookLevel {
  price: string;
  quantity: string;
}

export interface MarketTrade {
  id: string;
  price: string;
  quantity: string;
  quoteQuantity: string;
  time: number;
  aggressorSide: MarketSide;
}

export interface MarketTicker {
  high: string;
  low: string;
  last: string;
  change: string;
  changePercent: string;
  baseVolume: string;
  quoteVolume: string | null;
}

export interface MarketOrderBookResponse {
  venue: string;
  symbol: string;
  fetchedAt: string;
  lastUpdateId: number;
  bids: MarketOrderBookLevel[];
  asks: MarketOrderBookLevel[];
  recentTrades: MarketTrade[];
  ticker: MarketTicker;
}

export interface LadderLevel {
  price: number;
  quantity: number;
  notional: number;
  cumulativeQuantity: number;
  raw: MarketOrderBookLevel;
}

export interface MarketBookStats {
  bestBid: number;
  bestAsk: number;
  midpoint: number;
  spread: number;
  spreadBps: number;
  last: number;
  high24h: number;
  low24h: number;
  bidQuantityInBand: number;
  askQuantityInBand: number;
  bandPercent: number;
}

const finiteNumber = (value: string | number | null | undefined): number => {
  const parsed = typeof value === 'number' ? value : Number(value);
  return Number.isFinite(parsed) ? parsed : 0;
};

export function buildLadder(
  levels: MarketOrderBookLevel[],
  side: MarketSide,
  limit = 12
): LadderLevel[] {
  const sorted = levels
    .map((level) => ({
      raw: level,
      price: finiteNumber(level.price),
      quantity: finiteNumber(level.quantity),
    }))
    .filter((level) => level.price > 0 && level.quantity > 0)
    .sort((a, b) => (side === 'buy' ? b.price - a.price : a.price - b.price))
    .slice(0, limit);

  let cumulativeQuantity = 0;
  return sorted.map((level) => {
    cumulativeQuantity += level.quantity;
    return {
      ...level,
      notional: level.price * level.quantity,
      cumulativeQuantity,
    };
  });
}

interface ParsedDecimal {
  coefficient: bigint;
  scale: number;
}

function parseUnsignedDecimal(value: string): ParsedDecimal | null {
  const match = /^(\d+)(?:\.(\d+))?$/.exec(value.trim());
  if (!match) return null;

  const fraction = match[2] ?? '';
  return {
    coefficient: BigInt(`${match[1]}${fraction}`),
    scale: fraction.length,
  };
}

function powerOfTen(exponent: number): bigint {
  return BigInt(10) ** BigInt(exponent);
}

function groupingScale(grouping: PriceGrouping): number {
  return grouping.split('.')[1]?.length ?? 0;
}

function bucketPriceUnits(price: string, side: MarketSide, grouping: PriceGrouping): bigint | null {
  const parsedPrice = parseUnsignedDecimal(price);
  const parsedGrouping = parseUnsignedDecimal(grouping);
  if (
    !parsedPrice ||
    !parsedGrouping ||
    parsedPrice.coefficient <= BigInt(0) ||
    parsedGrouping.coefficient <= BigInt(0)
  ) {
    return null;
  }

  const commonScale = Math.max(parsedPrice.scale, parsedGrouping.scale);
  const scaledPrice = parsedPrice.coefficient * powerOfTen(commonScale - parsedPrice.scale);
  const scaledGrouping =
    parsedGrouping.coefficient * powerOfTen(commonScale - parsedGrouping.scale);
  const wholeBuckets = scaledPrice / scaledGrouping;
  const remainder = scaledPrice % scaledGrouping;
  const bucketIndex =
    side === 'sell' && remainder > BigInt(0) ? wholeBuckets + BigInt(1) : wholeBuckets;
  const commonBucket = bucketIndex * scaledGrouping;

  return commonBucket / powerOfTen(commonScale - parsedGrouping.scale);
}

function formatDecimalUnits(units: bigint, scale: number): string {
  if (scale === 0) return units.toString();
  const digits = units.toString().padStart(scale + 1, '0');
  return `${digits.slice(0, -scale)}.${digits.slice(-scale)}`;
}

/**
 * Groups a full depth snapshot into exchange-style price buckets before the
 * result is limited. Bid prices round down and ask prices round up so grouped
 * levels never imply a more favorable executable price than the source book.
 */
export function buildGroupedLadder(
  levels: MarketOrderBookLevel[],
  side: MarketSide,
  grouping: PriceGrouping,
  limit = 12
): LadderLevel[] {
  const scale = groupingScale(grouping);
  const grouped = new Map<string, { bucketUnits: bigint; quantity: number; notional: number }>();

  for (const level of levels) {
    const quantity = finiteNumber(level.quantity);
    const price = finiteNumber(level.price);
    const bucketUnits = bucketPriceUnits(level.price, side, grouping);
    if (quantity <= 0 || price <= 0 || bucketUnits === null || bucketUnits <= BigInt(0)) continue;

    const key = bucketUnits.toString();
    const current = grouped.get(key) ?? {
      bucketUnits,
      quantity: 0,
      notional: 0,
    };
    current.quantity += quantity;
    current.notional += price * quantity;
    grouped.set(key, current);
  }

  const sorted = [...grouped.values()]
    .sort((a, b) => {
      if (a.bucketUnits === b.bucketUnits) return 0;
      if (side === 'buy') return a.bucketUnits > b.bucketUnits ? -1 : 1;
      return a.bucketUnits < b.bucketUnits ? -1 : 1;
    })
    .slice(0, Math.max(0, Math.floor(limit)));

  let cumulativeQuantity = 0;
  return sorted.map((level) => {
    cumulativeQuantity += level.quantity;
    const priceString = formatDecimalUnits(level.bucketUnits, scale);
    return {
      price: Number(priceString),
      quantity: level.quantity,
      notional: level.notional,
      cumulativeQuantity,
      raw: {
        price: priceString,
        quantity: level.quantity.toString(),
      },
    };
  });
}

export function calculateBookStats(
  response: MarketOrderBookResponse,
  bandPercent = 2
): MarketBookStats {
  const allBids = buildLadder(response.bids, 'buy', response.bids.length);
  const allAsks = buildLadder(response.asks, 'sell', response.asks.length);
  const bestBid = allBids[0]?.price ?? 0;
  const bestAsk = allAsks[0]?.price ?? 0;
  const midpoint = bestBid > 0 && bestAsk > 0 ? (bestBid + bestAsk) / 2 : bestBid || bestAsk;
  const spread = bestBid > 0 && bestAsk > 0 ? Math.max(0, bestAsk - bestBid) : 0;
  const spreadBps = midpoint > 0 ? (spread / midpoint) * 10_000 : 0;
  const bandRatio = Math.max(0, bandPercent) / 100;
  const lowBound = midpoint * (1 - bandRatio);
  const highBound = midpoint * (1 + bandRatio);

  return {
    bestBid,
    bestAsk,
    midpoint,
    spread,
    spreadBps,
    last: finiteNumber(response.ticker.last) || midpoint,
    high24h: finiteNumber(response.ticker.high),
    low24h: finiteNumber(response.ticker.low),
    bidQuantityInBand: allBids
      .filter((level) => level.price >= lowBound)
      .reduce((sum, level) => sum + level.quantity, 0),
    askQuantityInBand: allAsks
      .filter((level) => level.price <= highBound)
      .reduce((sum, level) => sum + level.quantity, 0),
    bandPercent,
  };
}

export function marketTradeKey(trade: MarketTrade): string {
  return [trade.id, trade.time, trade.price, trade.quantity, trade.aggressorSide].join(':');
}

export function formatMarketPrice(value: number): string {
  if (!Number.isFinite(value) || value <= 0) return '...';
  return value.toLocaleString('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 8,
  });
}

/**
 * Formats a QRL quantity for a column of quantities.
 *
 * Precision is decided by magnitude bands rather than per value. Choosing it
 * per value (the old `value < 100 ? 2 : 0`) meant one bucket under the
 * threshold grew two decimals while its neighbours stayed whole, so a ladder
 * column read as "13.93" next to "1,152" and looked broken.
 *
 * At or above 1 QRL the fraction is noise next to the bucket size, so the
 * value is whole. Below 1 the fraction is the only information present, so it
 * is kept, and dust that would round to a misleading "0" is labelled instead.
 */
export function formatQrlQuantity(value: number): string {
  if (!Number.isFinite(value)) return '...';
  if (value > 0 && value < 0.01) return '<0.01';
  return value.toLocaleString('en-US', {
    minimumFractionDigits: 0,
    maximumFractionDigits: Math.abs(value) < 1 ? 2 : 0,
  });
}

/**
 * Formats a single execution's size for the trade tape.
 *
 * Precision is fixed at two decimals at every magnitude. An individual print
 * is small and its fraction is real information (7.75 QRL is not 8), so it
 * keeps the decimals that formatQrlQuantity drops for aggregates, and it
 * keeps them on every row so the column stays one shape.
 */
export function formatQrlTradeSize(value: number): string {
  if (!Number.isFinite(value)) return '...';
  return value.toLocaleString('en-US', {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  });
}
