import type { Metadata } from 'next';
import { sharedMetadata } from '../lib/seo/metaData';
import { getTaggedEndpoints } from '../lib/openapi';
import type { OpenApiOperation, OpenApiParameter } from '../lib/openapi';
import CopyButton from '../components/CopyButton';
import ApiReference from './api-reference';
import type { EndpointBodyData, EndpointCardData, TagSectionData } from './api-reference';

const DESCRIPTION =
  'Free public REST API for QRL 2.0 blockchain data: blocks, transactions, addresses, tokens, validators, and gas. No API key, no signup, and the full OpenAPI 3.1 spec is available at /openapi.json.';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'API Documentation | ZondScan',
  description: DESCRIPTION,
  alternates: {
    ...sharedMetadata.alternates,
    canonical: 'https://zondscan.com/api-explorer',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'API Documentation | ZondScan',
    description: DESCRIPTION,
    url: 'https://zondscan.com/api-explorer',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'API Documentation | ZondScan',
    description: DESCRIPTION,
  },
};

/* ── Spec-to-card precomputation (runs on the server) ──────────────────────── */

const SERVER = 'https://zondscan.com';
const BASE_URL = 'https://zondscan.com/api';

// Example values reused from the spec's own response examples so curl
// commands and Try-it links look like real traffic.
const EXAMPLE_WALLET = 'Q20d20b8026b8f02540249f42acbd6181dc4a0a48';
const EXAMPLE_CONTRACT = 'Q30b4e6b5d1a2c3f4e5d6a7b8c9d0e1f2a3b4c5d6';
const EXAMPLE_TX = '0x9c1d2e3f4a5b6c7d8e9f0a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d';

/** Path-parameter example values, keyed by spec path. */
const PATH_EXAMPLES: Record<string, Record<string, string>> = {
  '/api/block/{query}': { query: '148512' },
  '/api/tx/{query}': { query: EXAMPLE_TX },
  '/api/coinbase/{query}': { query: EXAMPLE_TX },
  '/api/pending-transaction/{hash}': { hash: EXAMPLE_TX },
  '/api/pending-tx-eta/{hash}': { hash: EXAMPLE_TX },
  '/api/address/aggregate/{query}': { query: EXAMPLE_WALLET },
  '/api/address/{address}/transactions': { address: EXAMPLE_WALLET },
  '/api/address/{address}/token-transfers': { address: EXAMPLE_WALLET },
  '/api/address/{address}/tokens': { address: EXAMPLE_WALLET },
  '/api/address/{address}/nfts': { address: EXAMPLE_WALLET },
  '/api/walletdistribution/{query}': { query: '100' },
  '/api/token/{address}/info': { address: EXAMPLE_CONTRACT },
  '/api/token/{address}/holders': { address: EXAMPLE_CONTRACT },
  '/api/token/{address}/tokens': { address: EXAMPLE_CONTRACT },
  '/api/token/{address}/transfers': { address: EXAMPLE_CONTRACT },
  '/api/token/{address}/{id}': { address: EXAMPLE_CONTRACT, id: '1' },
  '/api/validator/{id}': { id: '1' },
  '/api/epoch/{id}': { id: '1160' },
  '/api/contract/verify/{jobId}': { jobId: '9f2c4a6e8b0d1f3a' },
  '/api/contract/explain/{address}': { address: EXAMPLE_CONTRACT },
};

/** Query-string overrides for paths whose defaults alone make a poor example. */
const QUERY_EXAMPLES: Record<string, string> = {
  '/search': 'q=148512',
};

/** Full curl commands for the POST endpoints (bodies vary per endpoint). */
const POST_CURL: Record<string, string> = {
  '/api/getBalance': `curl -s -X POST "${SERVER}/api/getBalance" --data-urlencode "address=${EXAMPLE_WALLET}"`,
  '/api/contract/verify': `curl -s -X POST "${SERVER}/api/contract/verify" -H "Content-Type: application/json" -d '{"address":"${EXAMPLE_CONTRACT}","sourceCode":"<contract source>","contractName":"ExampleToken"}'`,
  '/api/contract/call': `curl -s -X POST "${SERVER}/api/contract/call" -H "Content-Type: application/json" -d '{"to":"${EXAMPLE_CONTRACT}","data":"0x06fdde03"}'`,
  '/api/contract/explain/{address}': `curl -s -X POST "${SERVER}/api/contract/explain/${EXAMPLE_CONTRACT}"`,
  '/faucet/claim': `curl -s -X POST "${SERVER}/faucet/claim" -H "Content-Type: application/json" -d '{"address":"${EXAMPLE_WALLET}","turnstileToken":"<turnstile-token>"}'`,
};

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function slugify(value: string): string {
  return value
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '');
}

