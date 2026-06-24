import 'server-only';

import { Web3 } from '@theqrl/web3';
import { MLDSA87 } from '@theqrl/wallet.js';

import type { Collection, ObjectId } from 'mongodb';

import { getFaucetDb } from './mongodb';

/**
 * Server-only faucet core: configuration, QRL address handling, account
 * derivation, transaction signing/broadcast, Cloudflare Turnstile verification,
 * and the MongoDB-backed claim cooldown.
 *
 * Everything here runs exclusively in the Node runtime of the `/faucet/claim`
 * route handler. The funding seed and Turnstile secret are read from
 * server-only env vars and must NEVER be exposed to the client.
 *
 * Signing mirrors the proven flow in DigitalGuards/myqrlwallet-frontend
 * (qrlStore.signAndSendTransaction): a type-0x2 (EIP-1559) native transfer,
 * nonce from the pending pool, gas derived from the node's current gas price,
 * signed with `web3.qrl.accounts.signTransaction(tx, seed)` and broadcast via
 * `web3.qrl.sendSignedTransaction`.
 */

// ── Configuration ──────────────────────────────────────────────────────────

const DEFAULT_RPC_URL = 'https://qrlwallet.com/api/qrl-rpc/testnet';
const CLAIMS_COLLECTION = 'faucetClaims';

export interface FaucetConfig {
  /** Whole-quanta amount sent per successful claim. */
  dripQuanta: string;
  /** Cooldown window in hours, enforced per address and per IP. */
  cooldownHours: number;
  /** RPC endpoint the faucet signs/broadcasts against. */
  rpcUrl: string;
  /** True when a funding seed is configured (faucet can actually send). */
  configured: boolean;
  /** True when Turnstile is fully wired (site + secret keys present). */
  captchaEnabled: boolean;
}

export function getFaucetConfig(): FaucetConfig {
  return {
    dripQuanta: process.env.FAUCET_DRIP_QUANTA || '10',
    cooldownHours: Number(process.env.FAUCET_COOLDOWN_HOURS || '24'),
    rpcUrl: process.env.FAUCET_RPC_URL || DEFAULT_RPC_URL,
    configured: Boolean(process.env.FAUCET_SEED),
    captchaEnabled: Boolean(
      process.env.TURNSTILE_SECRET && process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY,
    ),
  };
}

/** Typed faucet failure so the route can map causes to HTTP status codes. */
export class FaucetError extends Error {
  constructor(
    public code:
      | 'NOT_CONFIGURED'
      | 'INVALID_ADDRESS'
      | 'CAPTCHA_FAILED'
      | 'COOLDOWN'
      | 'INSUFFICIENT_FAUCET_FUNDS'
      | 'BROADCAST_FAILED',
    message: string,
    public meta?: Record<string, unknown>,
  ) {
    super(message);
    this.name = 'FaucetError';
  }
}

// ── QRL address handling ─────────────────────────────────────────────────────

/**
 * Validate and normalise a user-supplied QRL address to the canonical
 * `Q`+lowercase-hex form the @theqrl/web3 lib accepts for `to`. Accepts the
 * common input shapes (`Q…`, `Z…`, `0x…`, bare hex). Returns null when the
 * value isn't a 40-hex-character address.
 */
export function normalizeQrlAddress(input: string): string | null {
  if (!input) return null;
  const trimmed = input.trim();
  let core = trimmed;
  if (/^0x/i.test(core)) core = core.slice(2);
  else if (/^[QZqz]/.test(core)) core = core.slice(1);
  if (!/^[0-9a-fA-F]{40}$/.test(core)) return null;
  return 'Q' + core.toLowerCase();
}

// ── web3 instance + faucet account (memoised) ────────────────────────────────

let cachedWeb3: Web3 | null = null;
let cachedRpcUrl: string | null = null;

function getWeb3(): Web3 {
  const { rpcUrl } = getFaucetConfig();
  if (!cachedWeb3 || cachedRpcUrl !== rpcUrl) {
    cachedWeb3 = new Web3(rpcUrl);
    cachedRpcUrl = rpcUrl;
  }
  return cachedWeb3;
}

/**
 * Resolve the funding seed (hex extended seed) from `FAUCET_SEED`. The env var
 * may hold either a hex extended seed (`0x0100…`) or a BIP39-style mnemonic
 * phrase (space-separated words), which is expanded via MLDSA87 the same way
 * the wallet's crypto worker does.
 */
