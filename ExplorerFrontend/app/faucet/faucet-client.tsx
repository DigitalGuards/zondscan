'use client';

import { useEffect, useRef, useState } from 'react';
import type { FormEvent, ChangeEvent } from 'react';
import Link from 'next/link';
import Script from 'next/script';

import { formatDuration, NATIVE_UNIT } from '../lib/helpers';
import { canonicalizeQrlAddress } from '../lib/qrlAddress';
import { mountFaucetCaptcha, parseFaucetStatus, type FaucetStatus, type TurnstileApi } from './captcha';

interface ClaimSuccess {
  txHash: string;
  amount: string;
  to: string;
  explorerUrl: string;
}

// Minimal typing for the Cloudflare Turnstile global injected by api.js.
declare global {
  interface Window {
    turnstile?: TurnstileApi;
  }
}

// A claim resolves the moment the node accepts the broadcast (the tx is then
// pending on-chain), which can be well under 100ms. Floor the visible
// "Sending..." state so the action registers instead of flashing past.
const MIN_SENDING_MS = 700;

export default function FaucetClient(): JSX.Element {
  const [address, setAddress] = useState<string>('');
  const [status, setStatus] = useState<FaucetStatus | null>(null);
  const [loadingStatus, setLoadingStatus] = useState<boolean>(true);
  const [isLoading, setIsLoading] = useState<boolean>(false);
  const [result, setResult] = useState<ClaimSuccess | null>(null);
  const [error, setError] = useState<string | null>(null);
  // Mirrors tokenRef in React state so the submit button can react to whether a
  // captcha has been solved (refs don't trigger re-renders).
  const [captchaToken, setCaptchaToken] = useState<string>('');
  const [captchaError, setCaptchaError] = useState<string | null>(null);
  const [scriptReady, setScriptReady] = useState(false);
  const [captchaAttempt, setCaptchaAttempt] = useState(0);

  const turnstileRef = useRef<HTMLDivElement>(null);
  const disposeWidgetRef = useRef<(() => void) | null>(null);
  const tokenRef = useRef<string>('');
  const submittingRef = useRef(false);

  // Load faucet config so we can show the drip amount / disabled state and know
  // whether captcha is enforced. Until this resolves we render a spinner rather
  // than a half-initialised form (which could be submitted without a captcha
  // token, or flash before the offline notice). On failure, treat the faucet as
  // offline rather than leaving an interactive but broken form.
  useEffect(() => {
    let cancelled = false;
    const controller = new AbortController();
    const timer = setTimeout(() => controller.abort(), 15000);
    fetch('/faucet/claim', { cache: 'no-store', signal: controller.signal })
      .then(r => {
        if (!r.ok) throw new Error('Status request failed');
        return r.json();
      })
      .then((value: unknown) => {
        const s = parseFaucetStatus(value);
        if (!s) throw new Error('Invalid faucet status');
        if (!cancelled) {
          setStatus(s);
          setLoadingStatus(false);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setError('Faucet status is unavailable. Please reload the page.');
          setLoadingStatus(false);
        }
      })
      .finally(() => clearTimeout(timer));
    return () => {
      cancelled = true;
      clearTimeout(timer);
      controller.abort();
    };
  }, []);

  useEffect(() => {
    if (!status?.configured || !status.captchaEnabled) return;
    let dispose: (() => void) | undefined;
    let readinessTimer: ReturnType<typeof setTimeout> | undefined;
    // Cancel abandoned StrictMode setups before creating an external widget.
    const setupTimer = setTimeout(() => {
      tokenRef.current = '';
      setCaptchaToken('');
      if (!status.turnstileSiteKey) {
        setCaptchaError('Captcha is unavailable. Please try again later.');
        return;
      }
      if (!window.turnstile || !turnstileRef.current) {
        readinessTimer = setTimeout(() => {
          setCaptchaError('Captcha could not load. Please reload the page.');
        }, 15000);
        return;
      }
      setCaptchaError(null);
      dispose = mountFaucetCaptcha(
        window.turnstile,
        turnstileRef.current,
        status.turnstileSiteKey,
        (token) => {
          tokenRef.current = token;
          setCaptchaToken(token);
          if (token) setCaptchaError(null);
        },
        () => setCaptchaError('Captcha verification failed. Please try again.'),
      );
      disposeWidgetRef.current = dispose;
    }, 0);
    return () => {
      clearTimeout(setupTimer);
      clearTimeout(readinessTimer);
      dispose?.();
      disposeWidgetRef.current = null;
      tokenRef.current = '';
    };
  }, [status, scriptReady, captchaAttempt]);

  const handleSubmit = async (event: FormEvent<HTMLFormElement>): Promise<void> => {
    event.preventDefault();
    if (submittingRef.current || !status?.configured) return;
    if (status.captchaEnabled && (!status.turnstileSiteKey || !tokenRef.current || captchaError)) return;
    submittingRef.current = true;
    setIsLoading(true);
    setError(null);
    setResult(null);

    // Reject obviously malformed addresses up front so we don't burn a network
    // round-trip or a one-time Turnstile token on a typo.
    const cleanAddress = address.replace(/\s/g, '');
    const canonicalAddress = canonicalizeQrlAddress(cleanAddress);
    if (!canonicalAddress) {
      setError('Enter a valid QRL address with exactly 128 hex characters.');
      setIsLoading(false);
      submittingRef.current = false;
      return;
    }

    const startedAt = performance.now();
    const claimToken = tokenRef.current;
    // Consume the challenge before any asynchronous request or widget callback.
    disposeWidgetRef.current?.();
    tokenRef.current = '';
    setCaptchaToken('');
    try {
      const res = await fetch('/faucet/claim', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          address: canonicalAddress,
          turnstileToken: claimToken || undefined,
        }),
      });
      const data = await res.json();

      if (!res.ok) {
        if (res.status === 429 && data.retryAfterSeconds) {
          setError(`${data.error} (try again in ~${formatDuration(data.retryAfterSeconds)})`);
        } else {
          setError(data.error || 'Failed to claim testnet funds.');
        }
        return;
      }

      // Keep the spinner up for a perceptible beat even when the node accepts
      // the broadcast near-instantly, so a successful claim doesn't feel like a
      // no-op. Errors above skip this and surface immediately.
      const elapsed = performance.now() - startedAt;
      if (elapsed < MIN_SENDING_MS) {
        await new Promise(resolve => setTimeout(resolve, MIN_SENDING_MS - elapsed));
      }

      setResult(data as ClaimSuccess);
    } catch {
      setError('Network error. Please try again.');
    } finally {
      setIsLoading(false);
      submittingRef.current = false;
      setCaptchaAttempt(attempt => attempt + 1);
    }
  };

  const handleAddressChange = (e: ChangeEvent<HTMLInputElement>): void => {
    setAddress(e.target.value);
  };

  // Hold the UI until the faucet status is known, so the form never renders in
  // an ambiguous half-loaded state.
  if (loadingStatus) {
    return (
      <div className="max-w-3xl mx-auto px-4 sm:px-6 py-8 flex justify-center items-center min-h-[400px]">
        <div className="animate-spin h-8 w-8 border-4 border-accent border-t-transparent rounded-full" />
      </div>
    );
  }

  const disabled = !status?.configured;

  return (
    <div className="max-w-3xl mx-auto px-4 sm:px-6 py-8">
      {status?.configured && status.captchaEnabled && status.turnstileSiteKey && (
        <Script
          src="https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit"
          onReady={() => setScriptReady(true)}
          onError={() => {
            disposeWidgetRef.current?.();
            tokenRef.current = '';
            setCaptchaToken('');
            setCaptchaError('Captcha could not load. Please reload the page.');
          }}
        />
      )}
      <div className="flex flex-col items-center justify-center">
        <h2 className="section-title mb-2">QRL Testnet v3 Faucet</h2>
        <p className="text-text-secondary mb-8 text-center max-w-md">
          Get free testnet Quanta to experiment with transactions, contracts, and tooling.
          {status && (
            <>
              {' '}Sends <span className="text-accent font-semibold">{status.dripQuanta} {NATIVE_UNIT}</span> per
              address, once every {status.cooldownHours}h.
            </>
          )}
        </p>

        <div className="w-full max-w-md card p-8">
          {disabled ? (
            <div role="alert" className="w-full p-4 bg-background rounded-lg border border-yellow-500/40 text-center">
              <div className="text-sm text-yellow-300">
                {error || 'The faucet is currently offline. Please check back later.'}
              </div>
            </div>
          ) : (
            <form className="flex flex-col items-center space-y-6" onSubmit={handleSubmit}>
              <div className="relative w-full">
                <input
                  aria-label="QRL address"
                  className="w-full px-4 py-3 bg-background text-text-primary rounded-lg border border-border focus:outline-none focus:border-accent transition-all duration-300"
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

              {captchaError && (
                <div role="alert" className="text-sm text-error">{captchaError}</div>
              )}

              {result && (
                <div role="status" className="w-full p-4 bg-background rounded-lg border border-green-500/40">
                  <div className="text-sm text-text-secondary">
                    {result.amount} {NATIVE_UNIT} broadcast to your address
                  </div>
                  <Link
                    href={result.explorerUrl}
                    className="text-accent font-semibold break-all hover:underline"
                  >
                    {result.txHash}
                  </Link>
                  <div className="mt-1 text-xs text-text-secondary">
                    Transaction is pending: it should confirm within a minute.
                  </div>
                </div>
              )}

              {error && (
                <div role="alert" className="w-full p-4 bg-background rounded-lg border border-red-500/50">
                  <div className="text-sm text-error">{error}</div>
                </div>
              )}

              <button
                type="submit"
                disabled={isLoading || (status?.captchaEnabled === true && (!captchaToken || !!captchaError))}
                className="w-full px-6 py-3 bg-accent text-background font-semibold rounded-lg hover:bg-accent-hover hover:shadow-glow-accent transition-all duration-300 disabled:opacity-50 disabled:cursor-not-allowed shadow-lg"
              >
                {isLoading ? (
                  <div className="flex items-center justify-center">
                    <svg className="animate-spin -ml-1 mr-3 h-5 w-5 text-background" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                    </svg>
                    Sending...
                  </div>
                ) : (
                  `Request testnet ${NATIVE_UNIT}`
                )}
              </button>
            </form>
          )}
        </div>
      </div>
    </div>
  );
}
