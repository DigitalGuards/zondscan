'use client';

import BalanceDisplay from './balance-display';
import ActivityDisplay from './activity-display';
import { useAddressAggregate, type AddressAggregateResponse } from './address-tabs';

interface LiveAddressStatsProps {
  address: string;
  /** The server-rendered aggregate; the same object AddressTabs receives. */
  initialAggregate: AddressAggregateResponse;
}

/** A row's timestamp in unix seconds. The handler sends it as a hex string
 *  ("0x6afc032c") on polled rows and the server page as it arrived, so both
 *  spellings are read here. */
const toSeconds = (value: unknown): number => {
  if (typeof value === 'number') return value;
  if (typeof value === 'string') return Number(value);
  return Number.NaN;
};

/**
 * First and last activity of an address, in unix seconds (0 = none).
 *
 * Prefers the server-computed range: the tx list is paginated, so deriving
 * the range from the loaded page reports the oldest row of the newest page
 * as "First Activity". Falls back to the page-derived range against
 * handlers predating first_seen/last_seen.
 */
export function activityRange(aggregate: AddressAggregateResponse): {
  firstSeen: number;
  lastSeen: number;
} {
  const firstSeen = aggregate.first_seen ?? 0;
  const lastSeen = aggregate.last_seen ?? 0;
  if (firstSeen) return { firstSeen, lastSeen };

  const timestamps = (aggregate.transactions_by_address ?? [])
    .map((tx) => toSeconds(tx.TimeStamp))
    .filter((ts) => !Number.isNaN(ts));
  if (timestamps.length === 0) return { firstSeen, lastSeen };
  return { firstSeen: Math.min(...timestamps), lastSeen: Math.max(...timestamps) };
}

/**
 * Balance and activity cards of the address header, read from the polled
 * aggregate. They used to be rendered once from the server response while
 * the transaction list below refreshed itself, so an incoming transfer
 * showed up in the list under the balance from before it.
 */
export default function LiveAddressStats({
  address,
  initialAggregate,
}: LiveAddressStatsProps): JSX.Element {
  const { data } = useAddressAggregate(address, initialAggregate);

  // The handler sends a balance with every successful response. Should one
  // arrive without it, the card falls back to the server-rendered balance.
  const polled = data.address?.balance;
  const balance = typeof polled === 'number' ? polled : (initialAggregate.address?.balance ?? 0);

  // First activity can only be as early as the earliest one ever seen, and
  // last activity only as late as the latest. Merging the server-rendered
  // range with the polled one keeps a single poll without the server-side
  // range (the handler answers 0 when that lookup fails) from moving
  // "First Activity" to the oldest row of the newest page.
  const seeded = activityRange(initialAggregate);
  const live = activityRange(data);
  const firstSeen =
    seeded.firstSeen && live.firstSeen
      ? Math.min(seeded.firstSeen, live.firstSeen)
      : seeded.firstSeen || live.firstSeen;
  const lastSeen = Math.max(seeded.lastSeen, live.lastSeen);

  return (
    <>
      <BalanceDisplay balance={balance} />
      <ActivityDisplay firstSeen={firstSeen} lastSeen={lastSeen} />
    </>
  );
}
