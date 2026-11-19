import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { renderToStaticMarkup } from 'react-dom/server';
import { useDisplayCurrency } from './useDisplayCurrency';

jest.mock('./PreferencesProvider', () => ({
  usePreferences: () => ({ preferences: { locale: 'en', currency: 'EUR' } }),
}));

afterEach(() => {
  jest.useRealTimers();
});

function Probe() {
  return <span>{useDisplayCurrency().status}</span>;
}

// Every page header reads the exchange rates. A server render that leaves a
// timer behind keeps that render's query client in memory until the timer
// fires, so the server's memory grows with every page it renders.
it('leaves no timer behind when a page that shows fiat amounts is rendered on the server', () => {
  jest.useFakeTimers();

  const html = renderToStaticMarkup(
    <QueryClientProvider client={new QueryClient()}>
      <Probe />
    </QueryClientProvider>
  );

  expect(html).toBe('<span>loading</span>');
  expect(jest.getTimerCount()).toBe(0);
});
