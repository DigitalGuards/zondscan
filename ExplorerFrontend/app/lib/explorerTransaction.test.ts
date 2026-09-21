import { sendExplorerTransaction } from './explorerTransaction';

const GENESIS = '0xd15407991193e6c23b733dc6bf9c628deaff8f9b6e252aa0d60030952b3e3ea4';
const HASH = `0x${'12'.repeat(32)}`;
const TX = { from: `Q${'1'.repeat(128)}`, to: `Q${'2'.repeat(128)}`, data: '0x01020304' };

function providerFixture(chain = '0x301825', genesis: unknown = { number: '0x0', hash: GENESIS }) {
  const request = jest.fn(async ({ method }: { method: string }): Promise<unknown> => {
    if (method === 'qrl_chainId') return chain;
    if (method === 'qrl_getBlockByNumber') return genesis;
    if (method === 'qrl_sendTransaction') return HASH;
    throw new Error('Unexpected wallet request');
  });
  return { request };
}

describe('explorer contract write network binding', () => {
  it('checks chain and genesis before sending a chain-pinned transaction', async () => {
    const provider = providerFixture();
    await expect(sendExplorerTransaction(provider, TX)).resolves.toBe(HASH);
    expect(provider.request.mock.calls.map(([args]) => args.method)).toEqual([
      'qrl_chainId',
      'qrl_getBlockByNumber',
      'qrl_sendTransaction',
    ]);
    expect(provider.request).toHaveBeenLastCalledWith({
      method: 'qrl_sendTransaction',
      params: [{ ...TX, chainId: '0x301825' }],
    });
  });

  it.each(['0x539', '3151909', 'invalid'])(
    'rejects mismatched or malformed chain %s before signing',
    async (chain) => {
      const provider = providerFixture(chain);
      await expect(sendExplorerTransaction(provider, TX)).rejects.toThrow(/network mismatch/);
      expect(provider.request).toHaveBeenCalledTimes(2);
    }
  );

  it.each([null, { number: '0x0', hash: HASH }, { number: '0x1', hash: GENESIS }])(
    'rejects an unqualified genesis',
    async (genesis) => {
      const provider = providerFixture('0x301825', genesis);
      await expect(sendExplorerTransaction(provider, TX)).rejects.toThrow(/genesis/);
      expect(provider.request).toHaveBeenCalledTimes(2);
    }
  );

  it('does not reuse an earlier network qualification', async () => {
    const provider = providerFixture();
    await sendExplorerTransaction(provider, TX);
    provider.request.mockImplementation(async () => '0x539');
    await expect(sendExplorerTransaction(provider, TX)).rejects.toThrow(/network mismatch/);
    expect(
      provider.request.mock.calls.filter(([args]) => args.method === 'qrl_sendTransaction')
    ).toHaveLength(1);
  });

  it('rejects legacy recipients and a conflicting explicit chain before wallet traffic', async () => {
    const provider = providerFixture();
    await expect(
      sendExplorerTransaction(provider, { ...TX, to: `Q${'2'.repeat(40)}` })
    ).rejects.toThrow(/QIP-55/);
    await expect(sendExplorerTransaction(provider, { ...TX, chainId: '0x539' })).rejects.toThrow(
      /chain/
    );
    expect(provider.request).not.toHaveBeenCalled();
  });
});
