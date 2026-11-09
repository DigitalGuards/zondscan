/**
 * Site identity tests.
 *
 *  1. Default build: every metadata, JSON-LD, sitemap, robots, manifest,
 *     security.txt and served OpenAPI output equals the zondscan.com output
 *     captured from origin/dev before the identity module existed
 *     (app/lib/__fixtures__/default-site-output.json).
 *  2. Alternate build (ShorScan on shorscan.com): none of those outputs
 *     mention zondscan, apart from the deliberate cross-network links listed
 *     in ALLOWED_ALTERNATE_MENTIONS.
 *  3. Guard: no new hardcoded https://zondscan.com in app/ source.
 */
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';

jest.setTimeout(120000);
jest.mock('server-only', () => ({}));
jest.mock('../address/[query]/address-view', () => ({ __esModule: true, default: () => null }));
jest.mock('../address/[query]/token-contract-view', () => ({
  __esModule: true,
  default: () => null,
}));
jest.mock('../validators/validators-client', () => ({ __esModule: true, default: () => null }));

const fixture = JSON.parse(
  fs.readFileSync(path.join(__dirname, '__fixtures__/default-site-output.json'), 'utf8'),
) as Record<string, unknown>;

const ENV_KEYS = ['DOMAIN_NAME', 'NEXT_PUBLIC_DOMAIN_NAME', 'NEXT_PUBLIC_SITE_NAME'] as const;
const DEFAULT_ORIGIN = 'https://zondscan.com';

async function collectOutputs(env: Partial<Record<(typeof ENV_KEYS)[number], string>>) {
  const saved = ENV_KEYS.map((key) => [key, process.env[key]] as const);
  for (const key of ENV_KEYS) Reflect.deleteProperty(process.env, key);
  Object.assign(process.env, env);
  jest.resetModules();
  try {
    const { renderToStaticMarkup } = await import('react-dom/server');
    const { createElement } = await import('react');
    const { collectPageMetadata } = await import('./siteIdentityCollect.fixture');
    const { metadata } = await import('../layout');
    const { sharedMetadata } = await import('./seo/metaData');
    const { structuredData } = await import('./seo/structuredData');
    const { default: sitemap } = await import('../sitemap');
    const { renderRobotsTxt, renderSecurityTxt, renderManifest } = await import('./siteFiles');
    const { renderOpenApiSpec } = await import('./openapi');
    const { SITE_NAME, SITE_ORIGIN } = await import('./site');
    const { default: Footer } = await import('../components/Footer');
    const { GET: getSearch } = await import('../search/route');
    const { NextRequest } = await import('next/server');
    const identity = { origin: SITE_ORIGIN, name: SITE_NAME };
    const searchRedirect = getSearch(new NextRequest('http://localhost:3000/search?q=1')).headers.get(
      'location',
    );
    return {
      pages: await collectPageMetadata(),
      layoutMetadata: JSON.parse(JSON.stringify(metadata)) as unknown,
      sharedMetadata: JSON.parse(JSON.stringify(sharedMetadata)) as unknown,
      structuredData: JSON.parse(JSON.stringify(structuredData)) as unknown,
      sitemap: sitemap().map((entry) => ({ ...entry, lastModified: 'DATE' })),
      robots: renderRobotsTxt(),
      manifest: renderManifest(identity),
      securityTxt: renderSecurityTxt(),
      openapi: renderOpenApiSpec(identity),
      footerHtml: renderToStaticMarkup(createElement(Footer)),
      searchRedirect,
    };
  } finally {
    for (const [key, value] of saved) {
      if (value === undefined) Reflect.deleteProperty(process.env, key);
      else process.env[key] = value;
    }
    jest.resetModules();
  }
}

describe('default site identity', () => {
  let out: Awaited<ReturnType<typeof collectOutputs>>;
  beforeAll(async () => {
    out = await collectOutputs({});
  });

  it('keeps page metadata for every page unchanged', () => {
    expect(Object.keys(out.pages).length).toBeGreaterThanOrEqual(39);
    expect(out.pages).toEqual(fixture.pages);
  });

  it('keeps root layout and shared metadata unchanged', () => {
    expect(out.layoutMetadata).toEqual(fixture.layoutMetadata);
    expect(out.sharedMetadata).toEqual(fixture.sharedMetadata);
  });

  it('keeps JSON-LD structured data unchanged', () => {
    expect(out.structuredData).toEqual(fixture.structuredData);
  });

  it('keeps sitemap entries unchanged', () => {
    expect(out.sitemap).toEqual(fixture.sitemap);
  });

  it('keeps robots.txt byte for byte', () => {
    expect(out.robots).toBe(fixture.robots);
  });

  it('keeps the web manifest unchanged', () => {
    expect(out.manifest).toEqual(fixture.manifest);
  });

  it('keeps security.txt byte for byte', () => {
    expect(out.securityTxt).toBe(fixture.securityTxt);
  });

  it('serves the OpenAPI document unchanged', () => {
    const hash = crypto.createHash('sha256').update(JSON.stringify(out.openapi)).digest('hex');
    expect(hash).toBe(fixture.openapiSha256);
  });

  it('redirects search to the zondscan.com origin', () => {
    expect(out.searchRedirect).toBe('https://zondscan.com/block/1');
  });

  it('keeps the footer wordmark', () => {
    expect(out.footerHtml).toContain('Zond<span class="text-accent">Scan</span>');
    expect(out.footerHtml).toContain('ZondScan is your gateway to the QRL 2.0 network.');
  });
});

