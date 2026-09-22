export interface FaucetStatus {
  configured: boolean;
  captchaEnabled: boolean;
  turnstileSiteKey: string | null;
  dripQuanta: string;
  cooldownHours: number;
}

export function publicTurnstileSiteKey(value: unknown): string | null {
  return typeof value === 'string' && /^[A-Za-z0-9_-]{3,100}$/.test(value) ? value : null;
}

export function parseFaucetStatus(value: unknown): FaucetStatus | null {
  if (!value || typeof value !== 'object') return null;
  const status = value as Record<string, unknown>;
  if (
    typeof status.configured !== 'boolean' ||
    typeof status.captchaEnabled !== 'boolean' ||
    typeof status.dripQuanta !== 'string' ||
    !/^\d+$/.test(status.dripQuanta) ||
    BigInt(status.dripQuanta) <= BigInt(0) ||
    !Number.isSafeInteger(status.cooldownHours) ||
    (status.cooldownHours as number) <= 0
  )
    return null;
  return {
    configured: status.configured,
    captchaEnabled: status.captchaEnabled,
    turnstileSiteKey: publicTurnstileSiteKey(status.turnstileSiteKey),
    dripQuanta: status.dripQuanta,
    cooldownHours: status.cooldownHours as number,
  };
}

export interface TurnstileApi {
  render: (
    element: HTMLElement,
    options: {
      sitekey: string;
      theme: string;
      callback: (token: string) => void;
      'error-callback': () => void;
      'expired-callback': () => void;
    }
  ) => string;
  remove: (widgetId: string) => void;
}

/** A widget owns its callbacks until disposal, including during client navigation. */
export function mountFaucetCaptcha(
  api: TurnstileApi,
  element: HTMLElement,
  siteKey: string,
  onToken: (token: string) => void,
  onError: () => void
): () => void {
  let active = true;
  let widgetId: string | undefined;
  const fail = () => {
    if (active) {
      onToken('');
      onError();
    }
  };
  try {
    if (!publicTurnstileSiteKey(siteKey)) throw new Error('Invalid public site key');
    widgetId = api.render(element, {
      sitekey: siteKey,
      theme: 'dark',
      callback: (token) => {
        if (!active) return;
        if (typeof token !== 'string' || !token.trim() || token.length > 2048) {
          fail();
          return;
        }
        onToken(token);
      },
      'expired-callback': () => {
        if (active) onToken('');
      },
      'error-callback': fail,
    });
    if (typeof widgetId !== 'string' || !widgetId) throw new Error('Invalid widget');
  } catch {
    fail();
    active = false;
  }
  return () => {
    active = false;
    if (widgetId) {
      try {
        api.remove(widgetId);
      } catch {
        /* Callbacks remain invalidated. */
      }
      widgetId = undefined;
    }
  };
}
