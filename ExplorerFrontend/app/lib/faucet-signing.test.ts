jest.mock('server-only', () => ({}));
jest.mock('@theqrl/web3', () => ({ Web3: jest.fn(() => mockWeb3) }));

import { FaucetError, sendFaucetDrip } from './faucet';

const from = `Q${'1'.repeat(128)}`;
const to = `Q${'2'.repeat(128)}`;
const seed = '0x010000';
const hash = `0x${'ab'.repeat(32)}`;
const raw = '0x02010203';
const mockWeb3 = {
  utils: {
    toPlanck: jest.fn(() => '10000000000000000000'),
    toHex: (value: bigint) => `0x${value.toString(16)}`,
  },
  qrl: {
    getGasPrice: jest.fn<Promise<unknown>, []>(),
    getBalance: jest.fn<Promise<unknown>, [string]>(),
    getTransactionCount: jest.fn<Promise<unknown>, [string, string]>(),
    accounts: {
      seedToAccount: jest.fn(() => ({ address: from })),
      signTransaction: jest.fn<Promise<unknown>, [unknown, string]>(),
    },
    sendSignedTransaction: jest.fn(() => ({
      on: (event: string, listener: () => void) => {
        if (event === 'transactionHash') listener();
      },
    })),
  },
};

describe('faucet transaction boundary', () => {
  const original = { ...process.env };
  beforeEach(() => {
    jest.clearAllMocks();
    process.env.FAUCET_SEED = seed;
    process.env.FAUCET_DRIP_QUANTA = '10';
    process.env.FAUCET_DAILY_CAP_QUANTA = '0';
    delete process.env.DATABASE_URL;
    mockWeb3.qrl.getGasPrice.mockResolvedValue(BigInt(1_000_000_000));
    mockWeb3.qrl.getBalance.mockResolvedValue(BigInt('100000000000000000000'));
    mockWeb3.qrl.getTransactionCount.mockResolvedValue(BigInt(7));
    mockWeb3.qrl.accounts.signTransaction.mockResolvedValue({
      rawTransaction: raw,
      transactionHash: hash,
    });
  });
  afterEach(() => {
    process.env = { ...original };
  });

  it('preserves every signing field and broadcasts the exact signed bytes', async () => {
    await expect(sendFaucetDrip(to)).resolves.toEqual({ txHash: hash, amount: '10', from, to });
    expect(mockWeb3.qrl.getTransactionCount).toHaveBeenCalledWith(from, 'pending');
    expect(mockWeb3.qrl.accounts.signTransaction).toHaveBeenCalledWith(
      {
        from,
        to,
        value: '10000000000000000000',
        gas: 21000,
        type: '0x2',
        maxFeePerGas: '0x77359400',
        maxPriorityFeePerGas: '0x3b9aca00',
        nonce: 7,
      },
      seed
    );
    expect(mockWeb3.qrl.sendSignedTransaction).toHaveBeenCalledWith(raw);
  });

  it.each([null, false, {}, [], 'invalid', -1, 1.5])(
    'rejects malformed gas price %p before signing',
    async (value) => {
      mockWeb3.qrl.getGasPrice.mockResolvedValue(value);
      await expect(sendFaucetDrip(to)).rejects.toBeInstanceOf(FaucetError);
      expect(mockWeb3.qrl.accounts.signTransaction).not.toHaveBeenCalled();
      expect(mockWeb3.qrl.sendSignedTransaction).not.toHaveBeenCalled();
    }
  );

  it.each([null, {}, 'invalid', BigInt(Number.MAX_SAFE_INTEGER) + BigInt(1)])(
    'rejects malformed or unsafe nonce %p before signing',
    async (value) => {
      mockWeb3.qrl.getTransactionCount.mockResolvedValue(value);
      await expect(sendFaucetDrip(to)).rejects.toBeInstanceOf(FaucetError);
      expect(mockWeb3.qrl.accounts.signTransaction).not.toHaveBeenCalled();
    }
  );

  it.each([
    null,
    {},
    { rawTransaction: '0x0', transactionHash: hash },
    { rawTransaction: raw, transactionHash: {} },
  ])('rejects malformed signer result %p before broadcast', async (value) => {
    mockWeb3.qrl.accounts.signTransaction.mockResolvedValue(value);
    await expect(sendFaucetDrip(to)).rejects.toBeInstanceOf(FaucetError);
    expect(mockWeb3.qrl.sendSignedTransaction).not.toHaveBeenCalled();
  });

  it('keeps the existing gas-price fallback for a transport failure', async () => {
    mockWeb3.qrl.getGasPrice.mockRejectedValue(new Error('offline'));
    await sendFaucetDrip(to);
    expect(mockWeb3.qrl.accounts.signTransaction).toHaveBeenCalledWith(
      expect.objectContaining({
        maxFeePerGas: '0x77359400',
        maxPriorityFeePerGas: '0x3b9aca00',
      }),
      seed
    );
  });
});