function resolveSeed(): string {
  const raw = process.env.FAUCET_SEED;
  if (!raw || !raw.trim()) {
    throw new FaucetError('NOT_CONFIGURED', 'Faucet is not configured (no funding seed)');
  }
  const value = raw.trim();
  if (value.includes(' ')) {
    // Mnemonic → hex extended seed.
    return MLDSA87.newWalletFromMnemonic(value).getHexExtendedSeed();
  }
  return value.startsWith('0x') ? value : '0x' + value;
}

let cachedSeed: string | null = null;
let cachedFaucetAddress: string | null = null;

function getSeed(): string {
  if (!cachedSeed) cachedSeed = resolveSeed();
  return cachedSeed;
}

/** Canonical `Q…` address of the configured faucet account. */
export function getFaucetAddress(): string {
  if (!cachedFaucetAddress) {
    const account = getWeb3().qrl.accounts.seedToAccount(getSeed());
    cachedFaucetAddress = account.address;
  }
  return cachedFaucetAddress;
}

// ── Signing + broadcast ──────────────────────────────────────────────────────

export interface DripResult {
  txHash: string;
  amount: string;
  from: string;
  to: string;
}

/**
 * Build, sign, and broadcast a native faucet transfer to `to` (already
 * normalised). Pre-checks the faucet balance so an empty/underfunded faucet
 * surfaces as a clean error rather than a node revert. Resolves once the node
 * has accepted the tx (transactionHash known), without waiting for a full
 * receipt, so the caller responds within a block time.
 */
export async function sendFaucetDrip(to: string): Promise<DripResult> {
  const web3 = getWeb3();
  const seed = getSeed();
  const from = getFaucetAddress();
  const { dripQuanta } = getFaucetConfig();

  const value = BigInt(web3.utils.toPlanck(dripQuanta, 'quanta'));

  // Gas: 21000 for a native transfer; fee derived from the node's gas price.
  const gas = BigInt(21000);
  const oneGquanta = BigInt('1000000000'); // 1 gquanta fallback
  let baseGasPrice: bigint;
  try {
    baseGasPrice = BigInt(await web3.qrl.getGasPrice());
  } catch {
    baseGasPrice = oneGquanta;
  }
  if (baseGasPrice <= BigInt(0)) baseGasPrice = oneGquanta;
  const maxPriorityFeePerGas = baseGasPrice;
  const maxFeePerGas = baseGasPrice * BigInt(2);

  // Guard against an empty faucet before we bother signing.
  let balance: bigint;
  try {
    balance = BigInt(await web3.qrl.getBalance(from));
  } catch (err) {
    throw new FaucetError('BROADCAST_FAILED', 'Unable to reach the QRL node', {
      cause: (err as Error).message,
    });
  }
  const maxCost = value + gas * maxFeePerGas;
  if (balance < maxCost) {
    throw new FaucetError(
      'INSUFFICIENT_FAUCET_FUNDS',
      'The faucet is temporarily out of funds. Please try again later.',
      { faucetAddress: from },
    );
  }

  const nonce = await web3.qrl.getTransactionCount(from, 'pending');

  const tx = {
    from,
    to,
    value: value.toString(),
    gas: Number(gas),
    type: '0x2',
    maxFeePerGas: web3.utils.toHex(maxFeePerGas),
    maxPriorityFeePerGas: web3.utils.toHex(maxPriorityFeePerGas),
    nonce: Number(nonce),
  };

  let rawTransaction: string;
  let txHash: string;
  try {
    const signed = await web3.qrl.accounts.signTransaction(tx, seed);
    rawTransaction = signed.rawTransaction;
    txHash = signed.transactionHash;
  } catch (err) {
    throw new FaucetError('BROADCAST_FAILED', 'Failed to sign the faucet transaction', {
      cause: (err as Error).message,
    });
  }

  // Broadcast and resolve as soon as the node returns the tx hash. The
  // PromiEvent rejects synchronously-ish on submission errors (bad nonce,
  // insufficient funds the balance check missed, etc.).
  await new Promise<void>((resolve, reject) => {
    const pe = web3.qrl.sendSignedTransaction(rawTransaction);
    pe.on('transactionHash', () => resolve());
    pe.on('error', (e: Error) => reject(e));
    // If a receipt arrives first (fast chains), that's success too.
    pe.on('receipt', () => resolve());
  }).catch((err: Error) => {
    throw new FaucetError('BROADCAST_FAILED', 'The node rejected the faucet transaction', {
      cause: err.message,
    });
  });

  return { txHash, amount: dripQuanta, from, to };
}

// ── Cloudflare Turnstile ─────────────────────────────────────────────────────

/**
 * Verify a Turnstile token server-side. When Turnstile isn't configured this is
 * a no-op (returns true) so the faucet still works in dev without captcha keys;
 * production deploys set both keys to enforce it.
 */
