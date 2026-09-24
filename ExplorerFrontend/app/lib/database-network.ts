import { networkId, type ExplorerNetworkId } from '../../network-config.cjs';

export interface DatabaseNetworkConfig {
  network: ExplorerNetworkId;
  database: string;
  chainId: string;
  genesisHash: string;
  addressBytes: number;
}

export function databaseNetworkConfig(
  env: Record<string, string | undefined>
): DatabaseNetworkConfig {
  const network = networkId(env.EXPLORER_NETWORK || env.NEXT_PUBLIC_EXPLORER_NETWORK);
  if (
    env.EXPLORER_NETWORK &&
    env.NEXT_PUBLIC_EXPLORER_NETWORK &&
    networkId(env.EXPLORER_NETWORK) !== networkId(env.NEXT_PUBLIC_EXPLORER_NETWORK)
  ) {
    throw new Error('Server and public explorer network must match');
  }
  const database = (env.MONGO_DB_NAME || '').trim() || 'qrldata-z';
  if (
    !/^[a-zA-Z0-9][a-zA-Z0-9_-]{0,62}$/.test(database) ||
    ['admin', 'local', 'config'].includes(database.toLowerCase())
  )
    throw new Error('MONGO_DB_NAME is invalid');
  if (network === 'v3' && (!env.MONGO_DB_NAME?.trim() || database.toLowerCase() === 'qrldata-z')) {
    throw new Error('Testnet v3 requires an explicit separate MONGO_DB_NAME');
  }
  const rawChainId = (env.EXPECTED_CHAIN_ID || '').trim();
  let chainId = '';
  if (rawChainId) {
    if (!/^(?:[0-9]+|0x[0-9a-f]+)$/i.test(rawChainId) || rawChainId.replace(/^0x/i, '').length > 78)
      throw new Error('EXPECTED_CHAIN_ID is invalid');
    chainId = BigInt(rawChainId).toString();
    if (BigInt(chainId) <= BigInt(0) || BigInt(chainId) >= BigInt(2) ** BigInt(256))
      throw new Error('EXPECTED_CHAIN_ID must be a positive 256-bit value');
  }
  const genesisHash = (env.EXPECTED_GENESIS_HASH || '').trim().toLowerCase();
  if (genesisHash && (!/^0x[0-9a-f]{64}$/.test(genesisHash) || /^0x0+$/.test(genesisHash))) {
    throw new Error('EXPECTED_GENESIS_HASH must be a nonzero 32-byte hash');
  }
  if (Boolean(chainId) !== Boolean(genesisHash) || (network === 'v3' && !chainId)) {
    throw new Error('Both EXPECTED_CHAIN_ID and EXPECTED_GENESIS_HASH are required together');
  }
  return { network, database, chainId, genesisHash, addressBytes: network === 'v3' ? 64 : 20 };
}

/** The URI's default database must agree with the explicit database selection. */
export function validateDatabaseUri(uri: string, database: string): void {
  // mongodb supports multiple hosts in its authority, which WHATWG URL does not.
  const match = /^mongodb(?:\+srv)?:\/\/[^/?#]+(?:\/([^?#]*))?(?:\?[^#]*)?$/.exec(uri);
  if (!match) throw new Error('DATABASE_URL must be a MongoDB connection URI');
  let uriDatabase: string;
  try {
    uriDatabase = decodeURIComponent(match[1] || '');
  } catch {
    throw new Error('DATABASE_URL has an invalid database name');
  }
  if (uriDatabase && uriDatabase !== database) {
    throw new Error('DATABASE_URL database must match MONGO_DB_NAME');
  }
}

/** Never write faucet claims to a database marked as belonging to another chain. */
export function assertDatabaseNetwork(marker: unknown, config: DatabaseNetworkConfig): void {
  if (marker == null && config.network === 'v2' && !config.chainId) return;
  if (!marker || typeof marker !== 'object')
    throw new Error('Explorer database identity is missing');
  const record = marker as Record<string, unknown>;
  if (record.networkId !== config.network || record.addressBytes !== config.addressBytes) {
    throw new Error('Explorer database belongs to a different network');
  }
  if (
    record.identityVerified !== Boolean(config.chainId) ||
    record.chainId !== config.chainId ||
    record.genesisHash !== config.genesisHash
  ) {
    throw new Error('Explorer database chain identity does not match the configured pins');
  }
}
