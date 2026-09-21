import type { QrlSignedMessageResult } from '@qrlwallet/connect';
import { shake256 } from '@noble/hashes/sha3.js';
import {
  bytesToHex,
  hexToBytes,
  utf8ToBytes,
} from '@noble/hashes/utils.js';

import { canonicalizeQrlAddress } from './qrlAddress';

const CHALLENGE_ID_PATTERN = /^[0-9a-f]{64}$/;
const MESSAGE_HEX_PATTERN = /^0x(?:[0-9a-f]{2})+$/;
const UTC_SECONDS_PATTERN =
  /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/;
const MAX_MESSAGE_BYTES = 16 * 1024;
const ML_DSA_87_DESCRIPTOR = '0x010000';
const QRL_SIGN_MESSAGE_V1 = utf8ToBytes('QRL-SIGN-MSG-v1');
const AUTHORITATIVE_CREATOR_PROVENANCE = new Set([
  'direct-deployment',
  'create-trace-outer-sender',
]);

export interface ContractExplainChallenge {
  challengeId: string;
  messageHex: string;
  signer: string;
  contract: string;
  origin: string;
  chainId: string;
  expiresAt: string;
}

export interface ContractExplainProof {
  challengeId: string;
  proof: QrlSignedMessageResult & { descriptor: string };
}

export interface ContractExplainWalletProvider {
  request(args: { method: string; params?: unknown[] }): Promise<unknown>;
  getAccounts(): string[];
}

export function hasAuthoritativeCreatorProvenance(value: unknown): boolean {
  return typeof value === 'string' && AUTHORITATIVE_CREATOR_PROVENANCE.has(value);
}

