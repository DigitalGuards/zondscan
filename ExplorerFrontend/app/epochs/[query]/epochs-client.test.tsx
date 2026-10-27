import { renderToStaticMarkup } from 'react-dom/server';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import EpochsClient from './epochs-client';

jest.mock('next/navigation', () => ({ useRouter: () => ({ push: jest.fn() }) }));
jest.mock('../../components/SearchBar', () => ({ __esModule: true, default: () => null }));
jest.mock('../../components/TimeDisplay', () => ({ __esModule: true, default: () => null }));

it('keeps unknown epoch list values as dashes', () => {
  const client = new QueryClient();
  const html = renderToStaticMarkup(<QueryClientProvider client={client}><EpochsClient initialPage="1" initialData={{
    epochs: [{ epoch: '5000', timestamp: 1792982760, status: 'finalized', validatorsCount: null, activeCount: null, totalStaked: null }],
    total: 1, finalizedEpoch: '5000', justifiedEpoch: '5001',
  }} /></QueryClientProvider>);
  expect(html.match(/>-<\/td>/g)).toHaveLength(3);
  expect(html).not.toContain('>0 Quanta<');
  client.clear();
});
