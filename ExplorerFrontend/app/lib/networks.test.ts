import {
  assertNetworkCapability,
  explorerOrigin,
  readNetworkConfig,
} from '../../network-config.cjs';
import { explorerNetworks } from './networks';
import { translate } from './i18n';

test('legacy deployments keep v2 selected and v3 upcoming without extra configuration', () => {
  expect(readNetworkConfig({})).toEqual({
    network: 'v2',
    v2Url: 'https://zondscan.com/',
    v3Url: null,
    v3Available: false,
  });
  const networks = explorerNetworks(readNetworkConfig({}));
  expect(networks.find((network) => network.current)?.id).toBe('v2');
  expect(networks.find((network) => network.id === 'v3')?.href).toBeNull();
});

test('v3 only becomes a selectable root after both URL and availability are explicit', () => {
  const env = { NEXT_PUBLIC_V3_EXPLORER_URL: 'https://v3.example.test' };
  expect(explorerNetworks(readNetworkConfig(env))[1].href).toBeNull();
  expect(
    explorerNetworks(readNetworkConfig({ ...env, NEXT_PUBLIC_V3_EXPLORER_AVAILABLE: 'true' }))[1]
      .href
  ).toBe('https://v3.example.test/');
  expect(() => readNetworkConfig({ NEXT_PUBLIC_V3_EXPLORER_AVAILABLE: 'true' })).toThrow();
});

test.each([
  { EXPLORER_NETWORK: 'mainnet' },
  { EXPLORER_NETWORK: 'v2', NEXT_PUBLIC_EXPLORER_NETWORK: 'v3' },
  { NEXT_PUBLIC_V3_EXPLORER_AVAILABLE: 'yes' },
  { NEXT_PUBLIC_V3_EXPLORER_URL: 'https://zondscan.com' },
  { EXPLORER_NETWORK: 'v3' },
])('rejects ambiguous network configuration %j', (env) => {
  expect(() => readNetworkConfig(env)).toThrow();
});

test.each([
  '/v3',
  'javascript:alert(1)',
  'http://example.test',
  'https://example.test/tx/hash',
  'https://example.test/?network=v3',
  'https://example.test/#v3',
  'https://user:secret@example.test',
])('rejects unsafe or state-carrying network destinations: %s', (url) => {
  expect(() => explorerOrigin(url, 'test')).toThrow();
});

test('permits loopback origins for independent local deployments', () => {
  expect(explorerOrigin('http://127.0.0.1:19001', 'test')).toBe('http://127.0.0.1:19001/');
  expect(explorerOrigin('http://[::1]:19001', 'test')).toBe('http://[::1]:19001/');
});

test('the legacy binary fails closed for v3 until the reviewed 64-byte port is built', () => {
  expect(() => assertNetworkCapability('v2')).not.toThrow();
  expect(() => assertNetworkCapability('v3')).toThrow('20-byte frontend cannot serve Testnet v3');
  expect(() => assertNetworkCapability('v2', 64)).toThrow();
});

test.each(['zh', 'es', 'ru'] as const)(
  'network labels and status are localized in %s',
  (locale) => {
    for (const label of ['QRL Testnet v2', 'QRL Testnet v3', 'Upcoming']) {
      expect(translate(label, locale)).not.toBe(label);
    }
    expect(
      translate('Network: {network}', locale, { network: translate('QRL Testnet v3', locale) })
    ).not.toContain('{network}');
  }
);
