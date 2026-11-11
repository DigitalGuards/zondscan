/**
 * Brand switch tests: the site name selects the logo mark, wordmark and icon
 * set. The default name keeps the ZondScan assets, ShorScan gets its own, and
 * the icon files the switch points at exist and have the stated sizes.
 */
import fs from 'node:fs';
import path from 'node:path';
import { withSiteEnv } from './siteIdentityCollect.fixture';

jest.setTimeout(120000);
jest.mock('server-only', () => ({}));
jest.mock('next/navigation', () => ({
  usePathname: () => '/',
  useRouter: () => ({ push: () => undefined, replace: () => undefined, prefetch: () => undefined }),
  useSearchParams: () => new URLSearchParams(),
}));

const publicDir = path.resolve(__dirname, '..', '..', 'public');

async function renderBrand(name: string | undefined) {
  return withSiteEnv(name ? { NEXT_PUBLIC_SITE_NAME: name } : {}, async () => {
    const { renderToStaticMarkup } = await import('react-dom/server');
    const { createElement } = await import('react');
    const { QueryClient, QueryClientProvider } = await import('@tanstack/react-query');
    const { default: SiteHeader } = await import('../components/SiteHeader');
    const { default: AnimatedLogo } = await import('../components/AnimatedLogo');
    const queryClient = new QueryClient();
    const render = (node: unknown): string =>
      renderToStaticMarkup(node as Parameters<typeof renderToStaticMarkup>[0]);
    const header = render(
      createElement(QueryClientProvider, { client: queryClient }, createElement(SiteHeader)),
    );
    queryClient.clear();
    const logo = render(createElement(AnimatedLogo));
    const { sharedMetadata } = await import('./seo/metaData');
    const { renderManifest } = await import('./siteFiles');
    const { SITE_NAME } = await import('./site');
    return { header, logo, sharedMetadata, manifest: renderManifest({ name: SITE_NAME }) };
  });
}

describe('default site name (ZondScan)', () => {
  let out: Awaited<ReturnType<typeof renderBrand>>;
  beforeAll(async () => {
    out = await renderBrand(undefined);
  });

  it('keeps the image logo and the plain wordmark', () => {
    expect(out.header).toContain('ZondScan_Logo_static.webp');
    expect(out.header).not.toContain('viewBox="0 0 64 64"');
    expect(out.header).not.toContain('Zond<span');
    expect(out.logo).toContain('ZondScan_Logo_static.webp');
    expect(out.logo).not.toContain('<svg');
  });

  it('keeps the ZondScan icon files', () => {
    const icons = out.sharedMetadata.icons as { icon: { url: string }[]; apple: { url: string }[] };
    expect(icons.icon.map((i) => i.url)).toEqual([
      '/favis/favicon.ico',
      '/favis/favicon-16x16.png',
      '/favis/favicon-32x32.png',
      '/favis/favicon-48x48.png',
    ]);
    expect(icons.apple[0]?.url).toBe('/favis/apple-touch-icon.png');
    const manifestIcons = (out.manifest.icons as { src: string }[]).map((i) => i.src);
    expect(manifestIcons).toEqual(['favicon.ico', 'logo512.png']);
  });
});

