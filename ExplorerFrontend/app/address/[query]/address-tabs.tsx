'use client';

import { useCallback, useState } from 'react';
import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { useQuery } from '@tanstack/react-query';
import axios from 'axios';
import type { InternalTransaction, Transaction } from '@/app/types';
import config from '../../../config';
import TransactionsPanel from './panels/transactions-panel';
import InternalPanel from './panels/internal-panel';
import TokenTransfersPanel from './panels/token-transfers-panel';
import TokensPanel from './panels/tokens-panel';
import NftsPanel from './panels/nfts-panel';

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
 *   - router.replace (not push) so tab clicks don't pollute browser
 *     history. Back-button returns to the previous page, not the previous
 *     tab. Matches the codebase pattern (TransactionsList, blocks-client).
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

interface AddressTabsProps {
  address: string;
  transactions: Transaction[];
  /** True total from `addressData.transactions_count` (CountTransactions
   *  on the backend), independent of the paginated array length. */
  transactionsCount: number;
  internalt: InternalTransaction[];
}

export default function AddressTabs({
  address,
  transactions: initialTransactions,
  transactionsCount: initialTransactionsCount,
  internalt: initialInternalt,
}: AddressTabsProps): JSX.Element {
  const router = useRouter();
  const pathname = usePathname();
  const searchParams = useSearchParams();

  // The server rendered page 1 of the native-tx feed + counts. Seed local
  // state from those props, then keep them live by polling the same
  // /address/aggregate endpoint client-side, so a transaction mined after
  // the user lands (e.g. one they just sent from the wallet) appears on its
  // own instead of requiring a manual page refresh.
  //
  // Safe to seed once via the initializer: the parent (address-view) passes
  // key={addressSegment}, so this whole component unmounts + remounts on
  // address change and the seed re-runs with the new holder's data.
  const [transactions, setTransactions] = useState(initialTransactions);
  const [transactionsCount, setTransactionsCount] = useState(initialTransactionsCount);
  const [internalt, setInternalt] = useState(initialInternalt);

  // Visibility-aware polling, same TanStack Query discipline as the
  // home/gas/validators pages (backgrounded tabs go quiet). The interval
  // sits just above the backend's 10s aggregate cache so each poll has a
  // chance at fresh data rather than re-reading the same cached snapshot.
  // refetchOnMount:'always' fires one client fetch right after hydration,
  // which bypasses the page's 10s ISR and surfaces a tx that landed between
  // the (possibly cached) server render and the user arriving.
  useQuery<number>({
    queryKey: ['address-aggregate', address],
    queryFn: async () => {
      const res = await axios.get(
        `${config.handlerUrl}/address/aggregate/${address}`,
        { params: { page: 1, limit: 10 } },
      );
      const data = res.data ?? {};
      if (Array.isArray(data.transactions_by_address)) {
        // Mirror the gas-field normalization the server page applies so the
        // CSV export and any downstream readers see the same row shape.
        setTransactions(
          data.transactions_by_address.map((tx: any) => ({
            ...tx,
            gasUsedStr:
              tx.gasUsedStr || (tx.gasUsed ? `0x${tx.gasUsed.toString(16)}` : '0x0'),
            gasPriceStr:
              tx.gasPriceStr || (tx.gasPrice ? `0x${tx.gasPrice.toString(16)}` : '0x0'),
          })),
        );
      }
      if (typeof data.transactions_count === 'number') {
        setTransactionsCount(data.transactions_count);
      }
      if (Array.isArray(data.internal_transactions_by_address)) {
        setInternalt(data.internal_transactions_by_address);
      }
      return Date.now();
    },
    refetchInterval: 15000,
    refetchIntervalInBackground: false,
    refetchOnMount: 'always',
    staleTime: 0,
  });

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

  const setTab = useCallback(
    (key: TabKey) => {
      setMountedTabs((prev) => (prev.has(key) ? prev : new Set(prev).add(key)));
      router.replace(`${pathname}?tab=${key}`, { scroll: false });
    },
    [router, pathname],
  );

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
        // Aggregate doesn't return a true count for internal txns and
        // there's no dedicated endpoint. Leave unbadged rather than show
        // a clipped .length.
        return null;
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

      <div
        role="tablist"
        aria-label="Address activity sections"
        className="flex gap-1 border-b border-[#3d3d3d] overflow-x-auto"
      >
        {TAB_KEYS.map((key) => {
          const active = key === activeTab;
          const b = badge(key);
          return (
            <button
              key={key}
              id={`tab-${key}`}
              role="tab"
              aria-selected={active}
              aria-controls={`tabpanel-${key}`}
              onClick={() => setTab(key)}
              type="button"
              className={`whitespace-nowrap px-3 md:px-4 py-2 text-xs md:text-sm border-b-2 -mb-px transition-colors ${
                active
                  ? 'border-[#ffa729] text-[#ffa729]'
                  : 'border-transparent text-gray-400 hover:text-gray-200'
              }`}
            >
              {TAB_LABEL[key]}
              {b !== null && (
                <span className="ml-1 text-xs opacity-75">({b})</span>
              )}
            </button>
          );
        })}
      </div>

      <div className="overflow-hidden rounded-xl border border-[#3d3d3d]">
        {isPanelMounted('transactions') && (
          <div
            role="tabpanel"
            id="tabpanel-transactions"
            aria-labelledby="tab-transactions"
            hidden={activeTab !== 'transactions'}
          >
            <TransactionsPanel transactions={transactions} />
          </div>
        )}
        {isPanelMounted('internal') && (
          <div
            role="tabpanel"
            id="tabpanel-internal"
            aria-labelledby="tab-internal"
            hidden={activeTab !== 'internal'}
          >
            <InternalPanel internalt={internalt} />
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
