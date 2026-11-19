jest.mock('./PreferencesProvider', () => ({
  usePreferences: () => ({ preferences: { locale: 'en', currency: 'EUR' } }),
}));

afterEach(() => {
  jest.useRealTimers();
});

// Every page header reads the exchange rates. A server render that leaves a
// timer behind keeps that render's query client in memory until the timer
// fires, so the server's memory grows with every page it renders.
it('leaves no timer behind when a page that shows fiat amounts is rendered on the server', () => {
  jest.useFakeTimers();
  const React = jest.requireActual<typeof import('react')>('react');
  const { renderToStaticMarkup } =
    jest.requireActual<typeof import('react-dom/server')>('react-dom/server');
  const { QueryClient, QueryClientProvider } =
    jest.requireActual<typeof import('@tanstack/react-query')>('@tanstack/react-query');
  const { useDisplayCurrency } =
    jest.requireActual<typeof import('./useDisplayCurrency')>('./useDisplayCurrency');

  function Probe() {
    return React.createElement('span', null, useDisplayCurrency().status);
  }

  const html = renderToStaticMarkup(
    React.createElement(QueryClientProvider, { client: new QueryClient() }, React.createElement(Probe))
  );

  expect(html).toBe('<span>loading</span>');
  expect(jest.getTimerCount()).toBe(0);
});
