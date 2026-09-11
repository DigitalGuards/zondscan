import { describe, expect, it, jest } from '@jest/globals';

jest.mock('@qrlwallet/connect', () => ({
  hasSigningDescriptor: jest.fn(() => true),
  hexToBytes: jest.fn((value: string) =>
    Uint8Array.from(Buffer.from(value.slice(2), 'hex')),
  ),
  isQrlSignedMessageResult: jest.fn(() => true),
  verifyMessageForSigner: jest.fn(() => true),
}));

const {
  hasSigningDescriptor,
  isQrlSignedMessageResult,
  verifyMessageForSigner,
} = jest.requireMock<typeof import('@qrlwallet/connect')>('@qrlwallet/connect');

import { canonicalizeQrlAddress } from './qrlAddress';
import {
  hasAuthoritativeCreatorProvenance,
  normalizeQrlChainId,
  parseContractExplainChallenge,
  qrlSignedMessageDigest,
  signContractExplainChallenge,
} from './contractExplainAuth';

const CONTRACT = canonicalizeQrlAddress(`Q${'1'.repeat(128)}`)!;
const SIGNER = canonicalizeQrlAddress(`Q${'2'.repeat(128)}`)!;
const OTHER_SIGNER = canonicalizeQrlAddress(`Q${'3'.repeat(128)}`)!;
const MESSAGE_HEX = '0x5a4f4e445343414e2d415554482d7631';
const EXPIRES_AT = '2099-08-27T12:05:00Z';

function challenge(overrides: Record<string, unknown> = {}): unknown {
  return {
    challengeId: 'a'.repeat(64),
    messageHex: MESSAGE_HEX,
    signer: SIGNER,
    contract: CONTRACT,
    origin: 'https://zondscan.com',
    chainId: '0x539',
    expiresAt: EXPIRES_AT,
    ...overrides,
  };
}

function parse(value: unknown = challenge()) {
  return parseContractExplainChallenge(value, {
    contract: CONTRACT,
    signer: SIGNER,
    origin: 'https://zondscan.com',
    now: Date.parse('2099-08-27T12:00:00Z'),
  });
}

describe('contract explanation challenge parsing', () => {
  it('accepts only deployment-evidence-backed creator provenance', () => {
    expect(hasAuthoritativeCreatorProvenance('direct-deployment')).toBe(true);
    expect(hasAuthoritativeCreatorProvenance('create-trace-outer-sender')).toBe(true);
    expect(hasAuthoritativeCreatorProvenance('mint-heuristic')).toBe(false);
    expect(hasAuthoritativeCreatorProvenance(undefined)).toBe(false);
  });

  it('accepts an exact canonical challenge bound to the current page', () => {
    expect(parse()).toEqual(challenge());
  });

  it.each([
    ['legacy-width signer', { signer: `Q${'a'.repeat(40)}` }],
    ['cross-contract replay', { contract: OTHER_SIGNER }],
    ['cross-origin replay', { origin: 'https://example.com' }],
    ['noncanonical chain', { chainId: '0x0539' }],
    ['expired challenge', { expiresAt: '2099-08-27T11:59:59Z' }],
    ['explicit null signer', { signer: null }],
  ])('rejects %s', (_label, overrides) => {
    expect(() => parse(challenge(overrides))).toThrow();
  });

  it('rejects response fields that are not part of the signed protocol', () => {
    expect(() => parse(challenge({ extra: true }))).toThrow(
      'Invalid contract explanation challenge',
    );
  });

  it('normalizes valid chain IDs and rejects malformed values', () => {
    expect(() => normalizeQrlChainId('0X539')).toThrow(
      'Wallet returned an invalid QRL chain ID',
    );
    expect(normalizeQrlChainId('0x000A')).toBe('0xa');
    expect(() => normalizeQrlChainId('0x0')).toThrow(
      'Wallet returned an invalid QRL chain ID',
    );
    expect(() => normalizeQrlChainId('539')).toThrow(
      'Wallet returned an invalid QRL chain ID',
    );
  });
});

describe('contract explanation challenge signing', () => {
  function signedResult() {
    return {
      signature: '0xsignature',
      publicKey: '0xpublickey',
      descriptor: '0x010000',
      signer: SIGNER,
      digest: qrlSignedMessageDigest(MESSAGE_HEX),
      schemeVersion: 'QRL-SIGN-MSG-v1' as const,
    };
  }

  it('checks account and chain on both sides of local signature verification', async () => {
    const request = jest.fn(async ({ method }: { method: string }) =>
      method === 'qrl_chainId' ? '0x539' : signedResult(),
    );
    const provider = { request, getAccounts: () => [SIGNER] };

    await expect(signContractExplainChallenge(provider, parse())).resolves.toEqual({
      challengeId: 'a'.repeat(64),
      proof: signedResult(),
    });
    expect(request.mock.calls.map(([call]) => call.method)).toEqual([
      'qrl_chainId',
      'qrl_signMessage',
      'qrl_chainId',
    ]);
    expect(request.mock.calls[1]?.[0]).toEqual({
      method: 'qrl_signMessage',
      params: [SIGNER, MESSAGE_HEX],
    });
    expect(isQrlSignedMessageResult).toHaveBeenCalled();
    expect(hasSigningDescriptor).toHaveBeenCalled();
    expect(verifyMessageForSigner).toHaveBeenCalledWith(
      expect.objectContaining({ expectedSigner: SIGNER }),
    );
  });

  it('aborts if the connected account changes after signing', async () => {
    let accountRead = 0;
    const provider = {
      request: jest.fn(async ({ method }: { method: string }) =>
        method === 'qrl_chainId' ? '0x539' : signedResult(),
      ),
      getAccounts: () => {
        accountRead += 1;
        return [accountRead === 1 ? SIGNER : OTHER_SIGNER];
      },
    };

    await expect(signContractExplainChallenge(provider, parse())).rejects.toThrow(
      'The connected wallet is not the contract creator',
    );
  });

  it('fails closed when local bound verification rejects the proof', async () => {
    jest.mocked(verifyMessageForSigner).mockReturnValueOnce(false);
    const provider = {
      request: jest.fn(async ({ method }: { method: string }) =>
        method === 'qrl_chainId' ? '0x539' : signedResult(),
      ),
      getAccounts: () => [SIGNER],
    };

    await expect(signContractExplainChallenge(provider, parse())).rejects.toThrow(
      'Wallet challenge signature verification failed',
    );
  });
});