function describeType(param: OpenApiParameter): string {
  const schema = param.schema ?? {};
  const enumRaw = schema['enum'];
  if (Array.isArray(enumRaw)) return enumRaw.map(String).join(' | ');
  const type = schema['type'];
  return typeof type === 'string' ? type : 'string';
}

function describeParam(param: OpenApiParameter): string {
  const parts: string[] = [];
  if (param.description) parts.push(param.description);
  const schema = param.schema ?? {};
  const def = schema['default'];
  if (def !== undefined) parts.push(`Default ${String(def)}.`);
  return parts.join(' ');
}

function getBodySummary(operation: OpenApiOperation): EndpointBodyData | null {
  if (!isRecord(operation.requestBody)) return null;
  const content = operation.requestBody['content'];
  if (!isRecord(content)) return null;
  const contentType = Object.keys(content)[0];
  if (!contentType) return null;
  const media = content[contentType];
  if (!isRecord(media)) return null;
  const schema = media['schema'];
  if (!isRecord(schema)) return null;
  const requiredRaw = schema['required'];
  const required = Array.isArray(requiredRaw)
    ? requiredRaw.filter((r): r is string => typeof r === 'string')
    : [];
  const props = schema['properties'];
  if (!isRecord(props)) return null;
  const fields = Object.entries(props).map(([name, def]) => {
    const field = isRecord(def) ? def : {};
    const type = typeof field['type'] === 'string' ? field['type'] : 'object';
    const description = typeof field['description'] === 'string' ? field['description'] : '';
    return { name, type, required: required.includes(name), description };
  });
  return { contentType, fields };
}

function getSuccessExample(operation: OpenApiOperation): string | null {
  const responses = operation.responses;
  if (!isRecord(responses)) return null;
  for (const code of ['200', '202']) {
    const response = responses[code];
    if (!isRecord(response)) continue;
    const content = response['content'];
    if (!isRecord(content)) continue;
    const media = content['application/json'];
    if (!isRecord(media)) continue;
    if ('example' in media) {
      return JSON.stringify(media['example'], null, 2);
    }
  }
  return null;
}

function resolvePath(path: string): string {
  const examples = PATH_EXAMPLES[path];
  if (!examples) return path;
  return path.replace(/\{([^}]+)\}/g, (match, name: string) => examples[name] ?? match);
}

function exampleQuery(path: string, parameters: OpenApiParameter[]): string {
  const override = QUERY_EXAMPLES[path];
  if (override !== undefined) return override;
  const parts: string[] = [];
  for (const param of parameters) {
    if (param.in !== 'query') continue;
    const def = (param.schema ?? {})['default'];
    if (def !== undefined) parts.push(`${param.name}=${String(def)}`);
  }
  return parts.join('&');
}

function buildRequest(
  method: string,
  path: string,
  parameters: OpenApiParameter[],
): { curl: string; tryItUrl: string | null } {
  if (method === 'GET') {
    const query = exampleQuery(path, parameters);
    const url = `${SERVER}${resolvePath(path)}${query ? `?${query}` : ''}`;
    return { curl: `curl -s "${url}"`, tryItUrl: url };
  }
  const preset = POST_CURL[path];
  return {
    curl: preset ?? `curl -s -X ${method} "${SERVER}${resolvePath(path)}"`,
    tryItUrl: null,
  };
}

function toCard(
  method: string,
  path: string,
  operation: OpenApiOperation,
  anchor: string,
): EndpointCardData {
  const parameters = operation.parameters ?? [];
  const { curl, tryItUrl } = buildRequest(method, path, parameters);
  return {
    anchor,
    method,
    path,
    summary: operation.summary ?? path,
    description: operation.description ?? '',
    parameters: parameters.map((p) => ({
      name: p.name,
      location: p.in,
      type: describeType(p),
      required: p.required === true,
      description: describeParam(p),
    })),
    body: getBodySummary(operation),
    curl,
    tryItUrl,
    exampleResponse: getSuccessExample(operation),
  };
}

