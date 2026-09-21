import type { AnchorHTMLAttributes } from 'react';

let mockLocale = 'en';
let mockGasUnavailable = false;
const originalNetwork = process.env.NEXT_PUBLIC_EXPLORER_NETWORK;

jest.mock('next/navigation', () => ({ usePathname: () => '/' }));
jest.mock('next/image', () => () => null);
jest.mock('next/link', () => ({
  __esModule: true,
  default: ({ children, ...props }: AnchorHTMLAttributes<HTMLAnchorElement>) =>
    jest.requireActual<typeof import('react')>('react').createElement('a', props, children),
}));
jest.mock('./PreferencesProvider', () => ({
  usePreferences: () => ({ preferences: { locale: mockLocale, currency: 'USD' } }),
}));
jest.mock('./AppearanceMenu', () => () => null);
jest.mock('./NetworkMenu', () => () => null);
jest.mock('./SearchBar', () => () => null);
jest.mock('./MobileNavigation', () => () => null);
jest.mock('./NavigationLink', () => () => null);
jest.mock('@tanstack/react-query', () => ({
  useQuery: ({ queryKey }: { queryKey: string[] }) => {
    if (queryKey[0] === 'header-gas') {
      return {
        data: mockGasUnavailable ? undefined : { avgGasPriceHex: '0x7' },
        isError: mockGasUnavailable,
      };
    }
    return { data: { currentPrice: 1, priceChange24h: 0 }, isError: false };
  },
}));

beforeEach(() => {
  jest.resetModules();
  mockLocale = 'en';
  mockGasUnavailable = false;
});

afterEach(() => {
  if (originalNetwork === undefined) delete process.env.NEXT_PUBLIC_EXPLORER_NETWORK;
  else process.env.NEXT_PUBLIC_EXPLORER_NETWORK = originalNetwork;
});

function renderHeader(network: string): {
  subtitle: string | undefined;
  gasTitle: string | undefined;
} {
  process.env.NEXT_PUBLIC_EXPLORER_NETWORK = network;
  // Reload React, its renderer, and the real components together after changing
  // the build-time environment so all hooks use the same module instance.
  const React = jest.requireActual<typeof import('react')>('react');
  const { renderToStaticMarkup } =
    jest.requireActual<typeof import('react-dom/server')>('react-dom/server');
  const { default: SiteHeader } = jest.requireActual<typeof import('./SiteHeader')>('./SiteHeader');
  const html = renderToStaticMarkup(React.createElement(SiteHeader));
  const subtitle = html.match(/>ZondScan<\/span>\s*<span[^>]*>([^<]*)<\/span>/)?.[1];
  const gasAnchor = html.match(/<a\b(?=[^>]*href="\/gas")[^>]*>/)?.[0];
  return { subtitle, gasTitle: gasAnchor?.match(/\btitle="([^"]*)"/)?.[1] };
}

it.each([
  ['testnet-v2', 'en', 'QRL Testnet v3', 'QRL Testnet v3'],
  ['testnet-v3', 'en', 'QRL Testnet v3', 'QRL Testnet v3'],
  ['testnet-v2', 'es', 'Red de pruebas QRL v3', 'QRL Testnet v3'],
  ['testnet-v3', 'es', 'Red de pruebas QRL v3', 'QRL Testnet v3'],
])(
  'renders the %s header and gas network label with %s preferences',
  (network, locale, subtitle, gasName) => {
    mockLocale = locale;
    const rendered = renderHeader(network);
    expect(rendered.subtitle).toBe(subtitle);
    expect(rendered.gasTitle).toBe(`Average recent transaction gas price on ${gasName}: 7 Planck`);
  }
);

it.each(['testnet-v2', 'testnet-v3'])('preserves unavailable gas feedback on %s', (network) => {
  mockGasUnavailable = true;
  expect(renderHeader(network).gasTitle).toBe('Testnet gas price is temporarily unavailable');
});
