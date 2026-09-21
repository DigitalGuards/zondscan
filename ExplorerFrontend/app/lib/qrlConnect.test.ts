import { describe, expect, it, jest } from '@jest/globals';
import {
  getRestorableWalletSession,
  parseAuthorizedQrlAccount,
  requestAuthorizedQrlAccount,
} from './qrlAccounts';

const ACCOUNT_BODY = 'a'.repeat(128);
const ACCOUNT =
  'QaaaAAaaaAAaAaaAaAAAAaAAAaAaAaAAaAaaAaaaaAAAAAAAAaAAAAaAaAAaaAAaaaaAaAAAAaaAaAAaaaaaaAaAAaaaaAaAaaaaAaaaAAAAaAAAAaAaAaaAAAaAaaaAA';

describe('QRL Connect account authorization', () => {
  it('requests account access through qrl_requestAccounts', async () => {
    const request = jest.fn(async (_args: { method: string; params?: unknown[] }) => [
      `Q${ACCOUNT_BODY}`,
    ]);

    await expect(requestAuthorizedQrlAccount({ request })).resolves.toBe(ACCOUNT);
    expect(request).toHaveBeenCalledTimes(1);
    expect(request).toHaveBeenCalledWith({ method: 'qrl_requestAccounts' });
  });

  it.each([
    `Q${ACCOUNT_BODY}`,
    `q${ACCOUNT_BODY.toUpperCase()}`,
    `0x${ACCOUNT_BODY}`,
    `0X${ACCOUNT_BODY.toUpperCase()}`,
    ACCOUNT_BODY,
  ])('canonicalizes an authorized account alias %s', (account) => {
    expect(parseAuthorizedQrlAccount([account])).toBe(ACCOUNT);
  });

  it.each([
    ['127-hex address', [`Q${'a'.repeat(127)}`]],
    ['129-hex address', [`Q${'a'.repeat(129)}`]],
    ['legacy 20-byte address', [`Q${'a'.repeat(40)}`]],
    ['non-hex address', [`Q${'a'.repeat(127)}z`]],
    ['invalid mixed-case checksum', [`Q${'Ab'.repeat(64)}`]],
    ['multiple accounts', [ACCOUNT, `Q${'b'.repeat(128)}`]],
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
        getAccounts: () => [`Q${'Ab'.repeat(64)}`],
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
