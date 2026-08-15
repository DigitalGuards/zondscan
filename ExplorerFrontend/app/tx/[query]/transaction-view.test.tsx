import { renderToStaticMarkup } from 'react-dom/server';
import TransactionView from './transaction-view';
import type { TransactionDetails } from '@/app/types';

jest.mock('next/navigation', () => ({
  useSearchParams: () => new URLSearchParams(),
}));

jest.mock('@tanstack/react-query', () => ({
  useQuery: () => ({ data: undefined }),
}));

jest.mock('../../lib/hooks', () => ({
  useIsMobile: () => false,
}));

const confirmedTransaction: TransactionDetails = {
  hash: '0xd5e416fa509d3b157f6f5f160562cd4e4b076114f39bd7b97e1c22c0510c4dc9',
  blockNumber: 194338,
  latestBlock: 196247,
  from: 'Qc670e4e2d24db18ee19710eb4ece9dd3794d5740',
  to: 'Q75e6770674f9f954801c4d7d4cc0c8f8c2c3f1ea',
  value: '0x0',
  timestamp: 1786663596,
  gasUsed: '0x14b00',
  gasPrice: '0x1',
  receiptStatus: '0x1',
};

describe('TransactionView status', () => {
  it('renders one confirmed badge in the Status row', () => {
    const html = renderToStaticMarkup(
      <TransactionView transaction={confirmedTransaction} />,
    );

    expect(html.match(/>Confirmed</g)).toHaveLength(1);
    expect(html).toContain('>Status<');
    expect(html).toContain('1910 Confirmations');
  });
});
