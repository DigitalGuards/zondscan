import { NextResponse, type NextRequest } from 'next/server';

import {
  FaucetError,
  checkCooldown,
  getFaucetConfig,
  normalizeQrlAddress,
  recordClaim,
  sendFaucetDrip,
  verifyTurnstile,
} from '../../lib/faucet';

// Faucet signing uses MongoDB + the @theqrl post-quantum crypto bundle, neither
// of which run on the edge runtime. Never cache: every claim is a fresh write.
export const runtime = 'nodejs';
export const dynamic = 'force-dynamic';

/** First hop of X-Forwarded-For (the real client), falling back to X-Real-IP. */
function clientIp(req: NextRequest): string | null {
  const fwd = req.headers.get('x-forwarded-for');
  if (fwd) return fwd.split(',')[0].trim();
  return req.headers.get('x-real-ip');
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

  try {
    const remaining = await checkCooldown(address, ip);
    if (remaining !== null) {
      return NextResponse.json(
        {
          error: 'You have already claimed recently. Please wait before claiming again.',
          retryAfterSeconds: remaining,
        },
        { status: 429, headers: { 'Retry-After': String(remaining) } },
      );
    }

    const result = await sendFaucetDrip(address);
    await recordClaim(result, ip);

    return NextResponse.json({
      txHash: result.txHash,
      amount: result.amount,
      to: result.to,
      explorerUrl: `/tx/${result.txHash}`,
    });
  } catch (err) {
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
