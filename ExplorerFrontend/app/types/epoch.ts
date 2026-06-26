// Beacon-chain epoch snapshot surfaced by the backend and rendered on the
// home page, the validators page, and the EpochInfoPanel. Shared here so the
// three call sites stay in lock-step instead of each redeclaring the shape.
export interface EpochInfo {
  headEpoch: string;
  headSlot: string;
  finalizedEpoch: string;
  justifiedEpoch: string;
  slotsPerEpoch: number;
  secondsPerSlot: number;
  slotInEpoch: number;
  timeToNextEpoch: number;
  updatedAt: number;
}
