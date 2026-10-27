import { renderToStaticMarkup } from 'react-dom/server';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import GasClient from './gas-client';

jest.mock('../components/useDisplayCurrency', () => ({
  useDisplayCurrency: () => ({ format: () => '$0.00' }),
}));
jest.mock('../components/PreferencesProvider', () => ({
  usePreferences: () => ({ preferences: { timeZone: 'utc' } }),
}));

it.each([null, 60])('renders available gas data with block timing %s', (avgBlockTimeSec) => {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  client.setQueryData(['gas-summary'], {
    avgGasPriceHex: '0x3b9aca00',
    avgGasUsedHex: '0x5208',
    avgGasLimitHex: '0x100000',
    avgBlockTimeSec,
    pendingCount: 7,
    lastBlockNumberHex: '0x2a',
    lastGasUsedHex: '0x5208',
    lastGasLimitHex: '0x100000',
    gasPriceHistogram: [],
  });
  client.setQueryData(['gas-history', '24h'], { range: '24h', data: [] });
  try {
    const html = renderToStaticMarkup(<QueryClientProvider client={client}><GasClient /></QueryClientProvider>);
    expect(html).toContain('Avg Gas Price');
    expect(html).toContain('1.00 Shor');
    expect(html).toContain('Mempool Size');
    expect(html).toContain('>7<');
    expect(html).toMatch(avgBlockTimeSec === null ? /Avg Block Time<\/p><p[^>]*>-<\/p>/ : /Avg Block Time<\/p><p[^>]*>60\.0s<\/p>/);
  } finally {
    client.clear();
  }
});
