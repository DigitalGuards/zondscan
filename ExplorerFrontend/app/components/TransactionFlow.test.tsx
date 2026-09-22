import { renderToStaticMarkup } from 'react-dom/server';
import type { TransactionDetails } from '../types';
import TransactionFlow from './TransactionFlow';

const transaction: TransactionDetails = {
  hash: `0x${'1'.repeat(64)}`,
  from: `Q${'a'.repeat(128)}`,
  to: `Q${'b'.repeat(128)}`,
  value: '0x0',
  timestamp: 0,
  receiptStatus: '0x1',
};
const contractCreated = {
  address: `Q${'c'.repeat(128)}`,
  isToken: false,
  name: '',
  symbol: '',
  decimals: 0,
};

describe('TransactionFlow', () => {
  it('preserves full sender and recipient links, accessible addresses and copy labels', () => {
    const html = renderToStaticMarkup(<TransactionFlow transaction={transaction} />);
    expect(html).toContain('aria-label="Transaction flow"');
    for (const address of [transaction.from, transaction.to]) {
      expect(html).toContain(`href="/address/${address}"`);
      expect(html).toContain(`title="${address}"`);
      expect(html).toContain(`class="sr-only">${address}</span>`);
    }
    expect(html).toContain('aria-label="Copy sender address"');
    expect(html).toContain('aria-label="Copy recipient address"');
    expect(html).not.toMatch(/>IN<|>OUT<|>Sent<|>Received</);
  });

  it('keeps both endpoints for a self-transfer', () => {
    const html = renderToStaticMarkup(
      <TransactionFlow transaction={{ ...transaction, to: transaction.from }} />
    );
    expect(html.split(`href="/address/${transaction.from}"`)).toHaveLength(3);
  });

  it('links a directly created contract', () => {
    const html = renderToStaticMarkup(
      <TransactionFlow transaction={{ ...transaction, to: '', contractCreated }} />
    );
    expect(html).toContain(`href="/address/${contractCreated.address}"`);
    expect(html).toContain('aria-label="Copy contract address"');
  });

  it('retains the actual factory recipient when it creates a child contract', () => {
    const html = renderToStaticMarkup(
      <TransactionFlow transaction={{ ...transaction, contractCreated }} />
    );
    expect(html).toContain(`href="/address/${transaction.to}"`);
    expect(html).not.toContain(contractCreated.address);
    expect(html).toContain('aria-label="Copy recipient address"');
  });

  it('shows an unavailable deployment address without an empty link or copy action', () => {
    const html = renderToStaticMarkup(<TransactionFlow transaction={{ ...transaction, to: '' }} />);
    expect(html).toContain('Address unavailable');
    expect(html).not.toContain('href="/address/"');
    expect(html).not.toContain('Copy contract address');
  });

  it('does not link a created contract for a reverted deployment', () => {
    const html = renderToStaticMarkup(
      <TransactionFlow
        transaction={{ ...transaction, to: '', contractCreated, receiptStatus: '0x0' }}
      />
    );
    expect(html).toContain('No contract created');
    expect(html).not.toContain(contractCreated.address);
    expect(html).not.toContain('Copy contract address');
  });
});
