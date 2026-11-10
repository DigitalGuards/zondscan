import { SITE_NAME, CANONICAL_ORIGIN } from './lib/site';
import { structuredData } from './lib/seo/structuredData';
import './globals.css'
import SiteHeader from './components/SiteHeader';
import Providers from './providers'
import type { Metadata } from 'next';
import { Chakra_Petch, IBM_Plex_Sans, IBM_Plex_Mono } from 'next/font/google';
import { sharedMetadata } from './lib/seo/metaData';
import Footer from './components/Footer';
import { THEME_BOOTSTRAP } from './lib/preferences';

const displayFont = Chakra_Petch({
  weight: ['500', '600', '700'],
  subsets: ['latin'],
  variable: '--font-zs-display',
  display: 'swap',
});

const sansFont = IBM_Plex_Sans({
  weight: ['400', '500', '600'],
  subsets: ['latin'],
  variable: '--font-zs-sans',
  display: 'swap',
});

const monoFont = IBM_Plex_Mono({
  weight: ['400', '500'],
  subsets: ['latin'],
  variable: '--font-zs-mono',
  display: 'swap',
});

export const metadata: Metadata = {
  ...sharedMetadata,
  title: `${SITE_NAME} | QRL 2.0 Explorer`,
  description:
    'Blockchain explorer for QRL 2.0, an EVM-compatible blockchain secured with post-quantum cryptography. Track transactions, smart contracts, blocks, and validators.',
  alternates: {
    canonical: CANONICAL_ORIGIN,
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: `${SITE_NAME} | QRL 2.0 Explorer`,
    description:
      'Blockchain explorer for QRL 2.0. Track smart contracts, blocks, and transactions on a quantum-resistant EVM-compatible chain.',
    url: CANONICAL_ORIGIN,
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: `${SITE_NAME} | QRL 2.0 Explorer`,
    description:
      'Blockchain explorer for QRL 2.0. Track transactions, blocks, smart contracts, and validators on a post-quantum EVM chain.',
  },


};


export const viewport = {
  themeColor: '#0b0d13',
}


interface RootLayoutProps {
  children: React.ReactNode
}

export default function RootLayout({ children }: RootLayoutProps): JSX.Element {
  return (
    <html lang="en" data-theme="dark" suppressHydrationWarning className={`dark ${displayFont.variable} ${sansFont.variable} ${monoFont.variable}`}>
      <body className="min-h-screen bg-background text-text-secondary">
        <script id="explorer-appearance" dangerouslySetInnerHTML={{ __html: THEME_BOOTSTRAP }} />
        {/* Keep JSON-LD in the rendered body. Next's metadata pipeline owns
            <head>, and mixing a manual head child into that stream can cause
            a React hydration mismatch in development. Search engines accept
            structured data in either document section. */}
        <script
          id="schema-org"
          type="application/ld+json"
          dangerouslySetInnerHTML={{ __html: JSON.stringify(structuredData) }}
        />
        <Providers>
          <a href="#main-content" className="sr-only focus:not-sr-only focus:fixed focus:top-4 focus:left-4 focus:z-[100] focus:px-4 focus:py-2 focus:bg-accent focus:text-background focus:rounded-lg focus:text-sm focus:font-medium">
            Skip to main content
          </a>
          <div className="flex min-h-screen flex-col">
            <SiteHeader />
            {/* min-w-0 is load-bearing: without it this flex item refuses to
                shrink below its content's intrinsic width (flexbox min-width:
                auto), so any wide table or long unbroken hash stretches the
                whole page beyond the mobile viewport instead of scrolling
                inside its own overflow-x-auto container. */}
            <div className="flex-1 min-w-0 relative flex flex-col">
              <main id="main-content" lang="en" className="flex-1 relative">
                {children}
              </main>
              <Footer />
            </div>
          </div>
        </Providers>
      </body>
    </html>
  )
}
