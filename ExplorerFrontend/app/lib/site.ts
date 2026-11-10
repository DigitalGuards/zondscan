/**
 * Site identity: the public origin and brand name of this explorer build.
 *
 * Origin comes from `NEXT_PUBLIC_DOMAIN_NAME`, fixed at build time. The brand name comes from
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

/** Site name that selects the ShorScan mark, wordmark and icon set. */
export const SHORSCAN_NAME = 'ShorScan';

export interface SiteIdentity {
  readonly origin: string;
  readonly name: string;
}

/**
 * Normalises raw env values. The origin is parsed with `new URL()` and only
 * its `.origin` is kept, so a path, query, credentials or trailing slash are
 * dropped. Anything that is not a valid http(s) origin falls back to the
 * default, and so does an empty name.
 */
export function resolveSiteIdentity(
  rawOrigin: string | undefined,
  rawName: string | undefined,
): SiteIdentity {
  let origin = DEFAULT_SITE_ORIGIN;
  try {
    const parsed = new URL((rawOrigin ?? '').trim());
    if ((parsed.protocol === 'http:' || parsed.protocol === 'https:') && parsed.origin !== 'null') {
      origin = parsed.origin;
    }
  } catch {
    // Malformed value: keep the default.
  }
  const name = (rawName ?? '').trim();
  return { origin, name: name || DEFAULT_SITE_NAME };
}

// Both variables are NEXT_PUBLIC_*, so the bundler inlines them at build time
// on the server and in the browser. The runtime-only DOMAIN_NAME is not read:
// every page, route and client component must agree on one origin.
const identity = resolveSiteIdentity(
  process.env.NEXT_PUBLIC_DOMAIN_NAME,
  process.env.NEXT_PUBLIC_SITE_NAME,
);

/** Public origin without a trailing slash, e.g. `https://zondscan.com`. */
export const SITE_ORIGIN: string = identity.origin;

/** Brand name, e.g. `ZondScan`. */
export const SITE_NAME: string = identity.name;

/**
 * Name a paired wallet shows for this dApp. The default build keeps its
 * historical spelling `Zondscan`; any other site name is used as configured.
 */
export const PAIRING_NAME: string = identity.name === DEFAULT_SITE_NAME ? 'Zondscan' : identity.name;

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

/** Which logo mark a build shows. */
export type BrandMark = 'zond' | 'shor';

/** Icon files and manifest entries of one brand. Paths are site-absolute. */
export interface BrandAssets {
  readonly mark: BrandMark;
  readonly favicon: string;
  readonly favicon16: string;
  readonly favicon32: string;
  readonly favicon48: string;
  readonly appleTouchIcon: string;
  /** Manifest icon sources, relative like the web manifest lists them. */
  readonly manifestIco: string;
  readonly manifestPng512: string;
  /** Share card for OpenGraph and Twitter, 1200x630. */
  readonly ogImage: string;
  /** Monochrome Safari pinned-tab icon. */
  readonly maskIcon: string;
  /** Large PNG logo (site-absolute) used as the structured-data logo. */
  readonly logoPng: string;
  /** Text after the site name in the home page eyebrow. */
  readonly eyebrowSuffix: string;
  /** Chain keyword in the page keywords meta tag. */
  readonly chainKeyword: string;
}

const ZOND_ASSETS: BrandAssets = {
  mark: 'zond',
  favicon: '/favis/favicon.ico',
  favicon16: '/favis/favicon-16x16.png',
  favicon32: '/favis/favicon-32x32.png',
  favicon48: '/favis/favicon-48x48.png',
  appleTouchIcon: '/favis/apple-touch-icon.png',
  manifestIco: 'favicon.ico',
  manifestPng512: 'logo512.png',
  ogImage: '/og-image.png',
  maskIcon: '/favis/safari-pinned-tab.svg',
  logoPng: '/logo512.png',
  eyebrowSuffix: ' · QRL 2.0 · Zond Network',
  chainKeyword: 'Zond',
};

const SHOR_ASSETS: BrandAssets = {
  mark: 'shor',
  favicon: '/brand/shorscan/favicon.ico',
  favicon16: '/brand/shorscan/favicon-16x16.png',
  favicon32: '/brand/shorscan/favicon-32x32.png',
  favicon48: '/brand/shorscan/favicon-48x48.png',
  appleTouchIcon: '/brand/shorscan/apple-touch-icon.png',
  manifestIco: 'brand/shorscan/favicon.ico',
  manifestPng512: 'brand/shorscan/logo512.png',
  ogImage: '/brand/shorscan/og-image.png',
  maskIcon: '/brand/shorscan/safari-pinned-tab.svg',
  logoPng: '/brand/shorscan/logo512.png',
  eyebrowSuffix: ' · QRL 2.0 · QRL Network',
  chainKeyword: 'ShorScan',
};

/**
 * The one place that maps a site name to a brand. ShorScan gets its own mark
 * and icon set; every other name keeps the ZondScan assets.
 */
export function brandAssetsFor(name: string): BrandAssets {
  // Letter case is ignored, so "Shorscan" in a build setting still gets the
  // ShorScan assets that match its titles.
  return name.trim().toLowerCase() === SHORSCAN_NAME.toLowerCase() ? SHOR_ASSETS : ZOND_ASSETS;
}

/** Brand assets of this build. */
export const BRAND: BrandAssets = brandAssetsFor(SITE_NAME);

/**
 * Lowercase slug of the site name for section anchors: `ZondScan` ->
 * `zondscan`, `ShorScan` -> `shorscan`.
 */
export function brandSlug(name: string = SITE_NAME): string {
  return name
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

/** Slug of this build, e.g. `zondscan`. */
export const BRAND_SLUG: string = brandSlug(SITE_NAME);
