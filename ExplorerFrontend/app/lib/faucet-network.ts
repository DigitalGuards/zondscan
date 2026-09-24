import type { DatabaseNetworkConfig } from './database-network';

/** Read the signing node for every pinned claim, so a changed RPC cannot reuse
 * a previously successful identity check. The result also pins the signed tx. */
export async function verifyFaucetRpcNetwork(
  config: DatabaseNetworkConfig,
  read: { chainId: () => Promise<unknown>; genesis: () => Promise<unknown> }
): Promise<string | undefined> {
  if (!config.chainId) return undefined;
  let timer: ReturnType<typeof setTimeout> | undefined;
  try {
    const [chainId, genesis] = await Promise.race([
      Promise.all([read.chainId(), read.genesis()]),
      new Promise<never>((_, reject) => {
        timer = setTimeout(() => reject(new Error('Faucet network verification timed out')), 5000);
      }),
    ]);
    const chainText = String(chainId);
    const hash =
      genesis && typeof genesis === 'object'
        ? (genesis as Record<string, unknown>).hash
        : undefined;
    if (
      !/^(?:[0-9]+|0x[0-9a-f]+)$/i.test(chainText) ||
      BigInt(chainText).toString() !== config.chainId ||
      typeof hash !== 'string' ||
      hash.toLowerCase() !== config.genesisHash
    ) {
      throw new Error('Faucet RPC belongs to a different chain');
    }
    return config.chainId;
  } finally {
    if (timer) clearTimeout(timer);
  }
}
