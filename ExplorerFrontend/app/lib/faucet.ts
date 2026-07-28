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
  /**
   * Hard cap on total quanta dripped per rolling 24h across ALL claimers.
   * 0 means unlimited. A safety net against draining when address rotation
   * defeats the per-address cooldown (a fresh address sidesteps it entirely).
   */
  dailyCapQuanta: number;
  /**
   * Whether claims are served when no captcha is configured. Defaults to true
   * outside production (so local dev works without Turnstile keys) and false in
   * production unless `FAUCET_ALLOW_NO_CAPTCHA=true` is set explicitly.
   */
  allowNoCaptcha: boolean;
}

/** Parse a whole, positive integer env var, falling back when absent/invalid. */
function parsePositiveInt(raw: string | undefined, fallback: number): number {
  const n = Number(raw);
  return Number.isInteger(n) && n > 0 ? n : fallback;
}

/** A drip amount must be a positive whole number of quanta; otherwise default to 10. */
function parseDripQuanta(raw: string | undefined): string {
  const v = (raw || '').trim();
  if (/^\d+$/.test(v) && BigInt(v) > BigInt(0)) return v;
  return '10';
}

// Log the "no captcha" warning at most once per process so an unprotected
// faucet is loud in the server logs without spamming on every request.
let warnedNoCaptcha = false;

