import 'server-only';

import { assertNetworkCapability, readNetworkConfig } from '../../network-config.cjs';
import config from '../../config';
import { NETWORK_CONFIG } from './networks';
import { databaseNetworkConfig, validateDatabaseUri } from './database-network';
import { backendNetworkUrls, verifyBackendNetwork } from './backend-network';

/** Standalone servers also validate runtime identity against the compiled UI. */
export async function validateDeployment(): Promise<void> {
  const deployment = readNetworkConfig(process.env);
  assertNetworkCapability(deployment.network);
  if (deployment.network !== NETWORK_CONFIG.network) {
    throw new Error('Runtime explorer network differs from the built frontend; rebuild required');
  }
  const database = databaseNetworkConfig(process.env);
  if (process.env.DATABASE_URL) validateDatabaseUri(process.env.DATABASE_URL, database.database);
  // Builds validate configuration without requiring an online backend. Startup
  // verifies identity before the server begins serving a pinned deployment.
  if (process.env.NEXT_PHASE === 'phase-production-build') return;
  const origin = deployment.network === 'v2' ? deployment.v2Url : deployment.v3Url!;
  const urls = backendNetworkUrls(config.handlerUrl, process.env.NEXT_PUBLIC_HANDLER_URL, origin);
  await verifyBackendNetwork(database, urls);
}
