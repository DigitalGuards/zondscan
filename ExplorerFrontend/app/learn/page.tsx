import type { Metadata } from 'next';
import Link from 'next/link';
import { sharedMetadata } from '../lib/seo/metaData';
import {
  LEARN_CATEGORIES,
  LEARN_CATEGORY_ORDER,
  learnArticles,
} from './articles';

const TITLE = 'Learn QRL 2.0 | ZondScan';
const DESCRIPTION =
  'Plain English guides to QRL 2.0: post-quantum signatures, proof of stake, wallets, staking, swaps, tokens, and smart contracts. Written by the team behind ZondScan and MyQRLWallet.';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: TITLE,
  description: DESCRIPTION,
  alternates: {
    ...sharedMetadata.alternates,
    canonical: 'https://zondscan.com/learn',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: TITLE,
    description: DESCRIPTION,
    url: 'https://zondscan.com/learn',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: TITLE,
    description: DESCRIPTION,
  },
};

interface ExternalResource {
  title: string;
  description: string;
  href: string;
  domain: string;
}

const EXTERNAL_RESOURCES: readonly ExternalResource[] = [
  {
    title: 'QRL Ecosystem Index',
    description:
      'A community-maintained directory of projects, tools, services, and resources across QRL 1.x and QRL 2.0, with a submission process for new entries.',
    href: 'https://www.qrlecosystem.com/',
    domain: 'qrlecosystem.com',
  },
  {
    title: 'Blockchain Quantum Readiness Index',
    description:
      'An independent index that scores over a hundred blockchains on quantum readiness, with per-project reports and a published methodology. QRL sits in its top quantum-ready tier.',
    href: 'https://qrindex.org/',
    domain: 'qrindex.org',
  },
];

export default function LearnIndexPage(): JSX.Element {
  return (
    <div className="page-content py-4 sm:py-6 lg:py-8">
      <header className="mb-10 max-w-3xl">
        <h1 className="font-display text-2xl md:text-3xl font-semibold text-text-primary tracking-tight mb-3">
          Learn QRL 2.0
        </h1>
        <p className="text-text-secondary leading-relaxed">
          Plain English guides to QRL 2.0: how post-quantum signatures and proof of
          stake work, and how to use wallets, staking, swaps, tokens, and smart
          contracts on the network. Written by the team that builds ZondScan and
          MyQRLWallet. Every guide runs on the public testnet, where coins have no
          monetary value, so you can follow along and experiment freely.
        </p>
      </header>

      {LEARN_CATEGORY_ORDER.map((categoryId) => {
        const category = LEARN_CATEGORIES[categoryId];
        const articles = learnArticles
          .filter((a) => a.category === categoryId)
          .sort((a, b) => a.order - b.order);

        return (
          <section
            key={categoryId}
            aria-labelledby={`learn-category-${categoryId}`}
            className="mb-10"
          >
            <h2 id={`learn-category-${categoryId}`} className="section-title mb-1">
              {category.label}
            </h2>
            <p className="text-sm text-text-secondary mb-4">{category.blurb}</p>
            <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
              {articles.map((article) => (
                <Link
                  key={article.slug}
                  href={`/learn/${article.slug}`}
                  className="card card-hover p-5 flex flex-col group"
                >
                  <h3 className="font-display text-base font-semibold text-text-primary group-hover:text-accent transition-colors mb-2">
                    {article.title}
                  </h3>
                  <p className="text-sm text-text-secondary leading-relaxed flex-1">
                    {article.description}
                  </p>
                  <p className="text-xs text-text-muted mt-4">
                    {article.readingMinutes} min read
                  </p>
                </Link>
              ))}
            </div>
          </section>
        );
      })}

      <section aria-labelledby="learn-external-resources" className="mb-10">
        <h2 id="learn-external-resources" className="section-title mb-1">
          Further reading
        </h2>
        <p className="text-sm text-text-secondary mb-4">
          Independent sites worth bookmarking once you have the basics down. Both are
          maintained outside ZondScan.
        </p>
        <div className="grid gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {EXTERNAL_RESOURCES.map((resource) => (
            <a
              key={resource.href}
              href={resource.href}
              target="_blank"
              rel="noopener noreferrer"
              className="card card-hover p-5 flex flex-col group"
            >
              <h3 className="font-display text-base font-semibold text-text-primary group-hover:text-accent transition-colors mb-2">
                {resource.title}
              </h3>
              <p className="text-sm text-text-secondary leading-relaxed flex-1">
                {resource.description}
              </p>
              <p className="text-xs text-text-muted mt-4">{resource.domain}</p>
            </a>
          ))}
        </div>
      </section>
    </div>
  );
}
