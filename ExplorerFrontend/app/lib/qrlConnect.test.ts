import { describe, expect, it, jest } from '@jest/globals';
import {
  getRestorableWalletSession,
  parseAuthorizedQrlAccount,
  requestAuthorizedQrlAccount,
} from './qrlAccounts';

const ACCOUNT = `Q${'a'.repeat(40)}`;

describe('QRL Connect account authorization', () => {
  it('requests account access through qrl_requestAccounts', async () => {
    const request = jest.fn(async (_args: { method: string; params?: unknown[] }) => [ACCOUNT]);

    await expect(requestAuthorizedQrlAccount({ request })).resolves.toBe(ACCOUNT);
    expect(request).toHaveBeenCalledTimes(1);
    expect(request).toHaveBeenCalledWith({ method: 'qrl_requestAccounts' });
  });

  it.each([
    ['lowercase prefix', [`q${'a'.repeat(40)}`]],
    ['0x prefix', [`0x${'a'.repeat(40)}`]],
    ['short address', [`Q${'a'.repeat(39)}`]],
    ['long address', [`Q${'a'.repeat(41)}`]],
    ['non-hex address', [`Q${'z'.repeat(40)}`]],
    ['multiple accounts', [ACCOUNT, `Q${'b'.repeat(40)}`]],
    ['empty account list', []],
    ['non-array response', ACCOUNT],
  ])('rejects %s', (_label, accounts) => {
    expect(() => parseAuthorizedQrlAccount(accounts)).toThrow(
      'exactly one current-format QRL address',
    );
  });

  it('preserves an authorized account while its stored session is restorable', () => {
    expect(
      getRestorableWalletSession({
        hasStoredSession: () => true,
        getAccounts: () => [ACCOUNT],
      }),
    ).toEqual({ restorable: true, account: ACCOUNT });
  });

  it('fails closed on a malformed account without discarding the restorable session', () => {
    expect(
      getRestorableWalletSession({
        hasStoredSession: () => true,
        getAccounts: () => [`q${'a'.repeat(40)}`],
      }),
    ).toEqual({ restorable: true, account: null });
  });

  it('marks a session non-restorable after explicit storage removal', () => {
    expect(
      getRestorableWalletSession({
        hasStoredSession: () => false,
        getAccounts: () => [ACCOUNT],
      }),
    ).toEqual({ restorable: false, account: null });
  });
});