export async function verifyTurnstile(token: string | undefined, ip: string | null): Promise<boolean> {
  const secret = process.env.TURNSTILE_SECRET;
  if (!secret) return true; // captcha not enforced
  if (!token) return false;

  const body = new URLSearchParams();
  body.append('secret', secret);
  body.append('response', token);
  if (ip) body.append('remoteip', ip);

  try {
    const res = await fetch('https://challenges.cloudflare.com/turnstile/v0/siteverify', {
      method: 'POST',
      headers: { 'Content-Type': 'application/x-www-form-urlencoded' },
      body,
    });
    const data = (await res.json()) as { success?: boolean };
    return data.success === true;
  } catch {
    return false;
  }
}

// ── Cooldown (MongoDB) ───────────────────────────────────────────────────────

interface FaucetClaim {
  address: string;
  ip: string | null;
  txHash: string;
  amount: string;
  createdAt: Date;
}

/** Marker txHash for a reserved-but-not-yet-broadcast claim. */
const PENDING_TX = 'PENDING';

/**
 * createIndex is idempotent, but each call is still a round-trip to Mongo.
 * Ensure the cooldown indexes exist once per process rather than on every
 * claim. Best-effort: a failure here just means the lookups aren't indexed.
 */
let claimIndexesEnsured = false;
async function ensureClaimIndexes(col: Collection<FaucetClaim>): Promise<void> {
  if (claimIndexesEnsured) return;
  await Promise.all([
    col.createIndex({ address: 1, createdAt: -1 }),
    col.createIndex({ ip: 1, createdAt: -1 }),
  ])
    .then(() => {
      claimIndexesEnsured = true;
    })
    .catch(() => undefined);
}

export interface ClaimSlot {
  /** True when the slot was reserved and the caller may broadcast. */
  ok: boolean;
  /** Reservation id — pass to finalizeClaim or deletePendingClaim. */
  claimId?: ObjectId;
  /** Seconds until the cooldown clears, when ok is false. */
  retryAfterSeconds?: number;
}

/**
 * Atomically reserve a claim slot for (address, ip), closing the TOCTOU race a
 * naive check-then-send leaves open. We INSERT a PENDING row first, THEN look
 * for any *other* claim inside the cooldown window: because both concurrent
 * requests insert before they check, neither can slip through unseen. Fails
 * closed — if a rival row exists we delete our own and report the cooldown, so
 * the worst case is a spurious retry, never a double drip.
 *
 * On `ok`, the caller MUST settle the reservation: finalizeClaim once the drip
 * is broadcast, or deletePendingClaim if it fails.
 */
export async function claimSlot(address: string, ip: string | null): Promise<ClaimSlot> {
  const { cooldownHours, dripQuanta } = getFaucetConfig();
  const db = await getFaucetDb();
  const col = db.collection<FaucetClaim>(CLAIMS_COLLECTION);
  await ensureClaimIndexes(col);

  const now = new Date();
  const { insertedId } = await col.insertOne({
    address,
    ip,
    txHash: PENDING_TX,
    amount: dripQuanta,
    createdAt: now,
  });

  const windowStart = new Date(now.getTime() - cooldownHours * 3600 * 1000);
  const or: Record<string, unknown>[] = [{ address }];
  if (ip) or.push({ ip });

  const rival = await col.findOne(
    { _id: { $ne: insertedId }, createdAt: { $gt: windowStart }, $or: or },
    { sort: { createdAt: 1 } },
  );

  if (rival) {
    await col.deleteOne({ _id: insertedId }).catch(() => undefined);
    const elapsedMs = now.getTime() - rival.createdAt.getTime();
    const remainingMs = cooldownHours * 3600 * 1000 - elapsedMs;
    return { ok: false, retryAfterSeconds: Math.max(1, Math.ceil(remainingMs / 1000)) };
  }

  return { ok: true, claimId: insertedId };
}

/** Promote a reserved slot to a real claim once the drip is broadcast. */
export async function finalizeClaim(claimId: ObjectId, result: DripResult): Promise<void> {
  const db = await getFaucetDb();
  const col = db.collection<FaucetClaim>(CLAIMS_COLLECTION);
  await col.updateOne(
    { _id: claimId },
    { $set: { txHash: result.txHash, amount: result.amount, address: result.to } },
  );
}

/** Release a reserved slot when the drip fails, so the user can retry. */
export async function deletePendingClaim(claimId: ObjectId): Promise<void> {
  const db = await getFaucetDb();
  const col = db.collection<FaucetClaim>(CLAIMS_COLLECTION);
  await col.deleteOne({ _id: claimId }).catch(() => undefined);
}
