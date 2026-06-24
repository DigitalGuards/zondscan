import { NextResponse, type NextRequest } from 'next/server';

import {
  FaucetError,
  claimSlot,
  deletePendingClaim,
  finalizeClaim,
  getFaucetConfig,
  normalizeQrlAddress,
  sendFaucetDrip,
  verifyTurnstile,
} from '../../lib/faucet';

// Faucet signing uses MongoDB + the @theqrl post-quantum crypto bundle, neither
// of which run on the edge runtime. Never cache: every claim is a fresh write.
export const runtime = 'nodejs';
export const dynamic = 'force-dynamic';

/**
 * Best-effort client IP for the per-IP cooldown.
 *
 * `NextRequest.ip` was removed in Next 15, so we read headers. We prefer
 * `CF-Connecting-IP`, which Cloudflare sets to the true client and overwrites
 * on every request — clients can't forge it through the CF edge. We fall back
 * to `X-Real-IP`, then to the first hop of `X-Forwarded-For` only as a last
 * resort (spoofable when not fronted by a trusted proxy that rewrites it). All
 * deployments here sit behind Cloudflare, so the first branch is the live path;
 * captcha + the per-address cooldown remain the primary defenses regardless.
 */
function clientIp(req: NextRequest): string | null {
  const cf = req.headers.get('cf-connecting-ip');
  if (cf) return cf.trim();
  const real = req.headers.get('x-real-ip');
  if (real) return real.trim();
  const fwd = req.headers.get('x-forwarded-for');
  if (fwd) return fwd.split(',')[0].trim();
  return null;
}

/** GET /faucet/claim — public status so the page can render config/disabled state. */
export function GET(): NextResponse {
  const cfg = getFaucetConfig();
  return NextResponse.json({
    configured: cfg.configured,
    captchaEnabled: cfg.captchaEnabled,
    dripQuanta: cfg.dripQuanta,
    cooldownHours: cfg.cooldownHours,
  });
}

/** POST /faucet/claim — verify captcha + cooldown, then sign and broadcast a drip. */
export async function POST(req: NextRequest): Promise<NextResponse> {
  const cfg = getFaucetConfig();
  if (!cfg.configured) {
    return NextResponse.json(
      { error: 'The faucet is not currently available.' },
      { status: 503 },
    );
  }

  let payload: { address?: string; turnstileToken?: string };
  try {
    payload = await req.json();
    // A literal `null`/non-object body is valid JSON but would throw on the
    // property access below, outside any try/catch — guard it here.
    if (!payload || typeof payload !== 'object') throw new Error('non-object body');
  } catch {
    return NextResponse.json({ error: 'Invalid request body.' }, { status: 400 });
  }

  const address = normalizeQrlAddress(payload.address || '');
  if (!address) {
    return NextResponse.json(
      { error: 'Enter a valid QRL address (Q… followed by 40 hex characters).' },
      { status: 400 },
    );
  }

  const ip = clientIp(req);

  const captchaOk = await verifyTurnstile(payload.turnstileToken, ip);
  if (!captchaOk) {
    return NextResponse.json(
      { error: 'Captcha verification failed. Please try again.' },
      { status: 400 },
    );
  }

  // Reserve the cooldown slot BEFORE broadcasting. claimSlot inserts a pending
  // row and then checks for rivals, so two concurrent requests for the same
  // address/IP can't both pass the cooldown while the (slow) drip is in flight.
  // A Mongo outage here should degrade to a clean 503, not an unhandled 500.
  let slot: Awaited<ReturnType<typeof claimSlot>>;
  try {
    slot = await claimSlot(address, ip);
  } catch (err) {
    console.error('faucet cooldown check failed:', err);
    return NextResponse.json(
      { error: 'The faucet is temporarily unavailable. Please try again later.' },
      { status: 503 },
    );
  }
  if (!slot.ok || !slot.claimId) {
    const remaining = slot.retryAfterSeconds ?? 1;
    return NextResponse.json(
      {
        error: 'You have already claimed recently. Please wait before claiming again.',
        retryAfterSeconds: remaining,
      },
      { status: 429, headers: { 'Retry-After': String(remaining) } },
    );
  }

  try {
    const result = await sendFaucetDrip(address);
    // Promote the reservation to a real claim now that the tx is broadcast.
    await finalizeClaim(slot.claimId, result);

    return NextResponse.json({
      txHash: result.txHash,
      amount: result.amount,
      to: result.to,
      explorerUrl: `/tx/${result.txHash}`,
    });
  } catch (err) {
    // Drip failed — release the reservation so the user isn't stuck on cooldown.
    await deletePendingClaim(slot.claimId);
    if (err instanceof FaucetError) {
      const status =
        err.code === 'INSUFFICIENT_FAUCET_FUNDS' || err.code === 'NOT_CONFIGURED'
          ? 503
          : err.code === 'BROADCAST_FAILED'
            ? 502
            : 400;
      return NextResponse.json({ error: err.message }, { status });
    }
    console.error('faucet claim failed:', err);
    return NextResponse.json(
      { error: 'Something went wrong while sending testnet funds.' },
      { status: 500 },
    );
  }
}
