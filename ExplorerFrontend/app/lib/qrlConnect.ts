'use client';

import { QRLConnect, ConnectionStatus } from '@qrlwallet/connect';
import type { QRLConnectProvider } from '@qrlwallet/connect';

// Module-level singleton. The QRLConnect constructor announces itself via
// EIP-6963 and opens a relay socket, instantiating twice would double the
// announcements and stomp on the persisted-session storage key. Always go
// through getQrlConnect().
let instance: QRLConnectProvider | null = null;

// Default relay URL: the same backend that hosts the dApp example consumes
// the explorer's host as the relay endpoint, so anyone landing on
// zondscan.com pairs against the same relay that powers qrlwallet.com.
// VITE-style override matches what dapp-example uses.
function getRelayUrl(): string {
  if (typeof window !== 'undefined') {
    // Allow per-deploy override via a global injected at runtime (none today,
    // but cheap insurance for ops). Defaults to qrlwallet.com.
    const w = window as unknown as { __QRL_RELAY_URL__?: string };
    if (w.__QRL_RELAY_URL__) return w.__QRL_RELAY_URL__;
  }
  return 'https://qrlwallet.com';
}

export function getQrlConnect(): QRLConnectProvider {
  if (instance) return instance;
  if (typeof window === 'undefined') {
    throw new Error('getQrlConnect() called on the server; this module is client-only');
  }
  instance = new QRLConnect({
    dappMetadata: {
      name: 'Zondscan',
      url: window.location.origin,
    },
    relayUrl: getRelayUrl(),
    autoReconnect: true,
  });
  return instance;
}

export { ConnectionStatus };
export type { QRLConnectProvider };
