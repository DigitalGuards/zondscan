export type ExplorerNetworkId = 'v2' | 'v3';
export interface ExplorerNetworkConfig {
  network: ExplorerNetworkId;
  v2Url: string;
  v3Url: string | null;
  v3Available: boolean;
}
export const NATIVE_ADDRESS_BYTES: number;
export function networkId(value?: string): ExplorerNetworkId;
export function explorerOrigin(value: string | undefined, name: string): string | null;
export function readNetworkConfig(env: Record<string, string | undefined>): ExplorerNetworkConfig;
export function assertNetworkCapability(network: ExplorerNetworkId, addressBytes?: number): void;