const sections: TagSectionData[] = getTaggedEndpoints().map((group) => ({
  tag: group.tag,
  slug: slugify(group.tag),
  description: group.description ?? '',
  endpoints: group.endpoints.map((e) => toCard(e.method, e.path, e.operation, e.anchor)),
}));

const totalEndpoints = sections.reduce((n, s) => n + s.endpoints.length, 0);

const FIRST_REQUEST_CURL = `curl -s "${BASE_URL}/blocks?page=1&limit=5"`;

const PAGE_SECTIONS = [
  { id: 'getting-started', label: 'Getting started' },
  { id: 'conventions', label: 'Conventions' },
  { id: 'rate-limits', label: 'Rate limits and CORS' },
  { id: 'local-development', label: 'Run it locally' },
  { id: 'reference', label: 'Endpoint reference' },
];

/* ── Page ──────────────────────────────────────────────────────────────────── */

function MiniNav(): JSX.Element {
  return (
    <nav aria-label="API documentation sections" className="hidden xl:block">
      <div className="sticky top-6 max-h-[calc(100vh-3rem)] overflow-y-auto pr-2">
        <p className="eyebrow mb-3">On this page</p>
        <ul className="space-y-1">
          {PAGE_SECTIONS.map((s) => (
            <li key={s.id}>
              <a href={`#${s.id}`} className="nav-link text-sm block py-0.5">
                {s.label}
              </a>
            </li>
          ))}
        </ul>
        <p className="eyebrow mt-6 mb-3">Endpoints</p>
        <ul className="space-y-1">
          {sections.map((s) => (
            <li key={s.slug}>
              <a href={`#${s.slug}`} className="nav-link text-sm flex items-center justify-between gap-2 py-0.5">
                <span>{s.tag}</span>
                <span className="text-xs text-text-muted font-mono">{s.endpoints.length}</span>
              </a>
            </li>
          ))}
        </ul>
      </div>
    </nav>
  );
}

function Hero(): JSX.Element {
  return (
    <header className="mb-12">
      <p className="eyebrow mb-3">ZondScan API</p>
      <h1
        id="api-docs-heading"
        className="font-display text-3xl md:text-4xl font-semibold text-text-primary tracking-tight max-w-3xl"
      >
        Free public REST API for QRL 2.0 data. No API key. No signup.
      </h1>
      <p className="mt-4 text-text-secondary max-w-2xl leading-relaxed">
        {totalEndpoints} endpoints serve blocks, transactions, addresses, tokens, validators, gas data, and contract
        tooling for the QRL 2.0 public testnet, straight from the ZondScan indexer. The whole surface is described by
        one OpenAPI 3.1 document.
      </p>
      <div className="mt-6 flex flex-wrap gap-3">
        <a href="/openapi.json" className="btn-primary text-sm">
          OpenAPI 3.1 spec
        </a>
        <a
          href="https://editor-next.swagger.io/?url=https://zondscan.com/openapi.json"
          target="_blank"
          rel="noopener noreferrer"
          className="btn-secondary text-sm"
        >
          Open in Swagger Editor
        </a>
      </div>
    </header>
  );
}

function GettingStarted(): JSX.Element {
  return (
    <section id="getting-started" aria-labelledby="getting-started-heading" className="scroll-mt-20 mb-12">
      <h2 id="getting-started-heading" className="section-title mb-4">
        Getting started
      </h2>
      <div className="card p-4 sm:p-6 space-y-6">
        <div>
          <div className="flex items-center justify-between mb-2">
            <h3 className="eyebrow">Base URL</h3>
            <CopyButton text={BASE_URL} label="Copy base URL" size="sm" />
          </div>
          <pre className="well overflow-x-auto p-3 text-sm font-mono text-text-primary">
            <code>{BASE_URL}</code>
          </pre>
          <p className="text-sm text-text-muted mt-2">
            The search redirect and the faucet endpoints are served by the explorer web app at the site root; the
            reference below shows their full paths.
          </p>
        </div>
        <div>
          <div className="flex items-center justify-between mb-2">
            <h3 className="eyebrow">First request</h3>
            <CopyButton text={FIRST_REQUEST_CURL} label="Copy curl example" size="sm" />
          </div>
          <pre className="well overflow-x-auto p-3 text-sm font-mono text-text-primary">
            <code>{FIRST_REQUEST_CURL}</code>
          </pre>
          <p className="text-sm text-text-muted mt-2">
            This returns the five newest blocks as JSON. Every data endpoint responds with JSON and needs no
            authentication headers.
          </p>
        </div>
        <p className="text-sm text-text-secondary">
          New to QRL 2.0? The{' '}
          <a href="/learn" className="link-accent">
            Learn section
          </a>{' '}
          has guides on reading transactions, units, validators, and more.
        </p>
      </div>
    </section>
  );
}

