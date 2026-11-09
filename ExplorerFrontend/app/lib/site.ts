import config from '../../config';

/**
 * Site identity: the public origin and brand name of this explorer build.
 *
 * Origin comes from the existing domain config (`DOMAIN_NAME` on the server,
 * `NEXT_PUBLIC_DOMAIN_NAME` in the browser). The brand name comes from
 * `NEXT_PUBLIC_SITE_NAME`. With neither set the defaults reproduce the
 * zondscan.com build.
 *
 * Usage:
 *   siteUrl('/faq')        -> 'https://zondscan.com/faq'
 *   siteId('website')      -> 'https://zondscan.com/#website'
 *   pageTitle('Gas')       -> 'Gas | ZondScan'
 */

export const DEFAULT_SITE_ORIGIN = 'https://zondscan.com';
export const DEFAULT_SITE_NAME = 'ZondScan';

export interface SiteIdentity {
  readonly origin: string;
  readonly name: string;
}

/** Normalises raw env values: trims, drops trailing slashes, applies defaults. */
export function resolveSiteIdentity(
  rawOrigin: string | undefined,
  rawName: string | undefined,
): SiteIdentity {
  const origin = (rawOrigin ?? '').trim().replace(/\/+$/, '');
  const name = (rawName ?? '').trim();
  return {
    origin: /^https?:\/\//i.test(origin) ? origin : DEFAULT_SITE_ORIGIN,
    name: name || DEFAULT_SITE_NAME,
  };
}

const identity = resolveSiteIdentity(config.siteUrl, process.env.NEXT_PUBLIC_SITE_NAME);

/** Public origin without a trailing slash, e.g. `https://zondscan.com`. */
export const SITE_ORIGIN: string = identity.origin;

/** Brand name, e.g. `ZondScan`. */
export const SITE_NAME: string = identity.name;

/** Bare host of the origin, e.g. `zondscan.com`. */
export const SITE_HOST: string = new URL(SITE_ORIGIN).host;

/** Absolute URL for a site path. `path` must start with `/` (or be empty). */
export function siteUrl(path = ''): string {
  return `${SITE_ORIGIN}${path}`;
}

/** JSON-LD `@id` for a node of this site, e.g. `https://zondscan.com/#website`. */
export function siteId(fragment: string): string {
  return `${SITE_ORIGIN}/#${fragment}`;
}

/** Page title with the brand suffix: `Gas Tracker | ZondScan`. */
export function pageTitle(title: string): string {
  return `${title} | ${SITE_NAME}`;
}

/**
 * Splits the brand into a plain part and an accented last word for the
 * footer wordmark: `ZondScan` -> ['Zond', 'Scan'], `ShorScan` -> ['Shor', 'Scan'].
 * A name without a second capitalised word comes back whole with an empty accent.
 */
export function splitBrand(name: string = SITE_NAME): [string, string] {
  const match = /^(.+?)([A-Z][a-z0-9]*)$/.exec(name);
  return match ? [match[1], match[2]] : [name, ''];
}