export function getFaucetConfig(): FaucetConfig {
  const captchaEnabled = Boolean(
    process.env.TURNSTILE_SECRET && process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY,
  );
  // The faucet needs both a funding seed AND a Mongo connection for the
  // cooldown store; without the latter every claim would fail at runtime, so
  // we only report "online" when both are present.
  const configured = Boolean(process.env.FAUCET_SEED && process.env.DATABASE_URL);
  const isProd = process.env.NODE_ENV === 'production';
  const allowNoCaptcha = process.env.FAUCET_ALLOW_NO_CAPTCHA === 'true' || !isProd;

  if (configured && !captchaEnabled && !warnedNoCaptcha) {
    warnedNoCaptcha = true;
    console.warn(
      '[faucet] Turnstile is NOT configured - the faucet has no captcha protection. ' +
        'Set TURNSTILE_SECRET + NEXT_PUBLIC_TURNSTILE_SITE_KEY before exposing it publicly' +
        (isProd && !allowNoCaptcha
          ? '. Claims are disabled until captcha is configured (set FAUCET_ALLOW_NO_CAPTCHA=true to override).'
          : '.'),
    );
  }

  return {
    dripQuanta: parseDripQuanta(process.env.FAUCET_DRIP_QUANTA),
    cooldownHours: parsePositiveInt(process.env.FAUCET_COOLDOWN_HOURS, 24),
    rpcUrl: process.env.FAUCET_RPC_URL || DEFAULT_RPC_URL,
    configured,
    captchaEnabled,
    dailyCapQuanta: Math.max(0, Math.floor(Number(process.env.FAUCET_DAILY_CAP_QUANTA) || 0)),
    allowNoCaptcha,
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
      | 'DAILY_CAP'
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
 * `Q`+lowercase-hex form the @theqrl/web3 lib accepts for `to`. QRL addresses
 * are Q-prefixed - the `0x` prefix is only ever used for block/tx hashes - so we
 * accept a `Q`/`q` prefix or bare hex. Returns null when the value isn't a
 * 40-hex-character address.
 */
export function normalizeQrlAddress(input: string): string | null {
  if (!input) return null;
  let core = input.trim();
  if (/^[Qq]/.test(core)) core = core.slice(1);
  if (!/^[0-9a-fA-F]{40}$/.test(core)) return null;
  return 'Q' + core.toLowerCase();
}

/**
 * Collapse a client IP to the key used for the per-IP cooldown. IPv4 is used
 * verbatim, but IPv6 is reduced to its `/64` prefix: ISPs and clouds routinely
 * hand a single client a whole `/64` (2^64 addresses), so keying on the full
 * address would let one client rotate around the cooldown for free. On any
 * parse failure we fall back to the raw string (fail safe - still *some* key).
 */
export function ipCooldownKey(ip: string | null): string | null {
  if (!ip) return null;
  const raw = ip.trim();
  if (!raw) return null;
  // Strip an IPv6 zone id (e.g. fe80::1%eth0) before anything else.
  const addr = raw.split('%')[0];
  if (!addr.includes(':')) return addr; // IPv4 (or already a key)

  // IPv4-mapped IPv6 (e.g. ::ffff:127.0.0.1), which a dual-stack socket hands
  // back for plain IPv4 clients. Its first four hextets are all zero, so the
  // /64 reduction below would collapse every such client into one shared
  // bucket - key on the embedded IPv4 instead.
  const mapped = /^::ffff:(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})$/i.exec(addr);
  if (mapped) return mapped[1];

  // Expand `::` and take the first four hextets (the /64 network prefix).
  const halves = addr.split('::');
  if (halves.length > 2) return raw; // malformed - more than one '::'
  const head = halves[0] ? halves[0].split(':') : [];
  const tail = halves.length === 2 ? (halves[1] ? halves[1].split(':') : []) : [];
  let groups: string[];
  if (halves.length === 2) {
    const fill = 8 - head.length - tail.length;
    if (fill < 0) return raw;
    groups = [...head, ...Array(fill).fill('0'), ...tail];
  } else {
    groups = head;
  }
  if (groups.length < 4) return raw;
  const prefix = groups
    .slice(0, 4)
    .map(g => (g || '0').toLowerCase().replace(/^0+(?=.)/, ''));
  return prefix.join(':') + '::/64';
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
 * Serialize the read-nonce → sign → broadcast critical section. Concurrent
 * drips to *different* addresses would otherwise read the same `pending` nonce
 * and broadcast colliding transactions (one gets rejected as "nonce too low").
 * A simple promise chain funnels them through one at a time.
 *
 * NOTE: this guards a single process only. A horizontally-scaled deployment
 * with multiple instances sharing one faucet account would still need external
 * nonce coordination (or a single dedicated signer instance).
 */
let dripLock: Promise<unknown> = Promise.resolve();
function runExclusive<T>(fn: () => Promise<T>): Promise<T> {
  const result = dripLock.then(fn, fn);
  // Keep the chain alive whether fn resolves or rejects, but never let a
  // rejection propagate into the *next* waiter's run.
  dripLock = result.then(
    () => undefined,
    () => undefined,
  );
  return result;
}

/** Sum the quanta dripped (pending + finalized claims) since `windowStart`. */
async function drippedQuantaSince(windowStart: Date): Promise<bigint> {
  const db = await getFaucetDb();
  const col = db.collection<FaucetClaim>(CLAIMS_COLLECTION);
  const rows = await col
    .find({ createdAt: { $gt: windowStart } }, { projection: { amount: 1 } })
    .toArray();
  return rows.reduce((acc, r) => {
    const v = String(r.amount ?? '');
    return /^\d+$/.test(v) ? acc + BigInt(v) : acc;
  }, BigInt(0));
}

/**
 * Build, sign, and broadcast a native faucet transfer to `to` (already
 * normalised). Pre-checks the faucet balance so an empty/underfunded faucet
 * surfaces as a clean error rather than a node revert. Resolves once the node
 * has accepted the tx (transactionHash known), without waiting for a full
 * receipt, so the caller responds within a block time.
 */
export function sendFaucetDrip(to: string): Promise<DripResult> {
  // Serialize the whole sign+broadcast path so concurrent drips can't collide
  // on the faucet account's nonce (see runExclusive).
  return runExclusive(() => dripExclusive(to));
}

async function dripExclusive(to: string): Promise<DripResult> {
  const web3 = getWeb3();
  const seed = getSeed();
  const from = getFaucetAddress();
  const { dripQuanta, dailyCapQuanta } = getFaucetConfig();

  // Global safety cap: refuse once the rolling-24h total (this reservation
  // included, since claimSlot already inserted it) exceeds the configured
  // budget. Cheap backstop against draining via address rotation.
  if (dailyCapQuanta > 0) {
    const since = new Date(Date.now() - 24 * 3600 * 1000);
    const used = await drippedQuantaSince(since);
    if (used > BigInt(dailyCapQuanta)) {
      throw new FaucetError(
        'DAILY_CAP',
        'The faucet has reached its daily limit. Please try again tomorrow.',
      );
    }
  }

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
 * How long a PENDING reservation blocks rivals. A drip normally settles in
 * seconds; if the process dies between claimSlot and finalize/delete, the
 * orphaned PENDING row stops blocking after this window instead of locking the
 * address/IP for the full cooldown. The TTL index eventually deletes it.
 */
const RESERVATION_TTL_MS = 2 * 60 * 1000;

/**
 * Retention for the whole collection, used by the TTL index. Fixed (not derived
 * from cooldownHours) so re-deploys with a changed cooldown don't trigger an
 * IndexOptionsConflict. Must comfortably exceed both the cooldown window and the
 * 24h daily-cap window; 7 days covers any sane faucet config.
 */
const CLAIM_RETENTION_SECONDS = 7 * 24 * 3600;

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
    // TTL: bound collection growth and auto-clear stale rows (including orphaned
    // PENDING reservations) without a separate sweeper.
    col.createIndex({ createdAt: 1 }, { expireAfterSeconds: CLAIM_RETENTION_SECONDS }),
  ])
    .then(() => {
      claimIndexesEnsured = true;
    })
    .catch(() => undefined);
}

