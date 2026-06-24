'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import type { FormEvent, ChangeEvent } from 'react';
import Link from 'next/link';
import Script from 'next/script';

interface FaucetStatus {
  configured: boolean;
  captchaEnabled: boolean;
  dripQuanta: string;
  cooldownHours: number;
}

interface ClaimSuccess {
  txHash: string;
  amount: string;
  to: string;
  explorerUrl: string;
}

// Minimal typing for the Cloudflare Turnstile global injected by api.js.
interface TurnstileApi {
  render: (
    el: HTMLElement,
    opts: { sitekey: string; callback: (token: string) => void; 'error-callback'?: () => void; 'expired-callback'?: () => void; theme?: string },
  ) => string;
  reset: (widgetId?: string) => void;
}
declare global {
  interface Window {
    turnstile?: TurnstileApi;
  }
}

const SITE_KEY = process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY;

export default function FaucetClient(): JSX.Element {
  const [address, setAddress] = useState<string>('');
  const [status, setStatus] = useState<FaucetStatus | null>(null);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [result, setResult] = useState<ClaimSuccess | null>(null);
  const [error, setError] = useState<string | null>(null);

  const turnstileRef = useRef<HTMLDivElement>(null);
  const widgetIdRef = useRef<string | null>(null);
  const tokenRef = useRef<string>('');

  // Load faucet config so we can show the drip amount / disabled state and know
  // whether captcha is enforced.
  useEffect(() => {
    let cancelled = false;
    fetch('/faucet/claim')
      .then(r => r.json())
      .then((s: FaucetStatus) => {
        if (!cancelled) setStatus(s);
      })
      .catch(() => {
        if (!cancelled) setStatus(null);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  // Render the widget when every prerequisite is in place. We gate on
  // `window.turnstile` existing rather than a "script loaded" state flag: that
  // way the render is driven purely by external readiness and can be retried
  // safely from both the status effect and the script's onLoad — including the
  // client-side-nav case where the script is already cached and onLoad never
  // refires. Idempotent: the widgetIdRef guard prevents a double render.
  const renderTurnstile = useCallback(() => {
    if (
      !status?.captchaEnabled ||
      !SITE_KEY ||
      !turnstileRef.current ||
      !window.turnstile ||
      widgetIdRef.current !== null
    ) {
      return;
    }
    widgetIdRef.current = window.turnstile.render(turnstileRef.current, {
      sitekey: SITE_KEY,
      theme: 'dark',
      callback: (token: string) => {
        tokenRef.current = token;
      },
      'expired-callback': () => {
        tokenRef.current = '';
      },
      'error-callback': () => {
        tokenRef.current = '';
      },
    });
  }, [status?.captchaEnabled]);

  // Retry the render whenever its inputs change (notably once the status fetch
  // resolves). If the script hasn't loaded yet this is a no-op and onLoad will
  // drive it; if the script is already present this is what renders the widget.
  useEffect(() => {
    renderTurnstile();
  }, [renderTurnstile]);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>): Promise<void> => {
    event.preventDefault();
    setIsLoading(true);
    setError(null);
    setResult(null);

    try {
      const res = await fetch('/faucet/claim', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          address: address.replace(/\s/g, ''),
          turnstileToken: tokenRef.current || undefined,
        }),
      });
      const data = await res.json();

      if (!res.ok) {
        if (res.status === 429 && data.retryAfterSeconds) {
          const mins = Math.ceil(data.retryAfterSeconds / 60);
          setError(`${data.error} (try again in ~${mins} min)`);
        } else {
          setError(data.error || 'Failed to claim testnet funds.');
        }
        return;
      }
      setResult(data as ClaimSuccess);
    } catch {
      setError('Network error. Please try again.');
    } finally {
      setIsLoading(false);
      // One token per challenge — reset so the next claim gets a fresh one.
      if (widgetIdRef.current && window.turnstile) {
        window.turnstile.reset(widgetIdRef.current);
        tokenRef.current = '';
      }
    }
  };

  const handleAddressChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setAddress(e.target.value);
  };

  const disabled = status !== null && !status.configured;

  return (
    <div className="max-w-[1200px] mx-auto p-8">
      {status?.captchaEnabled && (
        <Script
          src="https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit"
          onLoad={() => renderTurnstile()}
        />
      )}
      <div className="flex flex-col items-center justify-center">
        <h2 className="text-2xl font-bold mb-2 text-accent">QRL 2.0 Testnet Faucet</h2>
        <p className="text-gray-400 mb-8 text-center max-w-md">
          Get free testnet QRL to experiment with transactions, contracts, and tooling.
          {status && (
            <>
              {' '}Sends <span className="text-accent font-semibold">{status.dripQuanta} QRL</span> per
              address, once every {status.cooldownHours}h.
            </>
          )}
        </p>

        <div className="w-full max-w-md bg-card-gradient p-8 rounded-lg border border-border shadow-xl">
          {disabled ? (
            <div role="alert" className="w-full p-4 bg-background rounded-lg border border-yellow-500/40 text-center">
              <div className="text-sm text-yellow-300">
                The faucet is currently offline. Please check back later.
              </div>
            </div>
          ) : (
            <form className="flex flex-col items-center space-y-6" onSubmit={handleSubmit}>
              <div className="relative w-full">
                <input
                  aria-label="QRL address"
                  className="w-full px-4 py-3 bg-background text-white rounded-lg border border-border focus:outline-none focus:border-accent transition-all duration-300"
                  type="text"
                  value={address}
                  onChange={handleAddressChange}
                  placeholder="Enter your QRL testnet address (Q…)"
                  spellCheck={false}
                  autoComplete="off"
                  required
                />
              </div>

              {status?.captchaEnabled && <div ref={turnstileRef} className="self-center" />}

              {result && (
                <div role="status" className="w-full p-4 bg-background rounded-lg border border-green-500/40">
                  <div className="text-sm text-gray-400">Sent {result.amount} QRL</div>
                  <Link
                    href={result.explorerUrl}
                    className="text-accent font-semibold break-all hover:underline"
                  >
                    {result.txHash}
                  </Link>
                </div>
              )}

              {error && (
                <div role="alert" className="w-full p-4 bg-background rounded-lg border border-red-500/50">
                  <div className="text-sm text-red-400">{error}</div>
                </div>
              )}

              <button
                type="submit"
                disabled={isLoading}
                className="w-full px-6 py-3 bg-gradient-to-r from-accent to-accent-hover text-white font-bold rounded-lg hover:from-accent-hover hover:to-accent transition-all duration-300 disabled:opacity-50 disabled:cursor-not-allowed shadow-lg"
              >
                {isLoading ? (
                  <div className="flex items-center justify-center">
                    <svg className="animate-spin -ml-1 mr-3 h-5 w-5 text-white" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                    Sending...
                  </div>
                ) : (
                  'Request testnet QRL'
                )}
              </button>
            </form>
          )}
        </div>
      </div>
    </div>
  );
}