function Conventions(): JSX.Element {
  return (
    <section id="conventions" aria-labelledby="conventions-heading" className="scroll-mt-20 mb-12">
      <h2 id="conventions-heading" className="section-title mb-4">
        Conventions
      </h2>
      <div className="grid gap-4 sm:grid-cols-2">
        <div className="card p-4 sm:p-5">
          <h3 className="font-display text-sm font-semibold text-text-primary mb-2">Pagination</h3>
          <p className="text-sm text-text-secondary leading-relaxed">
            List endpoints take <span className="chip font-mono">page</span> and{' '}
            <span className="chip font-mono">limit</span> query parameters, and limit is capped at 100 items. Most list
            endpoints number pages from 1. The contract and token list endpoints number pages from 0. Each endpoint
            below documents its own defaults.
          </p>
        </div>
        <div className="card p-4 sm:p-5">
          <h3 className="font-display text-sm font-semibold text-text-primary mb-2">Numeric encoding</h3>
          <p className="text-sm text-text-secondary leading-relaxed">
            On-chain quantities such as block fields, gas values, and raw transaction values are 0x-prefixed
            hexadecimal strings. Indexed aggregates are decimal strings or JSON numbers. Coin amounts are denominated
            in Quanta (1 Quanta = 10^9 Shor = 10^18 Planck), and fields carrying exact amounts use decimal strings. See
            the{' '}
            <a href="/learn/quanta-shor-planck" className="link-accent">
              units guide
            </a>{' '}
            or the{' '}
            <a href="/converter" className="link-accent">
              unit converter
            </a>
            .
          </p>
        </div>
        <div className="card p-4 sm:p-5">
          <h3 className="font-display text-sm font-semibold text-text-primary mb-2">Addresses</h3>
          <p className="text-sm text-text-secondary leading-relaxed">
            Addresses in responses are Q-prefixed, the canonical QRL 2.0 form, for example{' '}
            <span className="chip font-mono break-all">{EXAMPLE_WALLET}</span>. Most lookup parameters accept both
            the Q form and the 0x form of the same 40 hex characters; each parameter documents its accepted forms.
          </p>
        </div>
        <div className="card p-4 sm:p-5">
          <h3 className="font-display text-sm font-semibold text-text-primary mb-2">Caching</h3>
          <p className="text-sm text-text-secondary leading-relaxed">
            Hot endpoints are cached server side for 5 to 30 seconds, noted per endpoint in the reference below. The
            transaction detail endpoint is deliberately uncached so a newly mined transaction shows its true
            confirmation count immediately.
          </p>
        </div>
      </div>
    </section>
  );
}

