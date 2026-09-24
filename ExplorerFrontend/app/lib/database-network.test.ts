import {
  assertDatabaseNetwork,
  databaseNetworkConfig,
  validateDatabaseUri,
} from './database-network';

const pins = { EXPECTED_CHAIN_ID: '0x539', EXPECTED_GENESIS_HASH: `0x${'a'.repeat(64)}` };
const v3 = { ...pins, EXPLORER_NETWORK: 'v3', MONGO_DB_NAME: 'qrldata-z-v3' };

test('v2 preserves its existing database and accepts an unmarked legacy database', () => {
  const config = databaseNetworkConfig({});
  expect(config.database).toBe('qrldata-z');
  expect(() => assertDatabaseNetwork(null, config)).not.toThrow();
});
test.each([
  { EXPLORER_NETWORK: 'v3' },
  { ...v3, MONGO_DB_NAME: 'qrldata-z' },
  { ...v3, MONGO_DB_NAME: 'QRLDATA-Z' },
  { ...v3, EXPECTED_CHAIN_ID: '' },
  { ...v3, EXPECTED_GENESIS_HASH: '' },
  { EXPECTED_CHAIN_ID: '1337' },
  { ...pins, EXPECTED_CHAIN_ID: '0' },
  { ...pins, EXPECTED_CHAIN_ID: '1e3' },
  { ...pins, EXPECTED_GENESIS_HASH: `0x${'0'.repeat(64)}` },
  { MONGO_DB_NAME: 'wrong/database' },
  { MONGO_DB_NAME: 'Admin' },
  { MONGO_DB_NAME: '_network' },
  { ...pins, EXPECTED_CHAIN_ID: (BigInt(2) ** BigInt(256)).toString() },
])('rejects unsafe database identity configuration %j', (env) => {
  expect(() => databaseNetworkConfig(env)).toThrow();
});
test('v3 requires an exact verified marker in its separate database', () => {
  const config = databaseNetworkConfig(v3);
  expect(config.chainId).toBe('1337');
  const marker = {
    networkId: 'v3',
    addressBytes: 64,
    identityVerified: true,
    chainId: '1337',
    genesisHash: pins.EXPECTED_GENESIS_HASH,
  };
  expect(() => assertDatabaseNetwork(marker, config)).not.toThrow();
  for (const invalid of [
    null,
    {},
    { ...marker, networkId: 'v2' },
    { ...marker, addressBytes: 20 },
    { ...marker, identityVerified: false },
    { ...marker, chainId: '1338' },
    { ...marker, genesisHash: `0x${'b'.repeat(64)}` },
  ]) {
    expect(() => assertDatabaseNetwork(invalid, config)).toThrow();
  }
});
test('a v2 frontend also refuses a database marked for v3', () => {
  expect(() =>
    assertDatabaseNetwork({ networkId: 'v3', addressBytes: 64 }, databaseNetworkConfig({}))
  ).toThrow();
});
test('a pinned v2 frontend requires a verified matching identity', () => {
  expect(() => assertDatabaseNetwork(null, databaseNetworkConfig(pins))).toThrow();
});
test('unpinned v2 markers still require a complete canonical identity', () => {
  const config = databaseNetworkConfig({});
  const marker = {
    networkId: 'v2',
    addressBytes: 20,
    chainId: '',
    genesisHash: '',
    identityVerified: false,
  };
  expect(() => assertDatabaseNetwork(marker, config)).not.toThrow();
  for (const invalid of [
    false,
    { networkId: 'v2', addressBytes: 20 },
    { ...marker, chainId: null },
    { ...marker, identityVerified: 'false' },
    { ...marker, chainId: '1337', genesisHash: pins.EXPECTED_GENESIS_HASH, identityVerified: true },
  ]) {
    expect(() => assertDatabaseNetwork(invalid, config)).toThrow();
  }
});
test('chain configuration normalizes whitespace, 0X prefixes, and leading zeros', () => {
  expect(
    databaseNetworkConfig({ ...pins, EXPECTED_CHAIN_ID: ' 0X0539 ', MONGO_DB_NAME: ' qrldata-z ' })
      .chainId
  ).toBe('1337');
});
test('URI database matches explicit database with multi-host and SRV support', () => {
  for (const uri of [
    'mongodb://db-one,db-two/qrldata-z-v3?replicaSet=test',
    'mongodb+srv://example.test/qrldata-z-v3',
    'mongodb://example.test',
  ]) {
    expect(() => validateDatabaseUri(uri, 'qrldata-z-v3')).not.toThrow();
  }
  expect(() => validateDatabaseUri('mongodb://example.test/qrldata-z', 'qrldata-z-v3')).toThrow();
  expect(() => validateDatabaseUri('mongodb://example.test/qrldata%2Dz', 'qrldata-z-v3')).toThrow();
  expect(() => validateDatabaseUri('https://example.test', 'qrldata-z-v3')).toThrow();
});
