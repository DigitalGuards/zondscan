import { renderToStaticMarkup } from 'react-dom/server';
import { EpochSlotRow, EpochValidatorSummary } from './epoch-detail-client';

jest.mock('../../components/TimeDisplay', () => ({
  __esModule: true,
  default: ({ timestamp }: { timestamp: number }) => <time>{timestamp}</time>,
}));

it('links a proposed slot to its execution block and validator zero', () => {
  const html = renderToStaticMarkup(
    <EpochSlotRow
      slot={{
        slot: 18,
        status: 'proposed',
        blockNumber: '12',
        blockHash: '0xabcdef',
        proposerIndex: '0',
        proposerPubkey: '0xaabb',
        transactions: 2,
      }}
    />,
  );
  expect(html).toContain('href="/block/12"');
  expect(html).toContain('href="/validators/0"');
  expect(html).toContain('>18</a>');
  expect(html).toContain('>0</a>');
  expect(html).not.toContain('/block/18');
  expect(html).not.toContain('/address/');
});

it('names the validator assigned to a missed slot without a block link', () => {
  const html = renderToStaticMarkup(
    <EpochSlotRow
      slot={{
        slot: 17,
        status: 'missed',
        proposerIndex: '42',
        proposerPubkey: '0xccdd',
        transactions: 0,
      }}
    />,
  );
  expect(html).toContain('href="/validators/42"');
  expect(html).toContain('>42</a>');
  expect(html).toContain('Missed');
  expect(html).not.toContain('/block/');
});

it.each(['proposed', 'missed'])(
  'keeps unknown %s proposers as a dash',
  (status) => {
    for (const proposerIndex of [undefined, '', ' ']) {
      const html = renderToStaticMarkup(
        <EpochSlotRow
          slot={{ slot: 19, status, proposerIndex, transactions: 0 }}
        />,
      );
      expect(html).toContain('class="text-text-muted">-</span>');
      expect(html).not.toContain('/validators/');
      expect(html).not.toContain('/block/');
    }
  },
);

it('renders every unavailable validator summary as a dash', () => {
  const html = renderToStaticMarkup(<EpochValidatorSummary data={{
    validatorsCount: null, activeCount: null, pendingCount: null,
    exitedCount: null, slashedCount: null, totalStaked: null,
  }} />);
  expect(html.match(/>-<\/span>/g)).toHaveLength(6);
  expect(html).not.toContain('>0<');
});

it('preserves known zero counts and stake', () => {
  const html = renderToStaticMarkup(<EpochValidatorSummary data={{
    validatorsCount: 0, activeCount: 0, pendingCount: 0,
    exitedCount: 0, slashedCount: 0, totalStaked: '0',
  }} />);
  expect(html.match(/>0<\/span>/g)).toHaveLength(5);
  expect(html).toContain('>0 Quanta</span>');
  expect(html).not.toContain('>-<');
});
