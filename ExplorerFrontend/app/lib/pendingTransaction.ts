import axios from 'axios';
import type { ContractMeta, PendingTransaction } from '@/app/types';
import config from '../../config';

export type PendingStatus = 'pending' | 'mined' | 'dropped' | 'unavailable';

export interface PendingStatusResult {
  status: PendingStatus;
  transaction?: PendingTransaction;
  targetContract?: ContractMeta;
}

/** Only explicit endpoint absence establishes dropped; upstream failures retry. */
export async function fetchPendingTransactionStatus(hash: string): Promise<PendingStatusResult> {
  try {
    const response = await axios.get(`${config.handlerUrl}/pending-transaction/${hash}`);
    const transaction = response.data?.transaction;
    if (transaction?.status === 'mined') return { status: 'mined' };
    if (transaction?.status === 'pending') {
      return { status: 'pending', transaction, targetContract: response.data.targetContract };
    }
    return { status: 'unavailable' };
  } catch (error) {
    if (!axios.isAxiosError(error) || error.response?.status !== 404) {
      return { status: 'unavailable' };
    }
    if (error.response.data?.status === 'mined') return { status: 'mined' };
  }

  try {
    const response = await axios.get(`${config.handlerUrl}/tx/${hash}`);
    return response.data?.response?.TxHash?.toLowerCase() === hash.toLowerCase()
      ? { status: 'mined' }
      : { status: 'unavailable' };
  } catch (error) {
    return axios.isAxiosError(error) && error.response?.status === 404
      ? { status: 'dropped' }
      : { status: 'unavailable' };
  }
}

export function pendingStatusPollInterval(status?: PendingStatus): number | false {
  return status === 'mined' || status === 'dropped' ? false : 5000;
}
