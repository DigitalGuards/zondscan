import { renderToStaticMarkup } from 'react-dom/server';
import { BlockTable } from './home-client';

jest.mock('./components/TimeDisplay', () => ({ __esModule: true, default: () => null }));

it('shows the timestamp epoch after missed slots', () => {
  const timing = { genesisTime: 1790582760, slotsPerEpoch: 8, secondsPerSlot: 60 };
  const blocks = [{ number: '0x7b', hash: '0xaa', timestamp: String(timing.genesisTime + 40000 * 60), miner: '', transactions: [] }];
  const html = renderToStaticMarkup(<BlockTable blocks={blocks} status="ready" timing={timing} />);
  expect(html).toContain('/block/123');
  expect(html).toContain('epoch 5,000');
  const missing = renderToStaticMarkup(<BlockTable blocks={blocks} status="ready" timing={null} />);
  expect(missing).toContain('epoch -');
});
