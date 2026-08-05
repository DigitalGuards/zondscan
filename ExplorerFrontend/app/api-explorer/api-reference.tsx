'use client';

/**
 * Endpoint reference for /api-explorer.
 *
 * Receives the fully precomputed endpoint data as props from the server
 * component (page.tsx), so every card is present in the served HTML and
 * crawlable. The only client behavior is the text filter at the top; its
 * initial state renders everything, which keeps the first client render
 * identical to the server render (no hydration mismatch).
 */

import { useMemo, useState } from 'react';
import MethodBadge from '../components/MethodBadge';
import CopyButton from '../components/CopyButton';

export interface EndpointParameterData {
  name: string;
  location: string;
  type: string;
  required: boolean;
  description: string;
}

export interface EndpointBodyFieldData {
  name: string;
  type: string;
  required: boolean;
  description: string;
}

export interface EndpointBodyData {
  contentType: string;
  fields: EndpointBodyFieldData[];
}

export interface EndpointCardData {
  anchor: string;
  method: string;
  path: string;
  summary: string;
  description: string;
  parameters: EndpointParameterData[];
  body: EndpointBodyData | null;
  curl: string;
  tryItUrl: string | null;
  exampleResponse: string | null;
}

export interface TagSectionData {
  tag: string;
  slug: string;
  description: string;
  endpoints: EndpointCardData[];
}

