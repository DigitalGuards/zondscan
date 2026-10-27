import { isRecord } from './guards';
import config from '../../config';

export interface ChainTiming {
  slotsPerEpoch: number;
  secondsPerSlot: number;
  genesisTime: number;
}

export function isChainTiming(value: unknown): value is ChainTiming {
  return isRecord(value) && [value.slotsPerEpoch, value.secondsPerSlot, value.genesisTime]
    .every((n) => typeof n === 'number' && Number.isSafeInteger(n) && n > 0);
}

export async function fetchChainTiming(signal?: AbortSignal): Promise<ChainTiming> {
  const timeout = AbortSignal.timeout(10000);
  const response = await fetch(`${config.handlerUrl}/epoch`, {
    signal: signal ? AbortSignal.any([signal, timeout]) : timeout,
    cache: 'no-store',
  });
  if (!response.ok) throw new Error('Network timing unavailable');
  const value: unknown = await response.json();
  if (!isChainTiming(value)) throw new Error('Network timing unavailable');
  return value;
}

// Execution timestamps identify beacon slots even after missed proposals.
export function epochAtTimestamp(timestamp: number, timing: ChainTiming | null): number | null {
  if (!timing || !isChainTiming(timing) || !Number.isSafeInteger(timestamp)) return null;
  const elapsed = timestamp - timing.genesisTime;
  if (elapsed < 0 || elapsed % timing.secondsPerSlot !== 0) return null;
  return Math.floor(elapsed / timing.secondsPerSlot / timing.slotsPerEpoch);
}
