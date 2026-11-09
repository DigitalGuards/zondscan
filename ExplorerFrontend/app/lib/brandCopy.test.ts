/**
 * Brand copy tests: the rendered text of every page and component that names
 * the site.
 *
 *  1. Default identity: the capture equals app/lib/__fixtures__/brand-copy-default.json,
 *     taken from origin/dev before the copy moved onto lib/site.ts.
 *  2. Alternate identity (ShorScan): the same capture contains no `zondscan`
 *     in any letter case, apart from the allowlist below.
 *
 * Not renderable here (state-dependent text): orderbook/fund-flow.tsx error
 * and band-note copy, and the live pairing name inside getQrlConnect(), which
 * needs a browser window. The pairing name logic is tested through
 * dappDisplayName(); fund-flow is covered by the source guard.
 */
import fs from 'node:fs';
import path from 'node:path';
import { pageFiles, withSiteEnv, type SiteEnv } from './siteIdentityCollect.fixture';

jest.setTimeout(240000);
jest.mock('server-only', () => ({}));
jest.mock('@qrlwallet/connect', () => ({ QRLConnect: function QRLConnect() {}, ConnectionStatus: {} }));
jest.mock('next/navigation', () => ({
  usePathname: () => '/',
  useRouter: () => ({ push: () => undefined, replace: () => undefined, prefetch: () => undefined }),
  useSearchParams: () => new URLSearchParams(),
}));

const appDir = path.resolve(__dirname, '..');
const fixturePath = path.join(__dirname, '__fixtures__/brand-copy-default.json');

const RENDERED_PAGES = ['learn', 'api-explorer', 'faq'];

async function capture(env: SiteEnv) {
  const realFetch = global.fetch;
  global.fetch = () => Promise.reject(new Error('offline'));
  try {
    return await withSiteEnv(env, async () => {
    const { renderToStaticMarkup } = await import('react-dom/server');
    const { createElement } = await import('react');
    const out: Record<string, string> = {};
    const render = (node: unknown): string =>
      renderToStaticMarkup(node as Parameters<typeof renderToStaticMarkup>[0]);
    const files = pageFiles().filter((file) => {
      const rel = path.relative(appDir, file).split(path.sep).join('/');
      return (
        !rel.includes('[') &&
        RENDERED_PAGES.some((p) => rel === `${p}/page.tsx` || rel.startsWith(`${p}/`))
      );
    });
    for (const file of files) {
      const rel = path.relative(appDir, file).split(path.sep).join('/');
      try {
        const mod = (await import(file)) as {
          default: (props: unknown) => unknown;
        };
        const element = await mod.default({
          params: Promise.resolve({}),
          searchParams: Promise.resolve({}),
        });
        out[`page:${rel}`] = render(element);
      } catch (error) {
        out[`page:${rel}`] = `ERROR: ${(error as Error).message}`;
      }
    }
    const { default: SiteHeader } = await import('../components/SiteHeader');
    const { default: AnimatedLogo } = await import('../components/AnimatedLogo');
    const { default: Footer } = await import('../components/Footer');
    const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
    const queryClient = new QueryClient();
    out['component:SiteHeader'] = render(
      createElement(QueryClientProvider, { client: queryClient }, createElement(SiteHeader)),
    );
    // Query garbage-collection timers would keep the jest process alive.
    queryClient.clear();
    out['component:AnimatedLogo'] = render(createElement(AnimatedLogo));
    out['component:Footer'] = render(createElement(Footer));
    const { articleSummaries } = await import('./brandCopy.fixture');
    out['learn:articles'] = JSON.stringify(articleSummaries());
    const strings = await import('./brandCopy.fixture').then((m) => m.translatedSiteStrings());
    out['translations'] = JSON.stringify(strings);
    return out;
    });
  } finally {
    global.fetch = realFetch;
  }
}

it('writes or checks the default capture', async () => {
  const out = await capture({});
  if (process.env.WRITE_BRAND_FIXTURE) {
    fs.mkdirSync(path.dirname(fixturePath), { recursive: true });
    fs.writeFileSync(fixturePath, `${JSON.stringify(out, null, 2)}\n`);
    return;
  }
  const fixture = JSON.parse(fs.readFileSync(fixturePath, 'utf8')) as Record<string, string>;
  expect(Object.keys(out)).toEqual(Object.keys(fixture));
  for (const key of Object.keys(fixture)) expect([key, out[key]]).toEqual([key, fixture[key]]);
});

/**
 * `zondscan` mentions that stay in the alternate build on purpose: the
 * repository links, the logo image file names, and element ids or URL
 * fragments such as `what-is-zondscan`, and the `cd zondscan` line of the
 * repository clone command.
 */
const ALLOWED_ALTERNATE_MENTIONS: readonly RegExp[] = [
  /github\.com\/DigitalGuards\/zondscan/gi,
  /ZondScan_Logo\w*\.\w+/g,
  /[a-z0-9-]+-zondscan\b[a-z0-9-]*/g,
  /cd zondscan/g,
];

describe('alternate identity (ShorScan)', () => {
  let out: Record<string, string>;
  beforeAll(async () => {
    out = await capture({
      NEXT_PUBLIC_DOMAIN_NAME: 'https://shorscan.com',
      NEXT_PUBLIC_SITE_NAME: 'ShorScan',
    });
  });

  it('mentions no zondscan outside the allowlist', () => {
    const offenders: string[] = [];
    for (const [key, text] of Object.entries(out)) {
      const stripped = ALLOWED_ALTERNATE_MENTIONS.reduce((acc, re) => acc.replace(re, ''), text);
      for (const hit of stripped.match(/.{0,40}zondscan.{0,40}/gi) ?? []) {
        offenders.push(`${key}: ${hit}`);
      }
    }
    expect(offenders).toEqual([]);
  });

  it('uses the configured name', () => {
    expect(out['component:Footer']).toContain('Shor<span class="text-accent">Scan</span>');
    expect(out['page:learn/page.tsx']).toContain('ShorScan');
    const strings = JSON.parse(out.translations) as Record<string, string>;
    expect(strings['es:0']).toBe('Inicio de ShorScan');
    expect(strings['zh:0']).toBe('ShorScan 首页');
    expect(strings.pairingName).toBe('ShorScan');
  });
});
