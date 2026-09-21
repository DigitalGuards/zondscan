export const PREFERENCES_KEY = 'zondscan.preferences.v1';
export const PREFERENCES_EVENT = 'zondscan:preferences';

export type ThemePreference = 'light' | 'dim' | 'dark' | 'system';
export type ResolvedTheme = Exclude<ThemePreference, 'system'>;
export type AddressDisplay = 'middle' | 'back';
export type TimeZonePreference = 'utc' | 'local';
export type CurrencyPreference = 'USD' | 'EUR' | 'GBP' | 'CHF' | 'CAD' | 'AUD' | 'JPY' | 'CNY';
export type LocalePreference = 'en' | 'zh' | 'es' | 'ru';

export interface ExplorerPreferences {
  theme: ThemePreference;
  currency: CurrencyPreference;
  locale: LocalePreference;
  addressDisplay: AddressDisplay;
  timeZone: TimeZonePreference;
  expandDetails: boolean;
  hideZeroTokenTransfers: boolean;
  highlightAddresses: boolean;
}

export const DEFAULT_PREFERENCES: ExplorerPreferences = {
  theme: 'dark',
  currency: 'USD',
  locale: 'en',
  addressDisplay: 'middle',
  timeZone: 'utc',
  expandDetails: false,
  hideZeroTokenTransfers: true,
  highlightAddresses: true,
};

export function parsePreferences(raw: string | null): ExplorerPreferences {
  let value: Partial<ExplorerPreferences> = {};
  try {
    const parsed = JSON.parse(raw || '{}');
    if (parsed && typeof parsed === 'object' && !Array.isArray(parsed)) value = parsed;
  } catch {
    /* Invalid saved preferences use the defaults. */
  }
  return {
    theme: ['light', 'dim', 'dark', 'system'].includes(value.theme || '')
      ? value.theme!
      : DEFAULT_PREFERENCES.theme,
    currency: ['USD', 'EUR', 'GBP', 'CHF', 'CAD', 'AUD', 'JPY', 'CNY'].includes(
      value.currency || ''
    )
      ? value.currency!
      : 'USD',
    locale: ['en', 'zh', 'es', 'ru'].includes(value.locale || '') ? value.locale! : 'en',
    addressDisplay: value.addressDisplay === 'back' ? 'back' : 'middle',
    timeZone: value.timeZone === 'local' ? 'local' : 'utc',
    expandDetails: typeof value.expandDetails === 'boolean' ? value.expandDetails : false,
    hideZeroTokenTransfers:
      typeof value.hideZeroTokenTransfers === 'boolean' ? value.hideZeroTokenTransfers : true,
    highlightAddresses:
      typeof value.highlightAddresses === 'boolean' ? value.highlightAddresses : true,
  };
}

export function resolveTheme(theme: ThemePreference, systemDark: boolean): ResolvedTheme {
  return theme === 'system' ? (systemDark ? 'dark' : 'light') : theme;
}

export function displayAddress(
  address: string,
  mode: AddressDisplay,
  leading = 8,
  trailing = 6
): string {
  // Compact lists keep the short two-segment form on every address length so
  // rows stay on one line; detail pages render the full QIP-55 fingerprint
  // through AddressFingerprint where there is room for it.
  if (address.length <= leading + trailing + 3) return address;
  return mode === 'back'
    ? `${address.slice(0, leading + trailing)}...`
    : `${address.slice(0, leading)}...${address.slice(-trailing)}`;
}

export function unixSeconds(value: string | number | undefined | null): number | null {
  if (value === null || value === undefined || value === '') return null;
  const result = Number(value);
  return Number.isFinite(result) && result > 0 && result <= 8.64e12 ? result : null;
}

export function formatDateTime(
  seconds: number,
  timeZone: TimeZonePreference,
  locale: LocalePreference = 'en'
): string {
  return new Intl.DateTimeFormat(locale === 'en' ? 'en-GB' : locale, {
    day: '2-digit',
    month: 'short',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hourCycle: 'h23',
    timeZone: timeZone === 'utc' ? 'UTC' : undefined,
    timeZoneName: 'short',
  }).format(new Date(seconds * 1000));
}

/** Filter zero-quantity fungible transfers. NFT token IDs and native transfers are separate. */
export function isZeroTokenTransfer(transfer: {
  amount?: unknown;
  tokenStandard?: string;
}): boolean {
  if (transfer.tokenStandard && transfer.tokenStandard !== 'ERC-20') return false;
  return (
    typeof transfer.amount === 'string' && /^(?:0+(?:\.0+)?|0x0+)$/i.test(transfer.amount.trim())
  );
}

// Runs before visible content is parsed, so saved appearance also applies on a cold load.
export const THEME_BOOTSTRAP = `(()=>{try{const p=JSON.parse(localStorage.getItem('${PREFERENCES_KEY}')||'{}');const t=['light','dim','dark','system'].includes(p?.theme)?p.theme:'dark';const r=t==='system'?(matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light'):t;document.documentElement.dataset.theme=r;document.documentElement.classList.toggle('dark',r!=='light');document.documentElement.style.colorScheme=r==='light'?'light':'dark'}catch{}})();`;
