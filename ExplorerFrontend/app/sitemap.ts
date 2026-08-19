import type { MetadataRoute } from 'next';
import { learnArticles } from './learn/articles';

/**
 * Sitemap for ZondScan. Lists the static routes that have meaningful
 * standalone landing pages so search crawlers index the site map
 * (not every block / tx / address; those are uncacheable per-instance
 * pages that crawlers reach via internal links). Next.js App Router
 * serves this at /sitemap.xml.
 *
 * Adding a new public page? Add a row below; lastModified defaults to
 * the deploy time (build evaluation moment).
 */

const BASE = 'https://zondscan.com';

export default function sitemap(): MetadataRoute.Sitemap {
  const now = new Date();
  const entry = (
    path: string,
    changeFrequency: MetadataRoute.Sitemap[number]['changeFrequency'],
    priority: number,
  ): MetadataRoute.Sitemap[number] => ({
    url: `${BASE}${path}`,
    lastModified: now,
    changeFrequency,
    priority,
  });

  return [
    entry('/', 'hourly', 1.0),
    entry('/transactions', 'always', 0.9),
    entry('/blocks/1', 'always', 0.9),
    entry('/contracts', 'daily', 0.8),
    entry('/validators', 'hourly', 0.8),
    entry('/pending/1', 'always', 0.7),
    entry('/epochs/1', 'hourly', 0.7),
    entry('/gas', 'hourly', 0.7),
    entry('/orderbook', 'always', 0.7),
    entry('/learn', 'weekly', 0.7),
    entry('/richlist', 'daily', 0.6),
    entry('/faucet', 'monthly', 0.6),
    entry('/converter', 'yearly', 0.5),
    entry('/api-explorer', 'monthly', 0.5),
    entry('/verify-contract', 'monthly', 0.5),
    entry('/checker', 'monthly', 0.4),
    entry('/faq', 'monthly', 0.4),
    // Static bundle from the @qrlwallet/connect repo, served from
    // public/dapp-example/ (see next.config.js rewrite).
    entry('/dapp-example', 'monthly', 0.4),
    // Learn articles: one row per registry entry; the registry
    // (app/learn/articles.ts) is the source of truth for slugs.
    ...learnArticles.map((article) => entry(`/learn/${article.slug}`, 'monthly', 0.6)),
  ];
}
