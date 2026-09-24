import { databaseNetworkConfig } from './database-network';
import { backendNetworkUrls, verifyBackendNetwork } from './backend-network';

const genesisHash = `0x${'a'.repeat(64)}`;
const config = databaseNetworkConfig({
  EXPECTED_CHAIN_ID: '1337',
  EXPECTED_GENESIS_HASH: genesisHash,
});
const identity = {
  networkId: 'v2',
  chainId: '1337',
  genesisHash,
  addressBytes: 20,
  identityVerified: true,
};

test.each([undefined, '/api'])(
  'a wrong same-origin browser proxy fails startup (%s)',
  async (publicUrl) => {
    const request = jest
      .fn()
      .mockImplementation(async (url: string) =>
        Response.json(
          url.startsWith('http://api:8081') ? identity : { ...identity, networkId: 'v3' }
        )
      );
    const urls = backendNetworkUrls('http://api:8081', publicUrl, 'https://example.test/');
    await expect(verifyBackendNetwork(config, urls, request)).rejects.toThrow('network identity');
    expect(request.mock.calls.map(([url]) => url)).toEqual([
      'http://api:8081/network',
      'https://example.test/api/network',
    ]);
  }
);

test('an explicit public API is resolved independently of the explorer origin', () => {
  expect(
    backendNetworkUrls('http://api:8081', 'https://api.example.test/', 'https://example.test/')
  ).toEqual(['http://api:8081', 'https://api.example.test']);
});

test.each(['javascript:alert(1)', 'https://secret:password@example.test/api', '/api?network=v3'])(
  'invalid public API configuration is rejected without leaking its value',
  (url) => {
    expect(() => backendNetworkUrls('http://api:8081', url, 'https://example.test/')).toThrow(
      'Browser API requires a valid HTTP(S) endpoint'
    );
  }
);

test('unpinned legacy frontend remains compatible with its existing API', async () => {
  const request = jest.fn();
  await verifyBackendNetwork(databaseNetworkConfig({}), ['http://api:8081'], request);
  expect(request).not.toHaveBeenCalled();
});
test('verifies both private API and explicit public browser API overrides', async () => {
  const request = jest.fn().mockImplementation(async () => Response.json(identity));
  await verifyBackendNetwork(
    config,
    ['http://api:8081', 'https://example.test/api', 'http://api:8081'],
    request
  );
  expect(request.mock.calls.map(([url]) => url)).toEqual([
    'http://api:8081/network',
    'https://example.test/api/network',
  ]);
});
test.each([
  {},
  { ...identity, networkId: 'v3' },
  { ...identity, chainId: '1338' },
  { ...identity, genesisHash: `0x${'b'.repeat(64)}` },
  { ...identity, identityVerified: false },
])('a mismatched backend refuses frontend startup %j', async (data) => {
  await expect(
    verifyBackendNetwork(config, ['http://api:8081'], async () => Response.json(data))
  ).rejects.toThrow('network identity');
});
test('an unavailable API fails closed without exposing its internal URL', async () => {
  await expect(
    verifyBackendNetwork(config, ['http://private-api:8081'], async () => {
      throw new Error('Connection failed to http://private-api:8081');
    })
  ).rejects.toThrow('Explorer API network identity is unavailable or mismatched');
});
