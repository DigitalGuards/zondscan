import { SITE_ORIGIN, type SiteIdentity } from './site';

/**
 * Bodies of the generated site files (robots.txt, security.txt, web
 * manifest). Each takes the identity so tests can render alternates.
 */

export function renderRobotsTxt(origin: string = SITE_ORIGIN): string {
  return `# https://www.robotstxt.org/robotstxt.html
User-agent: *
Disallow:

Sitemap: ${origin}/sitemap.xml
`;
}

export function renderSecurityTxt(origin: string = SITE_ORIGIN): string {
  const host = new URL(origin).host;
  return `# ${host} security contact, per RFC 9116.
# Provided by DigitalGuards (KvK 91987482).

Contact: mailto:security@digitalguards.nl
Encryption: ${origin}/pgp-key.txt
# PGP key fingerprint: EAE7 9D7C 2805 9C17 4870  13CB BE48 C074 FFF0 495D
Expires: 2027-07-12T00:00:00.000Z
Preferred-Languages: en, nl
Canonical: ${origin}/.well-known/security.txt
Policy: https://qrlwallet.com/security
`;
}

export function renderManifest(identity: Pick<SiteIdentity, 'name'>): Record<string, unknown> {
  return {
    short_name: identity.name,
    name: `${identity.name} | QRL 2.0 Explorer`,
    icons: [
      {
        src: 'favicon.ico',
        sizes: '48x48 32x32 16x16',
        type: 'image/x-icon',
      },
      {
        src: 'logo512.png',
        type: 'image/png',
        sizes: '512x512',
      },
    ],
    start_url: '.',
    display: 'standalone',
    theme_color: '#0b0d13',
    background_color: '#0b0d13',
  };
}

