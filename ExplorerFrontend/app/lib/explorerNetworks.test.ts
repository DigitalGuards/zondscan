type NavigationConfig = typeof import('./navigation');

const originalNetwork = process.env.NEXT_PUBLIC_EXPLORER_NETWORK;

afterEach(() => {
  if (originalNetwork === undefined) delete process.env.NEXT_PUBLIC_EXPLORER_NETWORK;
  else process.env.NEXT_PUBLIC_EXPLORER_NETWORK = originalNetwork;
});

function loadNetwork(value: string | undefined): NavigationConfig {
  if (value === undefined) delete process.env.NEXT_PUBLIC_EXPLORER_NETWORK;
  else process.env.NEXT_PUBLIC_EXPLORER_NETWORK = value;
  let config: NavigationConfig | undefined;
  jest.isolateModules(() => {
    config = jest.requireActual<NavigationConfig>('./navigation');
  });
  if (!config) throw new Error('Network configuration did not load');
  return config;
}

it.each([
  [undefined, 'testnet-v2', 'QRL Testnet v2'],
  ['', 'testnet-v2', 'QRL Testnet v2'],
  ['testnet-v2', 'testnet-v2', 'QRL Testnet v2'],
  ['testnet-v3', 'testnet-v3', 'QRL Testnet v3'],
  ['mainnet', 'mainnet', 'QRL Mainnet'],
  ['unknown-network', 'testnet-v2', 'QRL Testnet v2'],
])('resolves build network %s to %s and its matching display label', (value, id, name) => {
  const config = loadNetwork(value);
  expect(config.CURRENT_EXPLORER_NETWORK).toBe(id);
  expect(config.CURRENT_EXPLORER_NETWORK_NAME).toBe(name);
  expect(config.EXPLORER_NETWORKS).toEqual([
    {
      id: 'testnet-v2',
      name: 'QRL Testnet v2',
      href: 'https://zondscan.com',
      status: id === 'testnet-v2' ? 'active' : 'live',
    },
    {
      id: 'testnet-v3',
      name: 'QRL Testnet v3',
      href: 'https://v3.zondscan.com',
      status: id === 'testnet-v3' ? 'active' : 'live',
    },
    {
      id: 'mainnet',
      name: 'QRL Mainnet',
      href: null,
      status: id === 'mainnet' ? 'active' : 'planned',
    },
  ]);
  expect(config.EXPLORER_NETWORKS.filter((network) => network.status === 'active')).toHaveLength(1);
});
