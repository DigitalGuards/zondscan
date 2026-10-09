import { isArray, isRecord, InvalidInputError } from './guards';

export interface ListedContract {
  _id?: string;
  creatorAddress: string;
  address: string;
  name?: string;
  symbol?: string;
  decimals?: number;
  totalSupply?: string;
  creationBlockNumber?: string;
  isToken: boolean;
  tokenStandard?: string;
  metadataName?: string;
  metadataImage?: string;
}

function isListedContract(value: unknown): value is ListedContract {
  return (
    isRecord(value) &&
    (value._id === undefined || typeof value._id === 'string') &&
    typeof value.creatorAddress === 'string' &&
    typeof value.address === 'string' &&
    typeof value.isToken === 'boolean' &&
    [
      'name',
      'symbol',
      'totalSupply',
      'creationBlockNumber',
      'tokenStandard',
      'metadataName',
      'metadataImage',
    ].every((key) => value[key] === undefined || typeof value[key] === 'string') &&
    (value.decimals === undefined || typeof value.decimals === 'number')
  );
}

export interface ContractsResponse {
  response: ListedContract[];
  total: number;
}

export function parseContractsResponse(value: unknown): ContractsResponse {
  if (
    !isRecord(value) ||
    !isArray(value.response) ||
    !value.response.every(isListedContract) ||
    typeof value.total !== 'number' ||
    !Number.isSafeInteger(value.total) ||
    value.total < 0
  ) {
    throw new InvalidInputError('Invalid contracts response');
  }
  return { response: value.response, total: value.total };
}

export interface EpochItem {
  epoch: string;
  timestamp: number;
  status: string;
  validatorsCount: number;
  activeCount: number;
  totalStaked: string;
}

export interface EpochsData {
  epochs: EpochItem[];
  total: number;
  finalizedEpoch: string;
  justifiedEpoch: string;
}

function isEpochItem(value: unknown): value is EpochItem {
  return (
    isRecord(value) &&
    typeof value.epoch === 'string' &&
    typeof value.timestamp === 'number' &&
    typeof value.status === 'string' &&
    typeof value.validatorsCount === 'number' &&
    typeof value.activeCount === 'number' &&
    typeof value.totalStaked === 'string'
  );
}

export function parseEpochsData(value: unknown): EpochsData {
  if (
    !isRecord(value) ||
    !isArray(value.epochs) ||
    !value.epochs.every(isEpochItem) ||
    typeof value.total !== 'number' ||
    !Number.isSafeInteger(value.total) ||
    value.total < 0 ||
    typeof value.finalizedEpoch !== 'string' ||
    typeof value.justifiedEpoch !== 'string'
  ) {
    throw new InvalidInputError('Invalid epochs response');
  }
  return {
    epochs: value.epochs,
    total: value.total,
    finalizedEpoch: value.finalizedEpoch,
    justifiedEpoch: value.justifiedEpoch,
  };
}
