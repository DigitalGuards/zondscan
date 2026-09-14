import axios from 'axios';
import { fetchPendingTransactionStatus, pendingStatusPollInterval } from './pendingTransaction';

jest.mock('axios');
const get = jest.mocked(axios.get);
const hash = `0x${'a'.repeat(64)}`;
const httpError = (status: number, data = {}) => ({ response: { status, data } });

beforeEach(() => {
  jest.resetAllMocks();
  jest
    .mocked(axios.isAxiosError)
    .mockImplementation((error) => Boolean((error as { response?: unknown })?.response));
});

it('honors an explicit mined tombstone without a second upstream request', async () => {
  get.mockRejectedValueOnce(httpError(404, { status: 'mined' }));
  expect(await fetchPendingTransactionStatus(hash)).toEqual({ status: 'mined' });
  expect(get).toHaveBeenCalledTimes(1);
});

it.each([500, 503, 504, 429])('keeps a failed detail lookup retryable (%s)', async (status) => {
  get.mockRejectedValueOnce(httpError(404)).mockRejectedValueOnce(httpError(status));
  const result = await fetchPendingTransactionStatus(hash);
  expect(result.status).toBe('unavailable');
  expect(pendingStatusPollInterval(result.status)).toBe(5000);
  get
    .mockRejectedValueOnce(httpError(404))
    .mockResolvedValueOnce({ data: { response: { TxHash: hash } } });
  expect(await fetchPendingTransactionStatus(hash)).toEqual({ status: 'mined' });
});

it('requires both endpoints to report absence before displaying dropped', async () => {
  get.mockRejectedValueOnce(httpError(404)).mockRejectedValueOnce(httpError(404));
  const result = await fetchPendingTransactionStatus(hash);
  expect(result.status).toBe('dropped');
  expect(pendingStatusPollInterval(result.status)).toBe(false);
});

it('keeps network failures and malformed responses unavailable', async () => {
  get.mockRejectedValueOnce(new Error('offline'));
  expect(await fetchPendingTransactionStatus(hash)).toEqual({ status: 'unavailable' });
  get.mockResolvedValueOnce({ data: {} });
  expect(await fetchPendingTransactionStatus(hash)).toEqual({ status: 'unavailable' });
});

it('preserves pending data and recipient metadata for server and client callers', async () => {
  const transaction = { hash, status: 'pending' };
  const targetContract = { name: 'Example' };
  get.mockResolvedValueOnce({ data: { transaction, targetContract } });
  expect(await fetchPendingTransactionStatus(hash)).toEqual({
    status: 'pending',
    transaction,
    targetContract,
  });
  expect(pendingStatusPollInterval('pending')).toBe(5000);
});
