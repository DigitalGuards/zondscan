import { InvalidInputError, isArray, isOneOf, isRecord } from './guards';
import type { ChainTiming } from './chainTiming';

export const CHANGE_KINDS = ['activated', 'exited', 'slashed', 'ejected'] as const;
export type ChangeKind = (typeof CHANGE_KINDS)[number];

export const CHANGE_KIND_LABELS: Record<ChangeKind, string> = {
  activated: 'Activated',
  exited: 'Exited',
  slashed: 'Slashed',
  ejected: 'Ejected (low balance)',
};

export interface SlashingEvent {
  id: string;
  slot: number;
  epoch: number;
  type: 'proposer' | 'attester';
  validatorIndices: string[];
  whistleblower: string;
  conflictingRoots: string[];
}

export interface ValidatorSetChange {
  id: string;
  epoch: number;
  kind: ChangeKind;
  validatorIndex: string;
  pubkey: string;
}

interface ActivityPage {
  total: number;
  page: number;
  limit: number;
  backfillComplete: boolean;
}

export interface SlashingsData extends ActivityPage {
  events: SlashingEvent[];
  slashedValidatorCount: number;
}

export interface ValidatorSetChangesData extends ActivityPage {
  changes: ValidatorSetChange[];
}

function isCount(value: unknown): value is number {
  return typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;
}

function isIndex(value: unknown): value is string {
  return typeof value === 'string' && /^\d+$/.test(value);
}

export function isSlashingEvent(value: unknown): value is SlashingEvent {
  return (
    isRecord(value) &&
    typeof value.id === 'string' &&
    isCount(value.slot) &&
    isCount(value.epoch) &&
    isOneOf(value.type, ['proposer', 'attester']) &&
    isArray(value.validatorIndices) &&
    value.validatorIndices.every(isIndex) &&
    value.validatorIndices.length > 0 &&
    isIndex(value.whistleblower) &&
    isArray(value.conflictingRoots) &&
    value.conflictingRoots.length === 2 &&
    value.conflictingRoots.every(
      (root) => typeof root === 'string' && /^0x[0-9a-f]{64}$/.test(root),
    )
  );
}

export function isValidatorSetChange(value: unknown): value is ValidatorSetChange {
  return (
    isRecord(value) &&
    typeof value.id === 'string' &&
    isCount(value.epoch) &&
    isOneOf(value.kind, CHANGE_KINDS) &&
    isIndex(value.validatorIndex) &&
    typeof value.pubkey === 'string'
  );
}

function isActivityPage(value: unknown): value is ActivityPage & Record<string, unknown> {
  return (
    isRecord(value) &&
    isCount(value.total) &&
    isCount(value.page) &&
    value.page >= 1 &&
    isCount(value.limit) &&
    value.limit >= 1 &&
    typeof value.backfillComplete === 'boolean'
  );
}

export function parseSlashings(value: unknown): SlashingsData {
  if (
    !isActivityPage(value) ||
    !isArray(value.events) ||
    !value.events.every(isSlashingEvent) ||
    !isCount(value.slashedValidatorCount)
  )
    throw new InvalidInputError('Invalid slashings response');
  return {
    ...value,
    events: value.events,
    slashedValidatorCount: value.slashedValidatorCount,
  };
}

export function parseValidatorSetChanges(value: unknown): ValidatorSetChangesData {
  if (
    !isActivityPage(value) ||
    !isArray(value.changes) ||
    !value.changes.every(isValidatorSetChange)
  ) {
    throw new InvalidInputError('Invalid validator set changes response');
  }
  return { ...value, changes: value.changes };
}

export function activityTimestamp(
  value: number,
  unit: 'slot' | 'epoch',
  timing?: ChainTiming,
): number | null {
  if (!timing) return null;
  const slot = value * (unit === 'epoch' ? timing.slotsPerEpoch : 1);
  const timestamp = timing.genesisTime + slot * timing.secondsPerSlot;
  return Number.isSafeInteger(timestamp) && timestamp >= 0 && timestamp <= 8_640_000_000_000
    ? timestamp
    : null;
}

export function slashingEmptyMessage(data: SlashingsData, validatorIndex?: string): string {
  if (validatorIndex !== undefined)
    return 'No slashing events have been indexed for this validator.';
  if (data.total > 0) return 'No slashing events on this page.';
  if (data.slashedValidatorCount > 0)
    return 'Slashed validators have been recorded. Their slashing evidence has not been indexed.';
  return 'No validator has been slashed on this network yet.';
}
