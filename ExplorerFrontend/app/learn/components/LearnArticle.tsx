// Shared shell + building blocks for /learn article pages. Server
// component: the only client JS on an article page is the CopyButton
// inside CodeBlock. Long-form typography lives in the .learn-prose block
// in app/globals.css; the pieces here provide structure (steps, callouts,
// code blocks, FAQ) plus the article header, prev/next cards, and JSON-LD.

import Link from 'next/link';
import type { ReactNode } from 'react';
import Breadcrumbs from '../../components/Breadcrumbs';
import CopyButton from '../../components/CopyButton';
import { getLearnArticle, getPrevNext, LEARN_CATEGORIES } from '../articles';

/* ── Building blocks (named exports) ─────────────────────────────────── */

interface LearnSectionProps {
  /** Kebab-case anchor id for the section. */
  id: string;
  title: string;
  children: ReactNode;
}

export function LearnSection({ id, title, children }: LearnSectionProps): JSX.Element {
  return (
    <section id={id} className="scroll-mt-24">
      <h2>{title}</h2>
      {children}
    </section>
  );
}

export function StepList({ children }: { children: ReactNode }): JSX.Element {
  return <ol className="learn-steps">{children}</ol>;
}

export function Step({ title, children }: { title: string; children: ReactNode }): JSX.Element {
  return (
    <li>
      <p className="learn-step-title">{title}</p>
      {children}
    </li>
  );
}

type CalloutType = 'info' | 'warning' | 'tip';

interface CalloutProps {
  type: CalloutType;
  title?: string;
  children: ReactNode;
}

const CALLOUT_CLASS: Record<CalloutType, string> = {
  info: 'learn-callout-info',
  warning: 'learn-callout-warning',
  tip: 'learn-callout-tip',
};

const CALLOUT_DEFAULT_TITLE: Record<CalloutType, string> = {
  info: 'Note',
  warning: 'Warning',
  tip: 'Tip',
};

export function Callout({ type, title, children }: CalloutProps): JSX.Element {
  return (
    <aside role="note" className={`learn-callout ${CALLOUT_CLASS[type]}`}>
      <p className="learn-callout-title">{title ?? CALLOUT_DEFAULT_TITLE[type]}</p>
      {children}
    </aside>
  );
}

export function CodeBlock({ code, language }: { code: string; language?: string }): JSX.Element {
  return (
    <figure className="learn-codeblock">
      <figcaption className="flex items-center justify-between gap-2 px-4 py-2 border-b border-border">
        <span className="eyebrow">{language ?? 'code'}</span>
        <CopyButton text={code} label="Copy code" size="sm" />
      </figcaption>
      <pre>
        <code>{code}</code>
      </pre>
    </figure>
  );
}

export interface LearnFAQItem {
  q: string;
  a: ReactNode;
}

export function LearnFAQ({ items }: { items: LearnFAQItem[] }): JSX.Element {
  return (
    <section id="faq" className="scroll-mt-24">
      <h2>FAQ</h2>
      {items.map((item) => (
        <details key={item.q}>
          <summary>{item.q}</summary>
          <div className="learn-faq-answer">{item.a}</div>
        </details>
      ))}
    </section>
  );
}

/* ── Article shell (default export) ──────────────────────────────────── */

interface LearnArticleProps {
  slug: string;
  children: ReactNode;
}

export default function LearnArticle({ slug, children }: LearnArticleProps): JSX.Element {
  const article = getLearnArticle(slug);
  const { prev, next } = getPrevNext(slug);
  const category = LEARN_CATEGORIES[article.category];
  const url = `https://zondscan.com/learn/${article.slug}`;

  // TechArticle + BreadcrumbList structured data, same plain <script>
  // pattern as the site-wide @graph in app/layout.tsx.
  const structuredData = {
    '@context': 'https://schema.org',
    '@graph': [
      {
        '@type': 'TechArticle',
        headline: article.title,
        description: article.description,
        dateModified: article.updated,
        inLanguage: 'en',
        publisher: {
          '@type': 'Organization',
          name: 'DigitalGuards',
          url: 'https://digitalguards.nl',
          logo: 'https://zondscan.com/logo512.png',
        },
        mainEntityOfPage: url,
      },
      {
        '@type': 'BreadcrumbList',
        itemListElement: [
          { '@type': 'ListItem', position: 1, name: 'Home', item: 'https://zondscan.com' },
          { '@type': 'ListItem', position: 2, name: 'Learn', item: 'https://zondscan.com/learn' },
          { '@type': 'ListItem', position: 3, name: article.title, item: url },
        ],
      },
    ],
  };

  return (
    <div className="page-content py-4 sm:py-6 lg:py-8">
      <script
        type="application/ld+json"
        dangerouslySetInnerHTML={{ __html: JSON.stringify(structuredData) }}
      />
      <div className="max-w-3xl">
        <Breadcrumbs items={[{ label: 'Learn', href: '/learn' }, { label: article.title }]} />

        <header className="mb-8">
          <span className="chip mb-3">{category.label}</span>
          <h1 className="font-display text-2xl md:text-3xl font-semibold text-text-primary tracking-tight mb-3">
            {article.title}
          </h1>
          <p className="text-sm text-text-muted">
            Updated <time dateTime={article.updated}>{article.updated}</time>
            <span aria-hidden="true" className="mx-2">
              &middot;
            </span>
            {article.readingMinutes} min read
          </p>
        </header>

        <article className="learn-prose">{children}</article>

        {(prev !== null || next !== null) && (
          <nav aria-label="More learn articles" className="mt-12 grid gap-4 sm:grid-cols-2">
            {prev !== null ? (
              <Link href={`/learn/${prev.slug}`} className="card card-hover p-4 group">
                <p className="eyebrow mb-1.5">Previous</p>
                <p className="font-display text-sm font-semibold text-text-primary group-hover:text-accent transition-colors">
                  {prev.title}
                </p>
              </Link>
            ) : (
              <div aria-hidden="true" className="hidden sm:block" />
            )}
            {next !== null && (
              <Link href={`/learn/${next.slug}`} className="card card-hover p-4 group sm:text-right">
                <p className="eyebrow mb-1.5">Next</p>
                <p className="font-display text-sm font-semibold text-text-primary group-hover:text-accent transition-colors">
                  {next.title}
                </p>
              </Link>
            )}
          </nav>
        )}
      </div>
    </div>
  );
}