interface ChallengeExpectation {
  contract: string;
  signer: string;
  origin: string;
  now?: number;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function hasExactKeys(
  value: Record<string, unknown>,
  keys: readonly string[],
): boolean {
  const actual = Reflect.ownKeys(value);
  return (
    actual.length === keys.length &&
    actual.every(
      (key) => typeof key === 'string' && keys.includes(key),
    )
  );
}

function requireCanonicalAddress(value: unknown, field: string): string {
  if (typeof value !== 'string') {
    throw new Error(`Invalid contract explanation ${field}`);
  }
  const canonical = canonicalizeQrlAddress(value);
  if (!canonical || canonical !== value) {
    throw new Error(`Invalid contract explanation ${field}`);
  }
  return canonical;
}

export function normalizeQrlChainId(value: unknown): string {
  if (typeof value !== 'string' || !/^0x[0-9a-fA-F]+$/.test(value)) {
    throw new Error('Wallet returned an invalid QRL chain ID');
  }
  const significantDigits = value.slice(2).replace(/^0+/, '');
  if (significantDigits.length === 0 || significantDigits.length > 64) {
    throw new Error('Wallet returned an invalid QRL chain ID');
  }
  const numeric = BigInt(value);
  return `0x${numeric.toString(16)}`;
}

export function qrlSignedMessageDigest(messageHex: string): string {
  if (!MESSAGE_HEX_PATTERN.test(messageHex)) {
    throw new Error('Invalid contract explanation challenge message');
  }
  const digest = shake256
    .create({ dkLen: 64 })
    .update(QRL_SIGN_MESSAGE_V1)
    .update(hexToBytes(messageHex.slice(2)))
    .digest();
  return `0x${bytesToHex(digest)}`;
}

function normalizeOrigin(value: unknown): string {
  if (typeof value !== 'string') {
    throw new Error('Invalid contract explanation origin');
  }
  let parsed: URL;
  try {
    parsed = new URL(value);
  } catch {
    throw new Error('Invalid contract explanation origin');
  }
  if (
    parsed.origin !== value ||
    parsed.username ||
    parsed.password ||
    (parsed.protocol !== 'https:' &&
      !(
        parsed.protocol === 'http:' &&
        (parsed.hostname === '127.0.0.1' ||
          parsed.hostname === 'localhost' ||
          parsed.hostname === '[::1]')
      ))
  ) {
    throw new Error('Invalid contract explanation origin');
  }
  return parsed.origin;
}

export function parseContractExplainChallenge(
  value: unknown,
  expectation: ChallengeExpectation,
): ContractExplainChallenge {
  const keys = [
    'challengeId',
    'messageHex',
    'signer',
    'contract',
    'origin',
    'chainId',
    'expiresAt',
  ] as const;
  if (!isRecord(value) || !hasExactKeys(value, keys)) {
    throw new Error('Invalid contract explanation challenge');
  }
  if (
    typeof value.challengeId !== 'string' ||
    !CHALLENGE_ID_PATTERN.test(value.challengeId)
  ) {
    throw new Error('Invalid contract explanation challenge ID');
  }
  if (
    typeof value.messageHex !== 'string' ||
    !MESSAGE_HEX_PATTERN.test(value.messageHex) ||
    (value.messageHex.length - 2) / 2 > MAX_MESSAGE_BYTES
  ) {
    throw new Error('Invalid contract explanation challenge message');
  }

  const contract = requireCanonicalAddress(value.contract, 'contract');
  const signer = requireCanonicalAddress(value.signer, 'signer');
  const expectedContract = requireCanonicalAddress(expectation.contract, 'contract');
  const expectedSigner = requireCanonicalAddress(expectation.signer, 'signer');
  const origin = normalizeOrigin(value.origin);
  const expectedOrigin = normalizeOrigin(expectation.origin);
  if (
    contract !== expectedContract ||
    signer !== expectedSigner ||
    origin !== expectedOrigin
  ) {
    throw new Error('Contract explanation challenge binding mismatch');
  }

  const chainId = normalizeQrlChainId(value.chainId);
  if (chainId !== value.chainId) {
    throw new Error('Invalid contract explanation chain ID');
  }
  if (
    typeof value.expiresAt !== 'string' ||
    !UTC_SECONDS_PATTERN.test(value.expiresAt)
  ) {
    throw new Error('Invalid contract explanation challenge expiry');
  }
  const expiresAt = new Date(value.expiresAt);
  if (
    Number.isNaN(expiresAt.getTime()) ||
    expiresAt.toISOString().replace('.000Z', 'Z') !== value.expiresAt ||
    expiresAt.getTime() <= (expectation.now ?? Date.now())
  ) {
    throw new Error('Contract explanation challenge expired');
  }

  return {
    challengeId: value.challengeId,
    messageHex: value.messageHex,
    signer,
    contract,
    origin,
    chainId,
    expiresAt: value.expiresAt,
  };
}

function currentCanonicalAccount(
  provider: ContractExplainWalletProvider,
): string {
  const accounts = provider.getAccounts();
  if (accounts.length !== 1) {
    throw new Error('Connect the contract creator wallet to regenerate');
  }
  const account = canonicalizeQrlAddress(accounts[0]);
  if (!account) {
    throw new Error('Wallet returned an invalid current QRL account');
  }
  return account;
}

async function assertWalletBinding(
  provider: ContractExplainWalletProvider,
  challenge: ContractExplainChallenge,
): Promise<void> {
  if (currentCanonicalAccount(provider) !== challenge.signer) {
    throw new Error('The connected wallet is not the contract creator');
  }
  const chainId = normalizeQrlChainId(
    await provider.request({ method: 'qrl_chainId' }),
  );
  if (chainId !== challenge.chainId) {
    throw new Error('Switch the wallet to the challenge network');
  }
}

export async function signContractExplainChallenge(
  provider: ContractExplainWalletProvider,
  challenge: ContractExplainChallenge,
): Promise<ContractExplainProof> {
  if (new Date(challenge.expiresAt).getTime() <= Date.now()) {
    throw new Error('Contract explanation challenge expired');
  }
  await assertWalletBinding(provider, challenge);

  const raw = await provider.request({
    method: 'qrl_signMessage',
    params: [challenge.signer, challenge.messageHex],
  });
  const {
    hasSigningDescriptor,
    hexToBytes: connectHexToBytes,
    isQrlSignedMessageResult,
    verifyMessageForSigner,
  } = await import('@qrlwallet/connect');
  if (
    !isQrlSignedMessageResult(raw) ||
    !hasSigningDescriptor(raw) ||
    raw.descriptor.toLowerCase() !== ML_DSA_87_DESCRIPTOR ||
    canonicalizeQrlAddress(raw.signer) !== challenge.signer ||
    raw.signer !== challenge.signer
  ) {
    throw new Error('Wallet returned an invalid signed challenge');
  }

  const messageBytes = connectHexToBytes(challenge.messageHex);
  const expectedDigest = qrlSignedMessageDigest(challenge.messageHex);
  if (raw.digest.toLowerCase() !== expectedDigest) {
    throw new Error('Wallet returned a mismatched challenge digest');
  }
  if (
    !verifyMessageForSigner({
      expectedSigner: challenge.signer,
      descriptor: raw.descriptor,
      signature: raw.signature,
      publicKey: raw.publicKey,
      messageBytes,
    })
  ) {
    throw new Error('Wallet challenge signature verification failed');
  }

  await assertWalletBinding(provider, challenge);
  if (new Date(challenge.expiresAt).getTime() <= Date.now()) {
    throw new Error('Contract explanation challenge expired');
  }

  return {
    challengeId: challenge.challengeId,
    proof: {
      signature: raw.signature,
      publicKey: raw.publicKey,
      descriptor: raw.descriptor,
      signer: raw.signer,
      digest: raw.digest,
      schemeVersion: raw.schemeVersion,
    },
  };
}