/**
 * Case-insensitive `zondscan` mentions that stay in the alternate build on
 * purpose. Each is a link to the explorer of a different network or to the
 * project repository, none is the site's own identity.
 */
const ALLOWED_ALTERNATE_MENTIONS: readonly RegExp[] = [
  // Network switcher entry for the zondscan.com testnet explorer lives in
  // app/lib/navigation.ts and is not part of any collected output. The
  // API explorer page and FAQ link the open-source repository.
  /github\.com\/DigitalGuards\/zondscan/gi,
];

function stripAllowed(text: string): string {
  return ALLOWED_ALTERNATE_MENTIONS.reduce((acc, re) => acc.replace(re, ''), text);
}

describe('alternate site identity (ShorScan on shorscan.com)', () => {
  let out: Awaited<ReturnType<typeof collectOutputs>>;
  beforeAll(async () => {
    out = await collectOutputs({
      DOMAIN_NAME: 'https://shorscan.com',
      NEXT_PUBLIC_DOMAIN_NAME: 'https://shorscan.com',
      NEXT_PUBLIC_SITE_NAME: 'ShorScan',
    });
  });

  const surfaces = [
    'pages',
    'layoutMetadata',
    'sharedMetadata',
    'structuredData',
    'sitemap',
    'robots',
    'manifest',
    'securityTxt',
    'openapi',
    'footerHtml',
    'searchRedirect',
  ] as const;

  it.each(surfaces)('%s mentions no zondscan', (surface) => {
    const text = stripAllowed(JSON.stringify(out[surface]));
    expect(text.match(/.{0,40}zondscan.{0,40}/gi) ?? []).toEqual([]);
  });

  it('uses the configured origin and name', () => {
    expect(out.sitemap[0]?.url).toBe('https://shorscan.com/');
    expect(out.robots).toContain('Sitemap: https://shorscan.com/sitemap.xml');
    expect(out.securityTxt).toContain('Canonical: https://shorscan.com/.well-known/security.txt');
    expect(out.securityTxt).toContain('# shorscan.com security contact');
    expect(out.manifest).toMatchObject({
      short_name: 'ShorScan',
      name: 'ShorScan | QRL 2.0 Explorer',
    });
    expect((out.layoutMetadata as { title: string }).title).toBe('ShorScan | QRL 2.0 Explorer');
    expect((out.openapi as { servers: { url: string }[] }).servers[0]?.url).toBe(
      'https://shorscan.com',
    );
    expect(out.footerHtml).toContain('Shor<span class="text-accent">Scan</span>');
    expect(out.searchRedirect).toBe('https://shorscan.com/block/1');
  });
});

describe('hardcoded origin guard', () => {
  // Files that may still name zondscan.com. navigation.ts links the testnet
  // explorer from the network switcher on purpose. site.ts holds the default.
  const ALLOWLIST = new Set([
    'lib/navigation.ts',
    'lib/site.ts',
    'lib/openapi.ts',
    'lib/siteIdentity.test.ts',
    'lib/siteIdentityCollect.fixture.ts',
  ]);

  function sourceFiles(dir: string, out: string[] = []): string[] {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        if (entry.name !== 'node_modules') sourceFiles(full, out);
      } else if (/\.(ts|tsx)$/.test(entry.name) && !/\.test\.tsx?$/.test(entry.name)) {
        out.push(full);
      }
    }
    return out;
  }

  it('finds no https://zondscan.com in app/ outside the allowlist', () => {
    const appDir = path.resolve(__dirname, '..');
    const offenders = sourceFiles(appDir)
      .filter((file) => !ALLOWLIST.has(path.relative(appDir, file).split(path.sep).join('/')))
      .filter((file) => /https?:\/\/(www\.)?zondscan\.com/.test(fs.readFileSync(file, 'utf8')))
      .map((file) => path.relative(appDir, file));
    expect(offenders).toEqual([]);
  });
});

describe('generated file routes (force-static)', () => {
  it('serve the pinned robots.txt, security.txt and manifest', async () => {
    const robots = await import('../robots.txt/route');
    const security = await import('../.well-known/security.txt/route');
    const manifest = await import('../manifest.json/route');
    expect(await robots.GET().text()).toBe(fixture.robots);
    expect(await security.GET().text()).toBe(fixture.securityTxt);
    expect(await manifest.GET().json()).toEqual(fixture.manifest);
  });
});

