'use client';

import { keepPreviousData, useQuery } from '@tanstack/react-query';
import type { UseQueryResult } from '@tanstack/react-query';
import axios from 'axios';
import type { InternalTransaction, Transaction } from '@/app/types';
import config from '../../../../config';

/**
 * Paged access to `/address/aggregate/:addr` for the Transactions and
 * Internal Txns panels.
 *
 * The backend paginates both tx lists in the aggregate (default 10/page,
 * DB-layer cap 50) since the hot-address timeout fix, but the panels used
 * to client-paginate whatever single page the server component fetched,
 * silently clipping every address to its 10 most recent rows ("84
 * transactions" badge, one page of 10 rows). These helpers give the panels
 * real server-side paging.
 *
 * Page 1 stays owned by the parent AddressTabs poll (live 15s refresh);
 * this hook only fetches pages > 1, so the two panels share one cache
 * entry per (address, page) and switching back to page 1 is instant.
 */

/** Rows per page in the panels. Matches the backend default so page 1 from
 *  the parent poll and pages > 1 from this hook tile seamlessly. */
export const PAGE_LIMIT = 10;

/** Per-request row cap for full-history export; the aggregate's DB layer
 *  clamps limit to 50, so asking for more just wastes nothing. */
const EXPORT_PAGE_LIMIT = 50;

/** Upper bound on exported rows (40 requests at 50/page) so Download on a
 *  validator-grade address can't fan out unbounded requests. */
export const MAX_EXPORT_ROWS = 2000;

export interface AggregatePage {
  transactions_by_address?: Transaction[];
  internal_transactions_by_address?: InternalTransaction[];
  transactions_count?: number;
  internal_transactions_count?: number;
}

export async function fetchAggregatePage(
  address: string,
  page: number,
  limit: number,
): Promise<AggregatePage> {
  const res = await axios.get<AggregatePage>(
    `${config.handlerUrl}/address/aggregate/${address}`,
    { params: { page, limit } },
  );
  return res.data ?? {};
}

/** Fetch one page of the aggregate. Disabled for page 1 (parent-owned).
 *  keepPreviousData holds the prior page on screen while the next loads,
 *  so paging doesn't flash an empty table. */
export function useAggregatePage(
  address: string,
  page: number,
): UseQueryResult<AggregatePage> {
  return useQuery<AggregatePage>({
    queryKey: ['address-aggregate-page', address, page],
    queryFn: () => fetchAggregatePage(address, page, PAGE_LIMIT),
    enabled: page > 1,
    // Matches the backend's 10s route cache; refetching sooner can't
    // observe anything fresher.
    staleTime: 10_000,
    placeholderData: keepPreviousData,
  });
}

/** Walk every page of one of the aggregate's row lists (for CSV export).
 *  Stops on the first short page or at MAX_EXPORT_ROWS. */
export async function fetchAllAggregateRows<T>(
  address: string,
  select: (page: AggregatePage) => T[] | undefined,
): Promise<T[]> {
  const rows: T[] = [];
  for (let page = 1; rows.length < MAX_EXPORT_ROWS; page++) {
    const batch = select(await fetchAggregatePage(address, page, EXPORT_PAGE_LIMIT)) ?? [];
    rows.push(...batch);
    if (batch.length < EXPORT_PAGE_LIMIT) break;
  }
  return rows.slice(0, MAX_EXPORT_ROWS);
}
