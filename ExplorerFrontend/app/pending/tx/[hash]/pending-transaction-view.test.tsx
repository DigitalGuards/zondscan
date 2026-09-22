import { renderToStaticMarkup } from 'react-dom/server';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import type { ContractMeta, PendingTransaction } from '../../../types';
import PendingTransactionView from './pending-transaction-view';

jest.mock('next/navigation', () => ({ useRouter: () => ({ replace: jest.fn() }) }));

const transaction: PendingTransaction = {
  hash: `0x${'1'.repeat(64)}`,
  from: `Q${'a'.repeat(128)}`,
  to: `Q${'b'.repeat(128)}`,
  input: '0x',
  value: '0x2386f26fc10000',
  gas: '0x5208',
  gasPrice: '0x3b9aca00',
  nonce: '0x1',
  status: 'pending',
  createdAt: 1789043400,
  lastSeen: 1789043400,
  accessList: [],
  blockHash: null,
  chainId: '0x301825',
  publicKey: '',
  transactionIndex: null,
  type: '0x2',
};

function render(overrides: Partial<PendingTransaction> = {}, targetContract?: ContractMeta) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const html = renderToStaticMarkup(
    <QueryClientProvider client={client}>
      <PendingTransactionView
        pendingTx={{ ...transaction, ...overrides }}
        targetContract={targetContract}
      />
    </QueryClientProvider>
  );
  client.clear();
  return html;
}

it('shares the mined detail card and flow while clearly preserving pending status', () => {
  const html = render();
  expect(html).toContain('class="detail-content"');
  expect(html).toContain('id="tx-detail-heading"');
  expect(html).toContain('Transaction Details');
  expect(html).toContain('Transfer');
  expect(html).toContain('bg-surface-2 text-text-secondary border-border');
  expect(html).toContain('aria-label="Transaction flow"');
  expect(html).toContain('Awaiting block inclusion');
  expect(html).toContain('>Pending</span>');
  for (const address of [transaction.from, transaction.to])
    expect(html).toContain(`/address/${address}`);
  expect(html).toContain('Copy sender address');
  expect(html).toContain('Copy recipient address');
  for (const absent of [
    'Transaction Fee',
    '>Timestamp<',
    '>Block<',
    'Confirmation',
    '>Confirmed<',
  ]) {
    expect(html).not.toContain(absent);
  }
  expect(html).toContain('Gas Price');
  expect(html).toContain('Gas Limit');
});

it.each(['', undefined])(
  'keeps pending creation %p address unavailable until confirmation',
  (to) => {
    const html = render({ to, input: '0x6000' });
    expect(html).toContain('Contract creation');
    expect(html).toContain('Available after confirmation');
    expect(html).not.toContain('Copy contract address');
    expect(html).not.toContain('No contract created');
    expect(html).not.toContain('href="/address/"');
  }
);

it('retains generic classification for unknown calldata', () => {
  const html = render({ input: '0x12345678' });
  expect(html).toContain('>Transaction</span>');
  expect(html).not.toContain('>Contract call</span>');
  expect(html).toContain('Input Data');
});

it('preserves known contract classification without requiring a receipt', () => {
  expect(render({}, {})).toContain('>Contract call</span>');
});

it('preserves decoded token intent separately from the actual recipient contract', () => {
  const tokenRecipient = 'c'.repeat(128);
  const html = render({ input: '0xa9059cbb' + tokenRecipient + 'a'.padStart(128, '0') }, {});
  expect(html).toContain('>Contract call</span>');
  expect(html).toContain('Token Transfer (Pending)');
  expect(html).toContain('QRC-20');
  expect(html).toContain('assumes 18 decimals');
  expect(html).toContain(`/address/${transaction.to}`);
  expect(html).toContain('transfer(address, uint256)');
});

it('keeps selector-like calldata neutral without contract evidence', () => {
  const html = render({ input: '0xa9059cbb' + 'c'.repeat(128) + 'a'.padStart(128, '0') });
  expect(html).toContain('>Transaction</span>');
  expect(html).not.toContain('>Contract call</span>');
  expect(html).toContain('Token Transfer (Pending)');
});
