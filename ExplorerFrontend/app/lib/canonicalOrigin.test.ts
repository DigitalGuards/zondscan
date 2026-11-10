/**
 * NEXT_PUBLIC_CANONICAL_ORIGIN: canonical links, og:url and page-identifying
 * structured data follow the setting; sitemap and robots keep the site's own
 * origin; an unset or invalid value changes nothing.
 */
import fs from 'node:fs';
import path from 'node:path';
import { withSiteEnv, type SiteEnv } from './siteIdentityCollect.fixture';

jest.setTimeout(120000);
jest.mock('server-only', () => ({}));
jest.mock('next/navigation', () => ({
  usePathname: () => '/',
  useRouter: () => ({ push: () => undefined, replace: () => undefined, prefetch: () => undefined }),
  useSearchParams: () => new URLSearchParams(),
}));
jest.mock('../address/[query]/address-view', () => ({ __esModule: true, default: () => null }));
jest.mock('../address/[query]/token-contract-view', () => ({
  __esModule: true,
  default: () => null,
}));
jest.mock('../validators/validators-client', () => ({ __esModule: true, default: () => null }));

const fixture = JSON.parse(
  fs.readFileSync(path.join(__dirname, '__fixtures__/default-site-output.json'), 'utf8'),
) as Record<string, unknown>;

const OLD = 'https://zondscan.com';
const NEW = 'https://shorscan.com';

async function collect(env: SiteEnv) {
  return withSiteEnv(env, async () => {
    const { collectPageMetadata } = await import('./siteIdentityCollect.fixture');
    const { metadata } = await import('../layout');
    const { structuredData } = await import('./seo/structuredData');
    const { default: sitemap } = await import('../sitemap');
    const { renderRobotsTxt } = await import('./siteFiles');
    const { learnMetadata } = await import('../learn/components/learn-metadata');
    const { renderToStaticMarkup } = await import('react-dom/server');
    const { createElement } = await import('react');
    const { default: LearnArticle } = await import('../learn/components/LearnArticle');
    const learnHtml = renderToStaticMarkup(createElement(LearnArticle, { slug: 'what-is-qrl-2', children: null }));
    const learnJsonLd = /<script type="application\/ld\+json">([\s\S]*?)<\/script>/.exec(learnHtml)?.[1] ?? '';
    return {
      learnJsonLd,
      pages: await collectPageMetadata(),
      layoutMetadata: JSON.parse(JSON.stringify(metadata)) as unknown,
      structuredData: JSON.parse(JSON.stringify(structuredData)) as unknown,
      sitemap: sitemap().map((entry) => ({ ...entry, lastModified: 'DATE' })),
      robots: renderRobotsTxt(),
      learn: JSON.parse(JSON.stringify(learnMetadata('what-is-qrl-2'))) as unknown,
    };
  });
}

interface PageMeta {
  alternates?: { canonical?: string };
  openGraph?: { url?: string };
}

describe('unset canonical origin', () => {
  let base: Awaited<ReturnType<typeof collect>>;
  beforeAll(async () => {
    base = await collect({});
  });

  it('keeps the pinned default output', () => {
    expect(base.pages).toEqual(fixture.pages);
    expect(base.layoutMetadata).toEqual(fixture.layoutMetadata);
    expect(base.learn).toEqual((fixture.pages as Record<string, unknown>)['learn/what-is-qrl-2/page.tsx']);
    expect(base.structuredData).toEqual(fixture.structuredData);
    expect(base.sitemap).toEqual(fixture.sitemap);
    expect(base.robots).toBe(fixture.robots);
  });

  it.each([
    'https://',
    '',
    'localhost',
    'ftp://shorscan.com',
    'javascript:alert(1)',
    'http://shorscan.com',
  ])('ignores the invalid or non-https value %j', async (raw) => {
    const out = await collect({ NEXT_PUBLIC_CANONICAL_ORIGIN: raw });
    expect(out).toEqual(base);
  });
});

describe('canonical origin set', () => {
  let base: Awaited<ReturnType<typeof collect>>;
  let out: Awaited<ReturnType<typeof collect>>;
  beforeAll(async () => {
    base = await collect({});
    out = await collect({ NEXT_PUBLIC_CANONICAL_ORIGIN: `${NEW}/ignored?x=1` });
  });

  it('moves canonical and og:url of a static route', () => {
    const page = out.pages['faq/page.tsx'] as PageMeta;
    expect(page.alternates?.canonical).toBe(`${NEW}/faq`);
    expect(page.openGraph?.url).toBe(`${NEW}/faq`);
    const layout = out.layoutMetadata as PageMeta;
    expect(layout.alternates?.canonical).toBe(NEW);
    expect(layout.openGraph?.url).toBe(NEW);
  });

  it('moves canonical and og:url of a dynamic route', () => {
    const block = out.pages['block/[query]/page.tsx'] as PageMeta;
    expect(block.alternates?.canonical).toBe(`${NEW}/block/1`);
    expect(block.openGraph?.url).toBe(`${NEW}/block/1`);
    const learn = out.learn as PageMeta;
    expect(learn.alternates?.canonical).toBe(`${NEW}/learn/what-is-qrl-2`);
    expect(learn.openGraph?.url).toBe(`${NEW}/learn/what-is-qrl-2`);
  });

  it('puts every canonical and og:url on the new origin with the same path', () => {
    const checked: string[] = [];
    for (const [file, meta] of Object.entries(out.pages)) {
      const before = base.pages[file] as PageMeta;
      const after = meta as PageMeta;
      if (typeof after === 'string') continue;
      for (const [b, a] of [
        [before.alternates?.canonical, after.alternates?.canonical],
        [before.openGraph?.url, after.openGraph?.url],
      ]) {
        if (b === undefined) continue;
        expect(a).toBe(b.replace(OLD, NEW));
        checked.push(file);
      }
    }
    expect(checked.length).toBeGreaterThan(60);
  });

  it('keeps sitemap and robots on the site origin', () => {
    expect(out.sitemap).toEqual(base.sitemap);
    expect(out.robots).toBe(base.robots);
    expect(out.robots).toContain(`Sitemap: ${OLD}/sitemap.xml`);
  });

  it('keeps structured data on the site origin', () => {
    expect(out.structuredData).toEqual(base.structuredData);
    const text = JSON.stringify(out.structuredData);
    expect(text).not.toContain(NEW);
    expect(text).toContain(`${OLD}/#website`);
    expect(out.learnJsonLd).toEqual(base.learnJsonLd);
    expect(out.learnJsonLd).toContain(`${OLD}/learn/what-is-qrl-2`);
    expect(out.learnJsonLd).not.toContain(NEW);
  });
});
