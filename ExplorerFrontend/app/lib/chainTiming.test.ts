import { epochAtTimestamp, isChainTiming, fetchChainTiming } from './chainTiming';
import { epochsToDays } from './helpers';

const timing = { slotsPerEpoch: 8, secondsPerSlot: 60, genesisTime: 1790582760 };

it('derives chain epochs from timestamps across missed execution blocks', () => {
  expect(epochAtTimestamp(timing.genesisTime, timing)).toBe(0);
  expect(epochAtTimestamp(timing.genesisTime + 7 * 60, timing)).toBe(0);
  expect(epochAtTimestamp(timing.genesisTime + 8 * 60, timing)).toBe(1);
  expect(epochAtTimestamp(timing.genesisTime + 40007 * 60, timing)).toBe(5000);
  expect(epochAtTimestamp(timing.genesisTime + 40007 * 12, { ...timing, slotsPerEpoch: 32, secondsPerSlot: 12 })).toBe(1250);
});

it('leaves unknown or unaligned timing unknown', () => {
  expect(epochAtTimestamp(timing.genesisTime, null)).toBeNull();
  expect(epochAtTimestamp(timing.genesisTime - 1, timing)).toBeNull();
  expect(epochAtTimestamp(timing.genesisTime + 1, timing)).toBeNull();
  expect(isChainTiming({ ...timing, slotsPerEpoch: 0 })).toBe(false);
  expect(isChainTiming({ ...timing, secondsPerSlot: NaN })).toBe(false);
  expect(isChainTiming({ slotsPerEpoch: 8, secondsPerSlot: 60 })).toBe(false);
});

it('uses network timing for validator ages', () => {
  expect(epochsToDays(180, timing)).toBe(1);
  expect(epochsToDays(180, { ...timing, slotsPerEpoch: 32, secondsPerSlot: 12 })).toBe(0.8);
  expect(epochsToDays(180, null)).toBeNull();
});

it('requires timing from the API', async () => {
  const fetchMock = jest.spyOn(globalThis, 'fetch');
  try {
    fetchMock.mockResolvedValue(new Response(JSON.stringify(timing)));
    expect(await fetchChainTiming()).toEqual(timing);
    fetchMock.mockResolvedValue(new Response('{}'));
    await expect(fetchChainTiming()).rejects.toThrow('Network timing unavailable');
    fetchMock.mockResolvedValue(new Response('{}', { status: 500 }));
    await expect(fetchChainTiming()).rejects.toThrow('Network timing unavailable');
  } finally {
    fetchMock.mockRestore();
  }
});
