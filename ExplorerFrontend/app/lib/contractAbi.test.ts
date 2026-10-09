import * as abi from '@theqrl/web3-qrl-abi';
import { isAbiFunction, isContractCallResponse, parseAbiFunctions } from './contractAbi';
import { decodeContractCall, decodeEventLog } from './helpers';

const transfer = {
  type: 'function',
  name: 'transfer',
  stateMutability: 'nonpayable',
  inputs: [
    { name: 'to', type: 'address' },
    { name: 'amount', type: 'uint256' },
  ],
  outputs: [{ name: '', type: 'bool' }],
} as const;

describe('ABI wire guards', () => {
  it('preserves the exact function and encoded transaction bytes', () => {
    const fn = parseAbiFunctions(JSON.stringify([transfer])).find(
      (entry) => entry.name === transfer.name
    );
    if (!fn) throw new Error('Missing validated function');
    expect(fn).toEqual(transfer);
    const address = `Q${'12'.repeat(64)}`;
    const amount = '123456789012345678901234567890';
    expect(abi.encodeFunctionCall(fn, [address, amount])).toBe(
      '0xa9059cbb' + address.slice(1) + BigInt(amount).toString(16).padStart(128, '0')
    );
  });

  it.each([
    null,
    [],
    false,
    { ...transfer, inputs: [null] },
    { ...transfer, inputs: [{ name: 'tuple', type: 'tuple', components: [false] }] },
    { ...transfer, outputs: [{ type: 42 }] },
    { ...transfer, stateMutability: 'invalid' },
  ])('drops malformed ABI entry %p before interaction', (value) => {
    expect(isAbiFunction(value)).toBe(false);
    expect(parseAbiFunctions(JSON.stringify([value]))).toEqual([]);
  });

  it.each(['null', '{}', 'false', 'invalid'])('rejects malformed ABI document %s', (json) => {
    expect(() => parseAbiFunctions(json)).toThrow();
  });

  it('validates read response fields and encoded hex bytes', () => {
    expect(isContractCallResponse({ result: '0x0123' })).toBe(true);
    expect(isContractCallResponse({ error: 'revert', reverted: true })).toBe(true);
    for (const value of [
      null,
      [],
      { result: {} },
      { result: '0x0' },
      { error: 3 },
      { reverted: 'false' },
    ]) {
      expect(isContractCallResponse(value)).toBe(false);
    }
  });

  it('drops malformed decoder entries without throwing', () => {
    for (const entry of [
      null,
      { type: 'function', inputs: [null] },
      { type: 'event', inputs: [4] },
    ]) {
      const json = JSON.stringify([entry]);
      expect(decodeContractCall('0x12345678', json)).toBeNull();
      expect(decodeEventLog([`0x${'ab'.repeat(32)}`], '0x', json)).toBeNull();
    }
  });
});
