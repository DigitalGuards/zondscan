import { parseContractsResponse, parseEpochsData } from './listResponses';

describe('list response boundaries', () => {
  it('accepts the contract response without a database ID', () => {
    const row = { address: 'contract', creatorAddress: 'creator', isToken: false };
    expect(parseContractsResponse({ response: [row], total: 1 })).toEqual({
      response: [row],
      total: 1,
    });
  });

  it.each([
    null,
    { response: [null], total: 1 },
    { response: [], total: '1' },
    {
      response: [{ address: 'contract', creatorAddress: 'creator', isToken: false, decimals: {} }],
      total: 1,
    },
  ])('rejects malformed contract response %p', (value) => {
    expect(() => parseContractsResponse(value)).toThrow('Invalid contracts response');
  });

  it('accepts epoch rows and preserves all values', () => {
    const value = {
      epochs: [
        {
          epoch: '1',
          timestamp: 2,
          status: 'finalized',
          validatorsCount: 3,
          activeCount: 3,
          totalStaked: '1000',
        },
      ],
      total: 1,
      finalizedEpoch: '1',
      justifiedEpoch: '1',
    };
    expect(parseEpochsData(value)).toEqual(value);
  });

  it.each([null, [], { epochs: [null], total: 1, finalizedEpoch: '1', justifiedEpoch: '1' }])(
    'rejects malformed epoch response %p',
    (value) => {
      expect(() => parseEpochsData(value)).toThrow('Invalid epochs response');
    }
  );
});
