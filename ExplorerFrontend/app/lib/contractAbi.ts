import { isArray, isOneOf, isRecord, InvalidInputError } from './guards';

export interface AbiInput {
  name: string;
  type: string;
  components?: AbiInput[];
}

export interface AbiFunction {
  type: 'function';
  name: string;
  stateMutability: 'view' | 'pure' | 'nonpayable' | 'payable';
  inputs: AbiInput[];
  outputs?: AbiInput[];
}

function isAbiInput(value: unknown): value is AbiInput {
  return (
    isRecord(value) &&
    typeof value.name === 'string' &&
    typeof value.type === 'string' &&
    (value.components === undefined ||
      (isArray(value.components) && value.components.every(isAbiInput)))
  );
}

export function isAbiFunction(value: unknown): value is AbiFunction {
  return (
    isRecord(value) &&
    value.type === 'function' &&
    typeof value.name === 'string' &&
    isOneOf(value.stateMutability, ['view', 'pure', 'nonpayable', 'payable']) &&
    isArray(value.inputs) &&
    value.inputs.every(isAbiInput) &&
    (value.outputs === undefined || (isArray(value.outputs) && value.outputs.every(isAbiInput)))
  );
}

/** Drop malformed function entries before displaying or encoding an interaction. */
export function parseAbiFunctions(json: string): AbiFunction[] {
  const value: unknown = JSON.parse(json);
  if (!isArray(value)) throw new InvalidInputError('Invalid contract ABI');
  return value.filter(isAbiFunction);
}

export interface ContractCallResponse {
  result?: string;
  error?: string;
  reverted?: boolean;
}

export function isContractCallResponse(value: unknown): value is ContractCallResponse {
  return (
    isRecord(value) &&
    (value.result === undefined ||
      (typeof value.result === 'string' && /^0x(?:[0-9a-fA-F]{2})*$/.test(value.result))) &&
    (value.error === undefined || typeof value.error === 'string') &&
    (value.reverted === undefined || typeof value.reverted === 'boolean')
  );
}
