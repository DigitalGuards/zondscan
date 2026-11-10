import { BRAND_SLUG } from '../lib/site';

/** Brand slug of the original site, whose section ids old links still carry. */
const LEGACY_SLUG = 'zondscan';

/**
 * Id that an old deep link carries for a brand-slug section id, or null when
 * this build already uses the old slug.
 */
export function legacyAnchorId(id: string, slug: string = BRAND_SLUG): string | null {
  if (slug === LEGACY_SLUG || !id.includes(slug)) return null;
  return id.split(slug).join(LEGACY_SLUG);
}

/**
 * Invisible alias target so `#what-is-zondscan` still lands on the section in
 * a build with another brand. Absolutely positioned and empty: no layout
 * effect. Renders nothing in the default build.
 *
 * Usage: <LegacyAnchor id={`what-is-${BRAND_SLUG}`} />
 */
export default function LegacyAnchor({ id }: { id: string }): JSX.Element | null {
  const legacy = legacyAnchorId(id);
  if (!legacy) return null;
  return <span id={legacy} aria-hidden="true" className="absolute scroll-mt-24" />;
}
