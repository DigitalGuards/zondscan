import { canonicalizeQrlAddress } from './qrlAddress';

interface WalletAccountProvider {
  request(args: { method: string; params?: unknown[] }): Promise<unknown>;
  getAccounts(): string[];
  hasStoredSession(): boolean;
}

export interface RestorableWalletSession {
  restorable: boolean;
  account: string | null;
}

export function parseAuthorizedQrlAccount(accounts: unknown): string {
  if (!Array.isArray(accounts) || accounts.length !== 1 || typeof accounts[0] !== 'string') {
    throw new Error('Wallet authorization must return exactly one current-format QRL address');
  }
  const account = canonicalizeQrlAddress(accounts[0]);
  if (!account) {
    throw new Error('Wallet authorization must return exactly one current-format QRL address');
  }
  return account;
}

export async function requestAuthorizedQrlAccount(
  provider: Pick<WalletAccountProvider, 'request'>,
): Promise<string> {
  const accounts = await provider.request({ method: 'qrl_requestAccounts' });
  return parseAuthorizedQrlAccount(accounts);
}

export function getRestorableWalletSession(
  provider: Pick<WalletAccountProvider, 'getAccounts' | 'hasStoredSession'>,
): RestorableWalletSession {
  try {
    if (!provider.hasStoredSession()) return { restorable: false, account: null };
  } catch {
    return { restorable: false, account: null };
  }

  try {
    return { restorable: true, account: parseAuthorizedQrlAccount(provider.getAccounts()) };
  } catch {
    return { restorable: true, account: null };
  }
}
