import { createElement, type ReactNode } from 'react';
import { renderToStaticMarkup } from 'react-dom/server';
import NetworkMenu from './NetworkMenu';
import { NETWORK_CONFIG } from '../lib/networks';
import { readNetworkConfig } from '../../network-config.cjs';

// Keep the popover open for markup assertions; Playwright exercises its actual
// keyboard and pointer behavior with Headless UI in the browser.
jest.mock('@headlessui/react', () => ({
  Menu: ({ children }: { children: ReactNode }) => children,
  MenuButton: ({ children }: { children: ReactNode }) => createElement('button', {}, children),
  MenuItems: ({ children }: { children: ReactNode }) => children,
  MenuItem: ({ as = 'div', children, ...props }: { as?: string; children: ReactNode }) =>
    createElement(as, props, children),
}));
jest.mock('../lib/networks', () => {
  const actual = jest.requireActual('../lib/networks');
  return { ...actual, NETWORK_CONFIG: { ...actual.NETWORK_CONFIG } };
});

afterEach(() => Object.assign(NETWORK_CONFIG, readNetworkConfig({})));

test('upcoming v3 has no navigable destination', () => {
  const html = renderToStaticMarkup(<NetworkMenu />);
  expect(html).toContain('QRL Testnet v3');
  expect(html).toContain('Upcoming');
  expect(html).not.toContain('<a ');
});

test('available v3 renders a native root anchor for a fresh network document', () => {
  Object.assign(
    NETWORK_CONFIG,
    readNetworkConfig({
      NEXT_PUBLIC_V3_EXPLORER_URL: 'https://v3.example.test',
      NEXT_PUBLIC_V3_EXPLORER_AVAILABLE: 'true',
    })
  );
  const html = renderToStaticMarkup(<NetworkMenu />);
  expect(html).toContain('<a href="https://v3.example.test/"');
  expect(html).not.toContain('?');
  expect(html).not.toContain('/address/');
  expect(html).not.toContain('/tx/');
});
