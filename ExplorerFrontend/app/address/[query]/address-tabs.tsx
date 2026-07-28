'use client';

import { useCallback, useMemo, useState } from 'react';
import { useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import axios from 'axios';
import type { InternalTransaction, Transaction } from '@/app/types';
import config from '../../../config';
import TabPillBar from '../../components/TabPillBar';
import TransactionsPanel from './panels/transactions-panel';
import InternalPanel from './panels/internal-panel';
import TokenTransfersPanel from './panels/token-transfers-panel';
import TokensPanel from './panels/tokens-panel';
import NftsPanel from './panels/nfts-panel';
import { setUrlParams } from '../../lib/use-url-param';

/**
 * Single unified tab bar that replaces the old stacked HoldingsDisplay +
 * TanStackTable sections under the address header. Active tab persists in
 * the URL via `?tab=`. Each panel is mounted lazily on first activation and
 * then kept in the React tree (hidden via the `hidden` attribute), so
 * pagination + filter state survive tab switches without re-fetching.
 *
 * Source-of-truth choices, deliberate:
 *   - activeTab is derived from useSearchParams every render; no local
 *     useState mirror to drift out of sync.
 *   - setUrlParams replace (not push) so tab clicks don't pollute browser
 *     history: Back returns to the previous page, not the previous tab.
 *     The shallow History-API write avoids the RSC refetch router.replace
 *     caused and preserves the panels' page params (?txPage etc.) so Back
 *     can restore them.
 *   - Per-address reset is delegated to the parent: address-view passes
 *     key={addressSegment} so React unmounts + remounts this whole
 *     component on address change. All internal state (mountedTabs,
 *     counts) resets to initial automatically, no effects needed.
 *   - mountedTabs accumulates "tabs we've ever rendered" via a useRef,
 *     mutated during render. Refs don't trigger re-renders, so this
 *     doesn't run afoul of react-hooks/set-state-in-effect the way
 *     mirroring it into useState + an effect would.
 */

const TAB_KEYS = ['transactions', 'internal', 'token-transfers', 'tokens', 'nfts'] as const;
type TabKey = (typeof TAB_KEYS)[number];

const TAB_LABEL: Record<TabKey, string> = {
  transactions: 'Transactions',
  internal: 'Internal Txns',
  'token-transfers': 'Token Transfers',
  tokens: 'Tokens (QRC-20)',
  nfts: 'NFTs',
};

const parseTabKey = (raw: string | null): TabKey => {
  if (raw && (TAB_KEYS as readonly string[]).includes(raw)) {
    return raw as TabKey;
  }
  return 'transactions';
};

/** Render an integer gas quantity (decimal or 0x-hex string, or number) as a
 *  0x-hex string. BigInt avoids the precision loss Number() suffers above
 *  2^53, matching this repo's exact-integer handling of on-chain values. The
 *  try/catch keeps a malformed value from throwing inside render (BigInt
 *  rejects non-integer input, where Number would have yielded NaN). */
const toHexQuantity = (value: string | number): string => {
  try {
    return `0x${BigInt(value).toString(16)}`;
  } catch {
    return '0x0';
  }
};

interface AddressTabsProps {
  address: string;
  transactions: Transaction[];
  /** True total from `addressData.transactions_count` (CountTransactions
   *  on the backend), independent of the paginated array length. */
  transactionsCount: number;
  internalt: InternalTransaction[];
  /** True total from `addressData.internal_transactions_count`; undefined
   *  on handlers predating the field. */
  internalTransactionsCount?: number;
}

/** Subset of the `/address/aggregate` payload this component consumes. The
 *  server page seeds the first page via props; the client poll refetches the
 *  same fields to keep them live. */
interface AddressAggregateResponse {
  transactions_by_address?: Transaction[];
  transactions_count?: number;
  internal_transactions_by_address?: InternalTransaction[];
  internal_transactions_count?: number;
}

export default function AddressTabs({
  address,
  transactions: initialTransactions,
  transactionsCount: initialTransactionsCount,
  internalt: initialInternalt,
  internalTransactionsCount: initialInternalTransactionsCount,
}: AddressTabsProps): JSX.Element {
  const searchParams = useSearchParams();

  // Keep the native-tx feed + counts live by polling the same
  // /address/aggregate endpoint the server rendered, so a transaction mined
  // after the user lands (e.g. one they just sent from the wallet) appears on
  // its own instead of requiring a manual page refresh.
  //
  // The query owns the data: initialData seeds the cache from the server
  // props (first paint shows server data, no flash) and the values below are
  // derived straight from `data`, no useState mirror written inside queryFn.
  // Visibility-aware, same TanStack Query discipline as the home/gas/
  // validators pages (backgrounded tabs go quiet). The 15s interval sits just
  // above the backend's 10s aggregate cache so each poll has a chance at
  // fresh data. refetchOnMount:'always' fires one client fetch right after
  // hydration, bypassing the page's 10s ISR to surface a tx that landed
  // between the (possibly cached) server render and the user arriving.
  //
  // Per-address reset is free: the parent (address-view) passes
  // key={addressSegment}, remounting this component on address change.
  const { data } = useQuery<AddressAggregateResponse>({
    queryKey: ['address-aggregate', address],
    queryFn: async () => {
      const res = await axios.get<AddressAggregateResponse>(
        `${config.handlerUrl}/address/aggregate/${address}`,
        { params: { page: 1, limit: 10 } },
      );
      return res.data ?? {};
    },
    initialData: {
      transactions_by_address: initialTransactions,
      transactions_count: initialTransactionsCount,
      internal_transactions_by_address: initialInternalt,
      internal_transactions_count: initialInternalTransactionsCount,
    },
    refetchInterval: 15000,
    refetchIntervalInBackground: false,
    refetchOnMount: 'always',
    staleTime: 0,
  });

  // Normalize gas fields to hex the same way the server page does so the CSV
  // export sees a consistent row shape. Rows that already carry a server-set
  // gasUsedStr/gasPriceStr keep it (the `||` short-circuits); only fresh
  // polled rows missing it get a computed fallback.
  const transactions = useMemo<Transaction[]>(
    () =>
      (data?.transactions_by_address ?? []).map((tx) => ({
        ...tx,
        gasUsedStr: tx.gasUsedStr || (tx.gasUsed ? toHexQuantity(tx.gasUsed) : '0x0'),
        gasPriceStr: tx.gasPriceStr || (tx.gasPrice ? toHexQuantity(tx.gasPrice) : '0x0'),
      })),
    [data?.transactions_by_address],
  );
  const transactionsCount = data?.transactions_count ?? 0;
  const internalt = data?.internal_transactions_by_address ?? [];
  const internalTransactionsCount = data?.internal_transactions_count;

  const activeTab = parseTabKey(searchParams?.get('tab') ?? null);

  // Lazy mount: each tab the user has visited stays in the React tree
  // (just hidden) so its panel-local state (filter, pagination, fetched
  // data) survives tab switches without re-mounting / re-fetching. The
  // set is updated synchronously inside the click handler instead of via
  // an effect, so it doesn't trip react-hooks/set-state-in-effect. The
  // parent forces a full remount on address change via key={address}, so
  // we don't need to clear this set ourselves.
  const [mountedTabs, setMountedTabs] = useState<Set<TabKey>>(
    () => new Set([activeTab]),
  );

  const setTab = useCallback((key: TabKey) => {
    setMountedTabs((prev) => (prev.has(key) ? prev : new Set(prev).add(key)));
    setUrlParams({ tab: key }, 'replace');
  }, []);

  // If the URL ever flips to a tab the click handler didn't go through
  // (theoretically possible on some external navigation), fall back to
  // mounting the active tab on the fly via this OR-with-activeTab check
  // at each panel guard below.
  const isPanelMounted = (key: TabKey): boolean =>
    mountedTabs.has(key) || key === activeTab;

  // Server-reported counts the panels surface back up to the tab bar so
  // the badges show honest totals (not capped array lengths). No reset
  // effect needed: parent's key={address} forces this component to
  // unmount + remount on address change, taking these states with it.
  const [tokenTransfersTotal, setTokenTransfersTotal] = useState<number | null>(null);
  const [tokensCount, setTokensCount] = useState<number | null>(null);
  const [nftsCount, setNftsCount] = useState<number | null>(null);

  const badge = (key: TabKey): string | null => {
    switch (key) {
      case 'transactions':
        return transactionsCount.toLocaleString('en-US');
      case 'internal':
        // Honest server-side count when the handler provides it; unbadged
        // (rather than a clipped .length) against older handlers.
        return typeof internalTransactionsCount === 'number'
          ? internalTransactionsCount.toLocaleString('en-US')
          : null;
      case 'token-transfers':
        return tokenTransfersTotal === null
          ? null
          : tokenTransfersTotal.toLocaleString('en-US');
      case 'tokens':
        return tokensCount === null ? null : String(tokensCount);
      case 'nfts':
        return nftsCount === null ? null : String(nftsCount);
    }
  };

  return (
    <section
      aria-labelledby="address-activity-heading"
      className="space-y-3 md:space-y-4"
    >
      <h2 id="address-activity-heading" className="sr-only">
        Address activity
      </h2>

      <TabPillBar
        ariaLabel="Address activity sections"
        withPanelIds
        activeKey={activeTab}
        onSelect={setTab}
        tabs={TAB_KEYS.map((key) => ({
          key,
          label: TAB_LABEL[key],
          badge: badge(key),
        }))}
      />

      <div className="overflow-hidden rounded-xl border border-border">
        {isPanelMounted('transactions') && (
          <div
            role="tabpanel"
            id="tabpanel-transactions"
            aria-labelledby="tab-transactions"
            hidden={activeTab !== 'transactions'}
          >
            <TransactionsPanel
              address={address}
              transactions={transactions}
              total={transactionsCount}
            />
          </div>
        )}
        {isPanelMounted('internal') && (
          <div
            role="tabpanel"
            id="tabpanel-internal"
            aria-labelledby="tab-internal"
            hidden={activeTab !== 'internal'}
          >
            <InternalPanel
              address={address}
              internalt={internalt}
              total={internalTransactionsCount}
            />
          </div>
        )}
        {isPanelMounted('token-transfers') && (
          <div
            role="tabpanel"
            id="tabpanel-token-transfers"
            aria-labelledby="tab-token-transfers"
            hidden={activeTab !== 'token-transfers'}
          >
            <TokenTransfersPanel address={address} onLoaded={setTokenTransfersTotal} />
          </div>
        )}
        {isPanelMounted('tokens') && (
          <div
            role="tabpanel"
            id="tabpanel-tokens"
            aria-labelledby="tab-tokens"
            hidden={activeTab !== 'tokens'}
          >
            <TokensPanel address={address} onLoaded={setTokensCount} />
          </div>
        )}
        {isPanelMounted('nfts') && (
          <div
            role="tabpanel"
            id="tabpanel-nfts"
            aria-labelledby="tab-nfts"
            hidden={activeTab !== 'nfts'}
          >
            <NftsPanel address={address} onLoaded={setNftsCount} />
          </div>
        )}
      </div>
    </section>
  );
}
