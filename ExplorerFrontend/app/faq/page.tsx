import type { Metadata } from 'next';
import Link from 'next/link';
import { sharedMetadata } from '../lib/seo/metaData';
import { faqs } from './faq-data';

export const metadata: Metadata = {
  ...sharedMetadata,
  title: 'FAQ | ZondScan',
  description:
    'Answers to frequently asked questions about ZondScan, QRL 2.0, transactions, smart contracts, wallets, and the open API.',
  alternates: {
    ...sharedMetadata.alternates,
    canonical: 'https://zondscan.com/faq',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'FAQ | ZondScan',
    description:
      'Answers to frequently asked questions about ZondScan, QRL 2.0, transactions, smart contracts, wallets, and the open API.',
    url: 'https://zondscan.com/faq',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'FAQ | ZondScan',
    description:
      'Answers to frequently asked questions about ZondScan, QRL 2.0, transactions, smart contracts, wallets, and the open API.',
  },
};

export default function FAQPage(): JSX.Element {
  const faqJsonLd = {
    '@context': 'https://schema.org',
    '@type': 'FAQPage',
    mainEntity: faqs.map((item) => ({
      '@type': 'Question',
      name: item.q,
      acceptedAnswer: {
        '@type': 'Answer',
        text: item.aText,
      },
    })),
  };

  return (
    <div aria-labelledby="faq-heading">
      <h1 id="faq-heading" className="sr-only">
        ZondScan FAQ
      </h1>
      <script type="application/ld+json">{JSON.stringify(faqJsonLd)}</script>
      <div className="min-h-screen text-text-secondary px-4 sm:px-6 py-4 md:py-8">
        <div className="max-w-3xl mx-auto">
          <h2 className="section-title mb-8">Frequently asked questions</h2>

          <div className="space-y-4">
            {faqs.map((faq) => (
              <details
                key={faq.id}
                id={faq.id}
                className="card-simple overflow-hidden group"
              >
                <summary className="flex w-full cursor-pointer list-none items-center justify-between gap-3 px-4 py-3 text-left [&::-webkit-details-marker]:hidden">
                  <span className="text-sm md:text-base font-medium text-text-primary">
                    {faq.q}
                  </span>
                  <svg
                    aria-hidden="true"
                    viewBox="0 0 20 20"
                    fill="currentColor"
                    className="h-5 w-5 shrink-0 text-accent transition-transform duration-200 group-open:rotate-180"
                  >
                    <path
                      fillRule="evenodd"
                      d="M5.22 8.22a.75.75 0 0 1 1.06 0L10 11.94l3.72-3.72a.75.75 0 1 1 1.06 1.06l-4.25 4.25a.75.75 0 0 1-1.06 0L5.22 9.28a.75.75 0 0 1 0-1.06Z"
                      clipRule="evenodd"
                    />
                  </svg>
                </summary>
                <div className="px-4 py-3 text-sm leading-relaxed text-text-secondary bg-background-tertiary/50 border-t border-border">
                  {faq.a}
                </div>
              </details>
            ))}
          </div>

          <p className="mt-8 text-sm text-text-secondary">
            Looking for more depth? The{' '}
            <Link
              href="/learn"
              className="text-accent hover:text-accent-hover underline decoration-accent/40 underline-offset-2 transition-colors"
            >
              Learn section
            </Link>{' '}
            has step by step guides covering wallets, staking, swaps, tokens,
            and contract development.
          </p>
        </div>
      </div>
    </div>
  );
}