export interface ClaimSlot {
  /** True when the slot was reserved and the caller may broadcast. */
  ok: boolean;
  /** Reservation id - pass to finalizeClaim or deletePendingClaim. */
  claimId?: ObjectId;
  /** Seconds until the cooldown clears, when ok is false. */
  retryAfterSeconds?: number;
}

/**
 * Atomically reserve a claim slot for (address, ip), closing the TOCTOU race a
 * naive check-then-send leaves open. We INSERT a PENDING row first, THEN look
 * for any *other* claim inside the cooldown window: because both concurrent
 * requests insert before they check, neither can slip through unseen. Fails
 * closed - if a rival row exists we delete our own and report the cooldown, so
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

  // Key the per-IP cooldown on the /64 prefix for IPv6 so a single allocation
  // can't rotate around it (see ipCooldownKey).
  const ipKey = ipCooldownKey(ip);

  const now = new Date();
  const { insertedId } = await col.insertOne({
    address,
    ip: ipKey,
    txHash: PENDING_TX,
    amount: dripQuanta,
    createdAt: now,
  });

  const windowStart = new Date(now.getTime() - cooldownHours * 3600 * 1000);
  const reservationStart = new Date(now.getTime() - RESERVATION_TTL_MS);
  const keyMatch: Record<string, unknown>[] = [{ address }];
  if (ipKey) keyMatch.push({ ip: ipKey });

  // A rival blocks us if it shares the address/IP AND is either a finalized
  // claim inside the cooldown window, or a *recent* PENDING reservation. Older
  // PENDING rows are treated as abandoned (a crashed claim) and ignored, so they
  // can't lock the key for the full cooldown.
  const rival = await col.findOne(
    {
      _id: { $ne: insertedId },
      $and: [
        { $or: keyMatch },
        {
          $or: [
            { txHash: { $ne: PENDING_TX }, createdAt: { $gt: windowStart } },
            { txHash: PENDING_TX, createdAt: { $gt: reservationStart } },
          ],
        },
      ],
    },
    { sort: { createdAt: 1 } },
  );

  if (rival) {
    await col.deleteOne({ _id: insertedId }).catch(() => undefined);
    // A live finalized claim waits out the cooldown; a concurrent PENDING rival
    // (the simultaneous-first-claim case) only needs a brief retry.
    const isPendingRival = rival.txHash === PENDING_TX;
    const windowMs = isPendingRival ? RESERVATION_TTL_MS : cooldownHours * 3600 * 1000;
    const elapsedMs = now.getTime() - rival.createdAt.getTime();
    const remainingMs = windowMs - elapsedMs;
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