describe('ShorScan site name', () => {
  let out: Awaited<ReturnType<typeof renderBrand>>;
  beforeAll(async () => {
    out = await renderBrand('ShorScan');
  });

  it('shows the inline mark and the two colour wordmark', () => {
    expect(out.header).not.toContain('ZondScan_Logo_static.webp');
    expect(out.header).toContain('viewBox="0 0 64 64"');
    expect(out.header).toContain('Shor<span class="text-accent">Scan</span>');
    expect(out.logo).toContain('viewBox="0 0 64 64"');
    expect(out.logo).not.toContain('ZondScan_Logo_static.webp');
  });

  it('draws 13 blocks and the scan line in theme colours', () => {
    const svg = /<svg[^>]*viewBox="0 0 64 64"[\s\S]*?<\/svg>/.exec(out.header)?.[0] ?? '';
    expect(svg.match(/<rect/g)).toHaveLength(14);
    expect(svg.match(/fill-text-primary/g)).toHaveLength(11);
    expect(svg.match(/fill-accent/g)).toHaveLength(3);
    expect(svg).not.toMatch(/fill="#/);
  });

  it('points favicons, app icons and the manifest at the ShorScan set', () => {
    const icons = out.sharedMetadata.icons as { icon: { url: string }[]; apple: { url: string }[] };
    const urls = [...icons.icon.map((i) => i.url), ...icons.apple.map((i) => i.url)];
    expect(urls).toEqual([
      '/brand/shorscan/favicon.ico',
      '/brand/shorscan/favicon-16x16.png',
      '/brand/shorscan/favicon-32x32.png',
      '/brand/shorscan/favicon-48x48.png',
      '/brand/shorscan/apple-touch-icon.png',
    ]);
    const manifestIcons = (out.manifest.icons as { src: string }[]).map((i) => i.src);
    expect(manifestIcons).toEqual(['brand/shorscan/favicon.ico', 'brand/shorscan/logo512.png']);
    for (const url of [...urls, ...manifestIcons.map((m) => `/${m}`)]) {
      expect(fs.existsSync(path.join(publicDir, url))).toBe(true);
    }
  });

  it('ships PNG icons with the stated sizes', () => {
    const sizes: Record<string, number> = {
      'favicon-16x16.png': 16,
      'favicon-32x32.png': 32,
      'favicon-48x48.png': 48,
      'apple-touch-icon.png': 180,
      'logo512.png': 512,
    };
    for (const [file, size] of Object.entries(sizes)) {
      const buf = fs.readFileSync(path.join(publicDir, 'brand', 'shorscan', file));
      expect(buf.readUInt32BE(16)).toBe(size);
      expect(buf.readUInt32BE(20)).toBe(size);
    }
  });
});

describe('ShorScan share card and mask icon', () => {
  it('ships a 1200x630 share card and a monochrome mask icon', () => {
    const png = fs.readFileSync(path.join(publicDir, 'brand', 'shorscan', 'og-image.png'));
    expect(png.readUInt32BE(16)).toBe(1200);
    expect(png.readUInt32BE(20)).toBe(630);
    const svg = fs.readFileSync(path.join(publicDir, 'brand', 'shorscan', 'safari-pinned-tab.svg'), 'utf8');
    expect(svg).toContain('viewBox="0 0 64 64"');
    expect(svg).not.toMatch(/fill=|style=|<image/);
  });

  it('keeps the default build on the existing files', async () => {
    const { brandAssetsFor } = await import('./site');
    const zond = brandAssetsFor('ZondScan');
    expect(zond.ogImage).toBe('/og-image.png');
    expect(zond.maskIcon).toBe('/favis/safari-pinned-tab.svg');
    expect(zond.logoPng).toBe('/logo512.png');
    expect(zond.eyebrowSuffix).toBe(' · QRL 2.0 · Zond Network');
  });

  it('rewrites the root icon files only in a ShorScan build', async () => {
    const config = (await import('../../next.config.js')) as {
      default: { rewrites: () => Promise<{ beforeFiles: { source: string; destination: string }[] }> };
    };
    const original = process.env.NEXT_PUBLIC_SITE_NAME;
    try {
      delete process.env.NEXT_PUBLIC_SITE_NAME;
      expect((await config.default.rewrites()).beforeFiles).toEqual([]);
      process.env.NEXT_PUBLIC_SITE_NAME = 'ShorScan';
      const rules = (await config.default.rewrites()).beforeFiles;
      expect(rules).toEqual([
        { source: '/favicon.ico', destination: '/brand/shorscan/favicon.ico' },
        { source: '/logo512.png', destination: '/brand/shorscan/logo512.png' },
      ]);
      for (const rule of rules) {
        expect(fs.existsSync(path.join(publicDir, rule.destination))).toBe(true);
      }
    } finally {
      if (original === undefined) delete process.env.NEXT_PUBLIC_SITE_NAME;
      else process.env.NEXT_PUBLIC_SITE_NAME = original;
    }
  });
});

describe('brandAssetsFor', () => {
  it('selects the ShorScan set only for the ShorScan name', async () => {
    const { brandAssetsFor } = await import('./site');
    expect(brandAssetsFor('ShorScan').mark).toBe('shor');
    expect(brandAssetsFor('Shorscan').mark).toBe('shor');
    expect(brandAssetsFor(' SHORSCAN ').mark).toBe('shor');
    expect(brandAssetsFor('ZondScan').mark).toBe('zond');
    expect(brandAssetsFor('Other').mark).toBe('zond');
  });
});
