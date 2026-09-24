import { assertDatabaseNetwork, type DatabaseNetworkConfig } from './database-network';

/** Include the same-origin browser proxy in the deployment identity checks. */
export function backendNetworkUrls(
  privateUrl: string,
  publicUrl: string | undefined,
  explorerOrigin: string
): string[] {
  try {
    const browserUrl = new URL(publicUrl || '/api', explorerOrigin);
    if (
      !['http:', 'https:'].includes(browserUrl.protocol) ||
      browserUrl.username ||
      browserUrl.password ||
      browserUrl.search ||
      browserUrl.hash
    ) {
      throw new Error('Invalid API URL');
    }
    return [privateUrl, browserUrl.href.replace(/\/$/, '')];
  } catch {
    throw new Error('Browser API requires a valid HTTP(S) endpoint');
  }
}

/** Pinned deployments must read from APIs serving their exact chain identity. */
export async function verifyBackendNetwork(
  config: DatabaseNetworkConfig,
  handlerUrls: string[],
  request: typeof fetch = fetch
): Promise<void> {
  if (!config.chainId) return;
  for (const handlerUrl of new Set(handlerUrls)) {
    try {
      const response = await request(`${handlerUrl.replace(/\/$/, '')}/network`, {
        cache: 'no-store',
        redirect: 'error',
        signal: AbortSignal.timeout(5000),
      });
      if (!response.ok) throw new Error('Identity unavailable');
      assertDatabaseNetwork(await response.json(), config);
    } catch {
      throw new Error('Explorer API network identity is unavailable or mismatched');
    }
  }
}
