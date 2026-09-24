import { readNetworkConfig, type ExplorerNetworkConfig } from '../../network-config.cjs';

// These explicit public accesses are replaced by Next at build time. Network
// identity never comes from browser storage, request parameters, or a cookie.
export const NETWORK_CONFIG = readNetworkConfig({
  NEXT_PUBLIC_EXPLORER_NETWORK: process.env.NEXT_PUBLIC_EXPLORER_NETWORK,
  NEXT_PUBLIC_V2_EXPLORER_URL: process.env.NEXT_PUBLIC_V2_EXPLORER_URL,
  NEXT_PUBLIC_V3_EXPLORER_URL: process.env.NEXT_PUBLIC_V3_EXPLORER_URL,
  NEXT_PUBLIC_V3_EXPLORER_AVAILABLE: process.env.NEXT_PUBLIC_V3_EXPLORER_AVAILABLE,
});

export const CURRENT_NETWORK_NAME = `QRL Testnet ${NETWORK_CONFIG.network}`;

export function explorerNetworks(config: ExplorerNetworkConfig) {
  return [
    { id: 'v2', name: 'QRL Testnet v2', current: config.network === 'v2', href: config.v2Url },
    {
      id: 'v3',
      name: 'QRL Testnet v3',
      current: config.network === 'v3',
      href: config.v3Available ? config.v3Url : null,
    },
    { id: 'mainnet', name: 'QRL Mainnet', current: false, href: null },
  ];
}