function RateLimits(): JSX.Element {
  return (
    <section id="rate-limits" aria-labelledby="rate-limits-heading" className="scroll-mt-20 mb-12">
      <h2 id="rate-limits-heading" className="section-title mb-4">
        Rate limits and CORS
      </h2>
      <div className="card p-4 sm:p-6 space-y-4">
        <p className="text-sm text-text-secondary leading-relaxed">
          GET endpoints carry no rate limits today beyond fair use. They respond with{' '}
          <span className="chip font-mono">Access-Control-Allow-Origin: *</span>, so any web page can call them
          directly from the browser.
        </p>
        <p className="text-sm text-text-secondary leading-relaxed">
          POST endpoints allow cross-origin browser access only from the explorer&apos;s own origins and from
          browser-extension origins. Browser apps hosted elsewhere should call POST endpoints from their server. The
          three contract POST endpoints carry per-IP rate limits; when a limit is exhausted the API returns 429 with
          the body <span className="chip font-mono">{'{"error": "rate limit exceeded"}'}</span>.
        </p>
        <div className="well overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border">
                <th className="px-3 py-2 text-left text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted">
                  Endpoint
                </th>
                <th className="px-3 py-2 text-left text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted">
                  Per-IP bucket
                </th>
                <th className="px-3 py-2 text-left text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted">
                  Refill rate
                </th>
              </tr>
            </thead>
            <tbody>
              <tr className="border-b border-border">
                <td className="px-3 py-2 font-mono text-text-primary whitespace-nowrap">POST /api/contract/verify</td>
                <td className="px-3 py-2 text-text-secondary whitespace-nowrap">5 requests</td>
                <td className="px-3 py-2 text-text-secondary whitespace-nowrap">5 per minute</td>
              </tr>
              <tr className="border-b border-border">
                <td className="px-3 py-2 font-mono text-text-primary whitespace-nowrap">POST /api/contract/call</td>
                <td className="px-3 py-2 text-text-secondary whitespace-nowrap">60 requests</td>
                <td className="px-3 py-2 text-text-secondary whitespace-nowrap">60 per minute</td>
              </tr>
              <tr>
                <td className="px-3 py-2 font-mono text-text-primary whitespace-nowrap">
                  POST /api/contract/explain/{'{address}'}
                </td>
                <td className="px-3 py-2 text-text-secondary whitespace-nowrap">10 requests</td>
                <td className="px-3 py-2 text-text-secondary whitespace-nowrap">4 per minute</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p className="text-sm text-text-muted leading-relaxed">
          AI explanations are additionally capped at 5 regenerations per contract per rolling 7 day window. The faucet
          claim endpoint uses a cooldown per address and per IP (24 hours by default) and reports the remaining wait in
          a Retry-After header.
        </p>
      </div>
    </section>
  );
}

function LocalDevelopment(): JSX.Element {
  const cloneCommands = 'git clone https://github.com/DigitalGuards/zondscan.git\ncd zondscan\ndocker compose up -d';
  return (
    <section id="local-development" aria-labelledby="local-development-heading" className="scroll-mt-20 mb-12">
      <h2 id="local-development-heading" className="section-title mb-4">
        Run it locally
      </h2>
      <div className="card p-4 sm:p-6 space-y-4">
        <p className="text-sm text-text-secondary leading-relaxed">
          The whole explorer stack is open source: the chain synchronizer, the REST API, and this frontend live in one
          repository at{' '}
          <a
            href="https://github.com/DigitalGuards/zondscan"
            target="_blank"
            rel="noopener noreferrer"
            className="link-accent"
          >
            github.com/DigitalGuards/zondscan
          </a>
          . Docker Compose starts every service: the frontend serves on port 3000 and the API on port 8082.
        </p>
        <div>
          <div className="flex items-center justify-between mb-2">
            <h3 className="eyebrow">Quick start</h3>
            <CopyButton text={cloneCommands} label="Copy local setup commands" size="sm" />
          </div>
          <pre className="well overflow-x-auto p-3 text-sm font-mono text-text-primary leading-relaxed">
            <code>{cloneCommands}</code>
          </pre>
        </div>
      </div>
    </section>
  );
}

export default function ApiDocumentationPage(): JSX.Element {
  return (
    <div className="page-content py-6 lg:py-10">
      <div className="xl:grid xl:grid-cols-[220px_minmax(0,1fr)] xl:gap-10">
        <MiniNav />
        <div className="min-w-0">
          <Hero />
          <GettingStarted />
          <Conventions />
          <RateLimits />
          <LocalDevelopment />
          <section id="reference" aria-labelledby="reference-heading" className="scroll-mt-20">
            <h2 id="reference-heading" className="section-title mb-1">
              Endpoint reference
            </h2>
            <p className="text-sm text-text-secondary mb-6">
              Every endpoint, grouped the same way as the OpenAPI document. Each card links to itself, so anchors are
              shareable.
            </p>
            <ApiReference sections={sections} />
          </section>
        </div>
      </div>
    </div>
  );
}
