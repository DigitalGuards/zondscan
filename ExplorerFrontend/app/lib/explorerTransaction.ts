import { CURRENT_EXPLORER_CHAIN_ID_HEX } from './navigation';
import { canonicalizeQrlAddress } from './qrlAddress';

const V3_GENESIS_HASH = '0xd15407991193e6c23b733dc6bf9c628deaff8f9b6e252aa0d60030952b3e3ea4';

interface TransactionProvider {
  request(args: { method: string; params?: unknown[] }): Promise<unknown>;
}

/** Bind each write to this explorer's qualified network immediately before
 * requesting wallet approval. The wallet also enforces the explicit chainId. */
export async function sendExplorerTransaction(
  provider: TransactionProvider,
  tx: Record<string, string>
): Promise<string> {
  const expectedChainId = CURRENT_EXPLORER_CHAIN_ID_HEX;
  if (expectedChainId !== '0x301825') {
    throw new Error('Contract writes are unavailable for this explorer network');
  }
  const from = canonicalizeQrlAddress(tx.from);
  const to = canonicalizeQrlAddress(tx.to);
  if (!from || !to) throw new Error('Contract writes require current QIP-55 addresses');
  if (tx.chainId !== undefined && tx.chainId !== expectedChainId) {
    throw new Error('Transaction chain does not match this explorer');
  }
  const [chainId, genesis] = await Promise.all([
    provider.request({ method: 'qrl_chainId', params: [] }),
    provider.request({ method: 'qrl_getBlockByNumber', params: ['0x0', false] }),
  ]);
  if (
    typeof chainId !== 'string' ||
    !/^0x[0-9a-fA-F]+$/.test(chainId) ||
    BigInt(chainId) !== BigInt(expectedChainId)
  ) {
    throw new Error('Wallet network mismatch: select QRL Testnet v3 (Private)');
  }
  if (
    typeof genesis !== 'object' ||
    genesis === null ||
    !('number' in genesis) ||
    genesis.number !== '0x0' ||
    !('hash' in genesis) ||
    typeof genesis.hash !== 'string' ||
    genesis.hash.toLowerCase() !== V3_GENESIS_HASH
  ) {
    throw new Error('Wallet genesis does not match this explorer network');
  }
  const result = await provider.request({
    method: 'qrl_sendTransaction',
    params: [{ ...tx, from, to, chainId: expectedChainId }],
  });
  if (typeof result !== 'string' || !/^0x[0-9a-fA-F]{64}$/.test(result)) {
    throw new Error('Wallet returned an invalid transaction hash');
  }
  return result;
}
