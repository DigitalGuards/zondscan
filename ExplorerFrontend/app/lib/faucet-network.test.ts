import { databaseNetworkConfig } from './database-network';
import { verifyFaucetRpcNetwork } from './faucet-network';

const genesisHash = `0x${'a'.repeat(64)}`;
const config = databaseNetworkConfig({
  EXPECTED_CHAIN_ID: '1337',
  EXPECTED_GENESIS_HASH: genesisHash,
});

test('unpinned legacy faucet does not add RPC requirements', async () => {
  const read = { chainId: jest.fn(), genesis: jest.fn() };
  await expect(verifyFaucetRpcNetwork(databaseNetworkConfig({}), read)).resolves.toBeUndefined();
  expect(read.chainId).not.toHaveBeenCalled();
});
test('verified chain ID is returned for inclusion in the signed transaction', async () => {
  await expect(
    verifyFaucetRpcNetwork(config, {
      chainId: async () => BigInt(1337),
      genesis: async () => ({ hash: genesisHash }),
    })
  ).resolves.toBe('1337');
});
test.each([
  ['0x53a', { hash: genesisHash }],
  ['0x539', { hash: `0x${'b'.repeat(64)}` }],
  ['0x539', null],
  [null, { hash: genesisHash }],
])('refuses signing with wrong or missing RPC identity %j', async (chainId, genesis) => {
  await expect(
    verifyFaucetRpcNetwork(config, {
      chainId: async () => chainId,
      genesis: async () => genesis,
    })
  ).rejects.toThrow();
});
test('RPC errors fail closed', async () => {
  await expect(
    verifyFaucetRpcNetwork(config, {
      chainId: async () => {
        throw new Error('RPC unavailable');
      },
      genesis: async () => ({ hash: genesisHash }),
    })
  ).rejects.toThrow('RPC unavailable');
});
test('stalled RPC verification has a bounded timeout', async () => {
  jest.useFakeTimers();
  try {
    const result = verifyFaucetRpcNetwork(config, {
      chainId: () => new Promise(() => {}),
      genesis: () => new Promise(() => {}),
    });
    const assertion = expect(result).rejects.toThrow('timed out');
    await jest.advanceTimersByTimeAsync(5000);
    await assertion;
  } finally {
    jest.useRealTimers();
  }
});
