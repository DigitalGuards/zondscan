import { renderToStaticMarkup } from 'react-dom/server';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import LiveAddressStats, { activityRange } from './live-address-stats';
import type { AddressAggregateResponse } from './address-tabs';
import { formatAmount } from '../../lib/helpers';
import type { Transaction } from '@/app/types';

jest.mock('../../components/TimeDisplay', () => ({
  __esModule: true,
  default: ({ timestamp }: { timestamp: number | null }) => <span>time:{timestamp ?? 'Unknown'}</span>,
}));

const ADDRESS = 'Q' + 'ab'.repeat(64);

/** Renders the stats. `cached` is what a poll has put in the shared cache
 *  entry, the one the transaction list reads too. */
function render(initial: AddressAggregateResponse, cached?: AddressAggregateResponse): string {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  if (cached) client.setQueryData(['address-aggregate', ADDRESS], cached);
  const html = renderToStaticMarkup(
    <QueryClientProvider client={client}>
      <LiveAddressStats address={ADDRESS} initialAggregate={initial} />
    </QueryClientProvider>
  );
  client.clear();
  return html;
}

const balanceText = (amount: number): string => `>${formatAmount(amount)[0]}<`;

describe('LiveAddressStats', () => {
  it('shows the server-rendered balance before any poll', () => {
    const html = render({ address: { balance: 12.5 } });
    expect(html).toContain(balanceText(12.5));
    expect(html).toContain('No transactions were signed from this wallet yet.');
  });

  it('shows the polled balance and activity once a poll has landed', () => {
    // The page was rendered for an empty address; a transfer arrived since.
    const html = render(
      { address: { balance: 0 }, first_seen: 0, last_seen: 0, transactions_count: 0 },
      { address: { balance: 100 }, first_seen: 1794900000, last_seen: 1794900000, transactions_count: 1 }
    );
    expect(html).toContain(balanceText(100));
    expect(html).not.toContain(balanceText(0));
    expect(html).toContain('time:1794900000');
    expect(html).not.toContain('No transactions were signed from this wallet yet.');
  });

  it('falls back to the server-rendered balance when a response carries none', () => {
    const html = render({ address: { balance: 7 } }, { transactions_count: 0 });
    expect(html).toContain(balanceText(7));
  });

  it('keeps the first activity when one poll lacks the server-side range', () => {
    // The handler answers first_seen 0 when its range lookup fails; the page
    // rows then only reach back to the oldest row of the newest page.
    const row: Partial<Transaction> = { TimeStamp: 1794800000 };
    const html = render(
      { address: { balance: 1 }, first_seen: 1700000000, last_seen: 1794800000 },
      { address: { balance: 1 }, first_seen: 0, last_seen: 0, transactions_by_address: [row as Transaction] }
    );
    expect(html).toContain('time:1700000000');
    expect(html).toContain('time:1794800000');
  });

  it('moves the last activity forward when a newer transaction is polled', () => {
    const html = render(
      { address: { balance: 1 }, first_seen: 1700000000, last_seen: 1794800000 },
      { address: { balance: 2 }, first_seen: 1700000000, last_seen: 1794900000 }
    );
    expect(html).toContain('time:1700000000');
    expect(html).toContain('time:1794900000');
    expect(html).not.toContain('time:1794800000');
  });
});

describe('activityRange', () => {
  const tx = (timestamp: number): Transaction => {
    const row: Partial<Transaction> = { TimeStamp: timestamp };
    return row as Transaction;
  };

  it('prefers the range the server computed over the loaded page', () => {
    expect(
      activityRange({ first_seen: 10, last_seen: 90, transactions_by_address: [tx(50), tx(60)] })
    ).toEqual({ firstSeen: 10, lastSeen: 90 });
  });

  it('derives the range from the loaded page against a handler without it', () => {
    expect(activityRange({ transactions_by_address: [tx(50), tx(20), tx(60)] })).toEqual({
      firstSeen: 20,
      lastSeen: 60,
    });
  });

  it('reports no activity for an address without transactions', () => {
    expect(activityRange({ transactions_by_address: [] })).toEqual({ firstSeen: 0, lastSeen: 0 });
    expect(activityRange({})).toEqual({ firstSeen: 0, lastSeen: 0 });
  });
});
