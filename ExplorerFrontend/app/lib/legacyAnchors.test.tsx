/**
 * Old deep links, slug fallback, chain tag in the OpenAPI text and keyword
 * list: behaviour that differs between the default site name and others.
 */
import { withSiteEnv, type SiteEnv } from './siteIdentityCollect.fixture';

jest.setTimeout(120000);
jest.mock('server-only', () => ({}));
jest.mock('next/navigation', () => ({
  usePathname: () => '/',
  useRouter: () => ({ push: () => undefined, replace: () => undefined, prefetch: () => undefined }),
  useSearchParams: () => new URLSearchParams(),
}));

async function renderPages(env: SiteEnv) {
  return withSiteEnv(env, async () => {
    const { renderToStaticMarkup } = await import('react-dom/server');
    const render = (node: unknown): string =>
      renderToStaticMarkup(node as Parameters<typeof renderToStaticMarkup>[0]);
    const faq = (await import('../faq/page')) as { default: () => unknown };
    const guide = (await import('../learn/deploy-and-verify-contract/page')) as {
      default: () => unknown;
    };
    return { faq: render(await faq.default()), guide: render(await guide.default()) };
  });
}

describe('old deep links', () => {
  it('keep the brand-slug ids and add invisible zondscan aliases in a ShorScan build', async () => {
    const out = await renderPages({ NEXT_PUBLIC_SITE_NAME: 'ShorScan' });
    expect(out.faq).toContain('id="what-is-shorscan"');
    expect(out.faq).toMatch(/<span id="what-is-zondscan" aria-hidden="true" class="absolute[^"]*"><\/span>/);
    expect(out.faq).toContain('id="who-builds-zondscan"');
    expect(out.guide).toContain('id="verify-on-shorscan"');
    expect(out.guide).toMatch(/<span id="verify-on-zondscan" aria-hidden="true" class="absolute[^"]*"><\/span>/);
  });

  it('add nothing in the default build', async () => {
    const out = await renderPages({});
    expect(out.faq).toContain('id="what-is-zondscan"');
    expect(out.guide).toContain('id="verify-on-zondscan"');
    expect(out.faq.match(/id="what-is-zondscan"/g)).toHaveLength(1);
    expect(out.guide.match(/id="verify-on-zondscan"/g)).toHaveLength(1);
    expect(out.faq).not.toContain('aria-hidden="true" class="absolute');
  });

  it('maps ids through legacyAnchorId', async () => {
    const { legacyAnchorId } = await import('../components/LegacyAnchor');
    expect(legacyAnchorId('where-shorscan-fits', 'shorscan')).toBe('where-zondscan-fits');
    expect(legacyAnchorId('where-zondscan-fits', 'zondscan')).toBeNull();
    expect(legacyAnchorId('other-id', 'shorscan')).toBeNull();
  });
});

describe('brandSlug', () => {
  it('falls back to explorer when the name has no letters or digits', async () => {
    const { brandSlug } = await import('./site');
    expect(brandSlug('ShorScan')).toBe('shorscan');
    expect(brandSlug('Zond Scan!')).toBe('zond-scan');
    expect(brandSlug('---')).toBe('explorer');
    expect(brandSlug('   ')).toBe('explorer');
  });
});

describe('OpenAPI chain tag', () => {
  async function description(name: string, origin: string): Promise<string> {
    const { renderOpenApiSpec } = await import('./openapi');
    return renderOpenApiSpec({ origin, name }).info.description ?? '';
  }

  it('drops "(Zond)" for ShorScan', async () => {
    const text = await description('ShorScan', 'https://shorscan.com');
    expect(text).toContain('the QRL 2.0 public testnet');
    expect(text).not.toMatch(/zond/i);
  });

  it('keeps "(Zond)" for a ZondScan-named build on another origin', async () => {
    const text = await description('ZondScan', 'https://explorer.example');
    expect(text).toContain('QRL 2.0 (Zond) public testnet');
  });
});

describe('keywords meta tag', () => {
  async function keywords(env: SiteEnv): Promise<string> {
    return withSiteEnv(env, async () => {
      const { sharedMetadata } = await import('./seo/metaData');
      return String(sharedMetadata.keywords);
    });
  }

  it('lists no extra chain keyword for ShorScan', async () => {
    const text = await keywords({ NEXT_PUBLIC_SITE_NAME: 'ShorScan' });
    expect(text).toBe(
      'QRL, QRL 2.0, Quantum Resistant Ledger, blockchain explorer, post-quantum cryptography, Proof of Stake, Web3, EVM, quantum resistant, cryptocurrency, smart contracts, validators, transactions, blocks',
    );
    expect(text).not.toMatch(/shorscan|zond/i);
  });

  it('keeps Zond in the default list', async () => {
    expect(await keywords({})).toContain('Quantum Resistant Ledger, Zond, blockchain explorer');
  });
});