describe('origin resolution', () => {
  it('ignores the runtime DOMAIN_NAME', async () => {
    const out = await collectOutputs({ DOMAIN_NAME: 'http://localhost:3000' });
    expect(out.searchRedirect).toBe('https://zondscan.com/block/1');
    expect(out.sitemap).toEqual(fixture.sitemap);
    expect(out.layoutMetadata).toEqual(fixture.layoutMetadata);
  });

  it('reads NEXT_PUBLIC_DOMAIN_NAME even when DOMAIN_NAME differs', async () => {
    const out = await collectOutputs({
      DOMAIN_NAME: 'http://localhost:3000',
      NEXT_PUBLIC_DOMAIN_NAME: 'https://shorscan.com',
    });
    expect(out.searchRedirect).toBe('https://shorscan.com/block/1');
  });

  it.each([
    ['https://', DEFAULT_ORIGIN],
    ['', DEFAULT_ORIGIN],
    ['https://bad host.com', DEFAULT_ORIGIN],
    ['localhost', DEFAULT_ORIGIN],
    ['localhost:3000', DEFAULT_ORIGIN],
    ['ftp://shorscan.com', DEFAULT_ORIGIN],
    ['javascript:alert(1)', DEFAULT_ORIGIN],
    ['https://shorscan.com/', 'https://shorscan.com'],
    ['https://shorscan.com/path?q=1#h', 'https://shorscan.com'],
    ['  https://shorscan.com  ', 'https://shorscan.com'],
    ['http://localhost:3000', 'http://localhost:3000'],
  ])('resolves %j to %s', async (raw, expected) => {
    const { resolveSiteIdentity } = await import('./site');
    expect(resolveSiteIdentity(raw, undefined).origin).toBe(expected);
  });

  it('never throws at import time on a malformed origin', async () => {
    for (const raw of ['https://', 'https://bad host', '::::']) {
      const out = await collectOutputs({ NEXT_PUBLIC_DOMAIN_NAME: raw });
      expect(out.sitemap).toEqual(fixture.sitemap);
    }
  });
});

describe('translated brand strings', () => {
  const HOME = '{name} home';
  const CHOOSE = 'Choose how {name} looks and how you read the chain. Changes apply immediately.';

  it('renders the default name identically in every language', async () => {
    const { translate } = await import('./i18n');
    const name = { name: 'ZondScan' };
    expect(translate(HOME, 'en', name)).toBe('ZondScan home');
    expect(translate(HOME, 'zh', name)).toBe('ZondScan 首页');
    expect(translate(HOME, 'es', name)).toBe('Inicio de ZondScan');
    expect(translate(HOME, 'ru', name)).toBe('Главная ZondScan');
    expect(translate(CHOOSE, 'es', name)).toBe(
      'Personaliza la apariencia de ZondScan y cómo consultas la cadena. Los cambios se aplican al instante.',
    );
    expect(translate(CHOOSE, 'en', name)).toBe(
      'Choose how ZondScan looks and how you read the chain. Changes apply immediately.',
    );
  });

  it('carries an alternate name into a non-English language', async () => {
    const { translate } = await import('./i18n');
    expect(translate(HOME, 'es', { name: 'ShorScan' })).toBe('Inicio de ShorScan');
    expect(translate(CHOOSE, 'ru', { name: 'ShorScan' })).toBe(
      'Настройте оформление ShorScan и отображение данных блокчейна. Изменения применяются сразу.',
    );
  });
});

describe('hardcoded brand name guard', () => {
  // The default name lives in lib/site.ts; qrlConnect.ts keeps the historical
  // `Zondscan` pairing spelling for the default build only.
  const ALLOWLIST = new Set([
    'lib/site.ts',
    'lib/qrlConnect.ts',
    'lib/siteIdentityCollect.fixture.ts',
  ]);

  function sourceFiles(dir: string, out: string[] = []): string[] {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        if (entry.name !== 'node_modules') sourceFiles(full, out);
      } else if (/\.(ts|tsx)$/.test(entry.name) && !/\.test\.tsx?$/.test(entry.name)) {
        out.push(full);
      }
    }
    return out;
  }

  it('finds no literal ZondScan or Zondscan in strings or JSX text outside the allowlist', () => {
    const appDir = path.resolve(__dirname, '..');
    const offenders: string[] = [];
    for (const file of sourceFiles(appDir)) {
      const rel = path.relative(appDir, file).split(path.sep).join('/');
      if (ALLOWLIST.has(rel)) continue;
      fs.readFileSync(file, 'utf8')
        .split('\n')
        .forEach((line, index) => {
          const code = line
            .replace(/(^|\s)\/\/\s.*$/, '')
            .replace(/ZondScan_Logo\w*\.\w+/g, '');
          if (/^\s*(\*|\/\*)/.test(line)) return;
          if (/Zond[sS]can/.test(code)) offenders.push(`${rel}:${index + 1}: ${line.trim()}`);
        });
    }
    expect(offenders).toEqual([]);
  });
});
