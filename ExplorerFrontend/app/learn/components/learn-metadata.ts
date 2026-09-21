// Metadata factory for /learn article pages. Each article page does:
//   export const metadata: Metadata = learnMetadata('<slug>');
// Title, description, and canonical URL all derive from the registry row
// in app/learn/articles.ts, so page files never restate them.

import type { Metadata } from 'next';
import { sharedMetadata } from '../../lib/seo/metaData';
import { getLearnArticle } from '../articles';

export function learnMetadata(slug: string): Metadata {
  const article = getLearnArticle(slug);
  const url = `https://zondscan.com/learn/${article.slug}`;
  const title = `${article.title} | ZondScan Learn`;

  return {
    ...sharedMetadata,
    title,
    description: article.description,
    alternates: {
      ...sharedMetadata.alternates,
      canonical: url,
    },
    openGraph: {
      ...sharedMetadata.openGraph,
      title,
      description: article.description,
      url,
    },
    twitter: {
      ...sharedMetadata.twitter,
      title,
      description: article.description,
    },
  };
}