function ParameterTable({ parameters }: { parameters: EndpointParameterData[] }): JSX.Element {
  return (
    <div className="mt-4">
      <h4 className="eyebrow mb-2">Parameters</h4>
      <div className="well overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border">
              <th className="px-3 py-2 text-left text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted">Name</th>
              <th className="px-3 py-2 text-left text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted">In</th>
              <th className="px-3 py-2 text-left text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted">Type</th>
              <th className="px-3 py-2 text-left text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted">Required</th>
              <th className="px-3 py-2 text-left text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted">Description</th>
            </tr>
          </thead>
          <tbody>
            {parameters.map((p) => (
              <tr key={`${p.location}-${p.name}`} className="border-b border-border last:border-b-0 align-top">
                <td className="px-3 py-2 font-mono text-text-primary whitespace-nowrap">{p.name}</td>
                <td className="px-3 py-2 text-text-secondary whitespace-nowrap">{p.location}</td>
                <td className="px-3 py-2 font-mono text-xs text-text-secondary whitespace-nowrap">{p.type}</td>
                <td className="px-3 py-2 text-text-secondary whitespace-nowrap">{p.required ? 'yes' : 'no'}</td>
                <td className="px-3 py-2 text-text-secondary min-w-[16rem]">{p.description}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function BodyTable({ body }: { body: EndpointBodyData }): JSX.Element {
  return (
    <div className="mt-4">
      <h4 className="eyebrow mb-2">
        Request body{' '}
        <span className="normal-case tracking-normal font-mono text-text-secondary">({body.contentType})</span>
      </h4>
      <div className="well overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-border">
              <th className="px-3 py-2 text-left text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted">Field</th>
              <th className="px-3 py-2 text-left text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted">Type</th>
              <th className="px-3 py-2 text-left text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted">Required</th>
              <th className="px-3 py-2 text-left text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted">Description</th>
            </tr>
          </thead>
          <tbody>
            {body.fields.map((f) => (
              <tr key={f.name} className="border-b border-border last:border-b-0 align-top">
                <td className="px-3 py-2 font-mono text-text-primary whitespace-nowrap">{f.name}</td>
                <td className="px-3 py-2 font-mono text-xs text-text-secondary whitespace-nowrap">{f.type}</td>
                <td className="px-3 py-2 text-text-secondary whitespace-nowrap">{f.required ? 'yes' : 'no'}</td>
                <td className="px-3 py-2 text-text-secondary min-w-[16rem]">{f.description}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function EndpointCard({ endpoint }: { endpoint: EndpointCardData }): JSX.Element {
  return (
    <article id={endpoint.anchor} className="card p-4 sm:p-6 scroll-mt-20">
      <div className="flex flex-wrap items-center gap-2 sm:gap-3">
        <MethodBadge method={endpoint.method} />
        <code className="font-mono text-sm text-text-primary break-all">{endpoint.path}</code>
        <CopyButton text={endpoint.path} label={`Copy path ${endpoint.path}`} size="sm" />
        <a
          href={`#${endpoint.anchor}`}
          aria-label={`Link to ${endpoint.method} ${endpoint.path}`}
          className="text-text-muted hover:text-accent transition-colors font-mono text-sm"
        >
          #
        </a>
      </div>
      <h3 className="mt-3 font-display text-base font-semibold text-text-primary">{endpoint.summary}</h3>
      {endpoint.description !== '' && (
        <p className="mt-1.5 text-sm text-text-secondary leading-relaxed">{endpoint.description}</p>
      )}

      {endpoint.parameters.length > 0 && <ParameterTable parameters={endpoint.parameters} />}
      {endpoint.body && <BodyTable body={endpoint.body} />}

      <div className="mt-4">
        <div className="flex items-center justify-between mb-2">
          <h4 className="eyebrow">Example request</h4>
          <CopyButton text={endpoint.curl} label={`Copy curl command for ${endpoint.path}`} size="sm" />
        </div>
        <pre className="well overflow-x-auto p-3 text-xs font-mono text-text-primary leading-relaxed">
          <code>{endpoint.curl}</code>
        </pre>
      </div>

      {endpoint.tryItUrl && (
        <a
          href={endpoint.tryItUrl}
          target="_blank"
          rel="noopener noreferrer"
          className="btn-secondary inline-flex items-center gap-1.5 mt-3 text-sm"
        >
          Try it
          <svg
            xmlns="http://www.w3.org/2000/svg"
            className="h-3.5 w-3.5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            aria-hidden="true"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth={2}
              d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14"
            />
          </svg>
        </a>
      )}

      {endpoint.exampleResponse && (
        <div className="mt-4">
          <div className="flex items-center justify-between mb-2">
            <h4 className="eyebrow">Example response</h4>
            <CopyButton text={endpoint.exampleResponse} label={`Copy example response for ${endpoint.path}`} size="sm" />
          </div>
          <pre className="well overflow-x-auto overflow-y-auto max-h-80 p-3 text-xs font-mono text-text-secondary leading-relaxed">
            <code>{endpoint.exampleResponse}</code>
          </pre>
        </div>
      )}
    </article>
  );
}

export default function ApiReference({ sections }: { sections: TagSectionData[] }): JSX.Element {
  const [filter, setFilter] = useState('');
  const query = filter.trim().toLowerCase();

  const total = useMemo(() => sections.reduce((n, s) => n + s.endpoints.length, 0), [sections]);

  const visible = useMemo(() => {
    if (query === '') return sections;
    return sections
      .map((s) => ({
        ...s,
        endpoints: s.endpoints.filter(
          (e) => e.path.toLowerCase().includes(query) || e.summary.toLowerCase().includes(query),
        ),
      }))
      .filter((s) => s.endpoints.length > 0);
  }, [sections, query]);

  const visibleCount = useMemo(() => visible.reduce((n, s) => n + s.endpoints.length, 0), [visible]);

  return (
    <div>
      <div className="flex flex-col sm:flex-row sm:items-center gap-3 mb-8">
        <label htmlFor="endpoint-filter" className="sr-only">
          Filter endpoints by path or summary
        </label>
        <input
          id="endpoint-filter"
          type="search"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          placeholder="Filter by path or summary"
          className="input w-full sm:max-w-md text-sm"
        />
        <p className="text-sm text-text-muted whitespace-nowrap" aria-live="polite">
          {query === '' ? `${total} endpoints` : `${visibleCount} of ${total} endpoints`}
        </p>
      </div>

      {visibleCount === 0 && (
        <div className="card p-6 text-sm text-text-secondary">
          No endpoints match <span className="font-mono text-text-primary">{filter}</span>. Try a path fragment such as{' '}
          <span className="font-mono text-text-primary">/api/token</span> or a word from a summary.
        </div>
      )}

      <div className="space-y-12">
        {visible.map((section) => (
          <section key={section.slug} aria-labelledby={section.slug}>
            <div className="flex flex-wrap items-baseline gap-3 mb-1">
              <h2 id={section.slug} className="section-title scroll-mt-20">
                <a href={`#${section.slug}`}>{section.tag}</a>
              </h2>
              <span className="chip">
                {section.endpoints.length} endpoint{section.endpoints.length === 1 ? '' : 's'}
              </span>
            </div>
            {section.description !== '' && <p className="text-sm text-text-secondary mb-4">{section.description}</p>}
            <div className="space-y-6">
              {section.endpoints.map((endpoint) => (
                <EndpointCard key={endpoint.anchor} endpoint={endpoint} />
              ))}
            </div>
          </section>
        ))}
      </div>
    </div>
  );
}
