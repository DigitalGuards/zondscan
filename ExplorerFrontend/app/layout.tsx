import './globals.css'
import Sidebar from "./components/Sidebar"
import Providers from './providers'
import type { Metadata } from 'next';
import { Chakra_Petch, IBM_Plex_Sans, IBM_Plex_Mono } from 'next/font/google';
import { sharedMetadata } from './lib/seo/metaData';
import Footer from './components/Footer';

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
  title: 'ZondScan | QRL 2.0 Explorer',
  description:
    'Blockchain explorer for QRL 2.0, an EVM-compatible blockchain secured with post-quantum cryptography. Track transactions, smart contracts, blocks, and validators.',
  alternates: {
    canonical: 'https://zondscan.com',
  },
  openGraph: {
    ...sharedMetadata.openGraph,
    title: 'ZondScan | QRL 2.0 Explorer',
    description:
      'Blockchain explorer for QRL 2.0. Track smart contracts, blocks, and transactions on a quantum-resistant EVM-compatible chain.',
    url: 'https://zondscan.com',
  },
  twitter: {
    ...sharedMetadata.twitter,
    title: 'ZondScan | QRL 2.0 Explorer',
    description:
      'Blockchain explorer for QRL 2.0. Track transactions, blocks, smart contracts, and validators on a post-quantum EVM chain.',
  },


};


export const viewport = {
  themeColor: '#0b0d13',
}

// Structured data for search engines. One @graph carrying:
//   - Organization: DigitalGuards, the publisher.
//   - WebSite: ZondScan with a SearchAction wired to the real /search
//     redirect route (app/search/route.ts), so Google can surface a
//     sitelinks search box that resolves addresses / tx hashes / blocks.
//   - WebApplication: the explorer itself, with its key sections and
//     related ecosystem apps.
// Serialized once at module scope; rendered with a plain <script> tag
// (the Next.js-documented pattern for JSON-LD in the App Router).
const structuredData = {
  '@context': 'https://schema.org',
  '@graph': [
    {
      '@type': 'Organization',
      '@id': 'https://zondscan.com/#organization',
      name: 'DigitalGuards',
      url: 'https://digitalguards.nl',
      logo: 'https://zondscan.com/logo512.png',
    },
    {
      '@type': 'WebSite',
      '@id': 'https://zondscan.com/#website',
      name: 'ZondScan',
      alternateName: 'QRL 2.0 Explorer',
      description:
        'Blockchain explorer for QRL 2.0, the post-quantum blockchain. Track transactions, blocks, smart contracts, and validators.',
      url: 'https://zondscan.com',
      publisher: { '@id': 'https://zondscan.com/#organization' },
      inLanguage: 'en',
      potentialAction: {
        '@type': 'SearchAction',
        target: {
          '@type': 'EntryPoint',
          urlTemplate: 'https://zondscan.com/search?q={search_term_string}',
        },
        'query-input': 'required name=search_term_string',
      },
    },
    {
      '@type': 'WebApplication',
      '@id': 'https://zondscan.com/#webapplication',
      name: 'ZondScan',
      description:
        'Blockchain explorer for QRL 2.0. Track transactions, blocks, smart contracts, and validators on the quantum-resistant proof-of-stake network.',
      url: 'https://zondscan.com',
      applicationCategory: 'Blockchain Explorer',
      operatingSystem: 'All',
      browserRequirements: 'Requires JavaScript',
      publisher: { '@id': 'https://zondscan.com/#organization' },
      isPartOf: { '@id': 'https://zondscan.com/#website' },
      offers: {
        '@type': 'Offer',
        price: '0',
        priceCurrency: 'USD',
      },
      relatedApplication: [
        {
          '@type': 'SoftwareApplication',
          name: 'MyQRLWallet Web Wallet',
          url: 'https://qrlwallet.com',
          applicationCategory: 'Blockchain Wallet',
          operatingSystem: 'All',
        },
        {
          '@type': 'SoftwareApplication',
          name: 'MyQRLWallet',
          url: 'https://myqrlwallet.com',
          applicationCategory: 'Blockchain Wallet',
          operatingSystem: 'All',
        },
        {
          '@type': 'SoftwareApplication',
          name: 'QuantaPool',
          url: 'https://quantapool.com',
          applicationCategory: 'DeFi Staking',
          operatingSystem: 'All',
        },
        {
          '@type': 'SoftwareApplication',
          name: 'QuantaSwap',
          url: 'https://quantaswap.io',
          applicationCategory: 'Cross-chain Swaps',
          operatingSystem: 'All',
        },
      ],
      hasPart: [
        {
          '@type': 'WebPage',
          name: 'Latest Transactions',
          description: 'View recent transactions on QRL 2.0',
          url: 'https://zondscan.com/transactions/1',
        },
        {
          '@type': 'WebPage',
          name: 'Pending Transactions',
          description: 'View pending mempool transactions',
          url: 'https://zondscan.com/pending/1',
        },
        {
          '@type': 'WebPage',
          name: 'Latest Blocks',
          description: 'View recently synced blocks',
          url: 'https://zondscan.com/blocks/1',
        },
        {
          '@type': 'WebPage',
          name: 'Smart Contracts',
          description: 'View QRL 2.0 smart contracts',
          url: 'https://zondscan.com/contracts',
        },
        {
          '@type': 'WebPage',
          name: 'Validators',
          description: 'Network validators',
          url: 'https://zondscan.com/validators',
        },
        {
          '@type': 'WebPage',
          name: 'Testnet Faucet',
          description: 'Claim QRL 2.0 testnet funds',
          url: 'https://zondscan.com/faucet',
        },
        {
          '@type': 'WebPage',
          name: 'Balance Checker',
          description: 'Check account balances',
          url: 'https://zondscan.com/checker',
        },
        {
          '@type': 'WebPage',
          name: 'Unit Converter',
          description: 'Convert Quanta to Shor',
          url: 'https://zondscan.com/converter',
        },
        {
          '@type': 'WebPage',
          name: 'Richlist',
          description: 'Top QRL 2.0 holders',
          url: 'https://zondscan.com/richlist',
        },
        {
          '@type': 'WebPage',
          name: 'Learn QRL 2.0',
          description: 'Plain English guides to QRL 2.0, wallets, staking, and smart contracts',
          url: 'https://zondscan.com/learn',
        },
        {
          '@type': 'WebPage',
          name: 'FAQ',
          description: 'Frequently asked questions about QRL 2.0 and ZondScan',
          url: 'https://zondscan.com/faq',
        },
        {
          '@type': 'WebPage',
          name: 'API Documentation',
          description: 'Interactive reference for the ZondScan REST API',
          url: 'https://zondscan.com/api-explorer',
        },
        {
          '@type': 'WebPage',
          name: 'Gas Tracker',
          description: 'Live gas prices and network usage on QRL 2.0',
          url: 'https://zondscan.com/gas',
        },
        {
          '@type': 'WebPage',
          name: 'Verify Contract',
          description: 'Publish verified smart contract source code',
          url: 'https://zondscan.com/verify-contract',
        },
      ],
    },
  ],
};

interface RootLayoutProps {
  children: React.ReactNode
}

export default function RootLayout({ children }: RootLayoutProps): JSX.Element {
  return (
    <html lang="en" className={`dark ${displayFont.variable} ${sansFont.variable} ${monoFont.variable}`}>
      <head>
        <script
          id="schema-org"
          type="application/ld+json"
          dangerouslySetInnerHTML={{ __html: JSON.stringify(structuredData) }}
        />
      </head>
      <body className="min-h-screen bg-background text-text-secondary">
        <Providers>
          <a href="#main-content" className="sr-only focus:not-sr-only focus:fixed focus:top-4 focus:left-4 focus:z-[100] focus:px-4 focus:py-2 focus:bg-accent focus:text-background focus:rounded-lg focus:text-sm focus:font-medium">
            Skip to main content
          </a>
          <div className="flex min-h-screen">
            <Sidebar />
            {/* min-w-0 is load-bearing: without it this flex item refuses to
                shrink below its content's intrinsic width (flexbox min-width:
                auto), so any wide table or long unbroken hash stretches the
                whole page beyond the mobile viewport instead of scrolling
                inside its own overflow-x-auto container. */}
            <div className="flex-1 min-w-0 lg:ml-64 min-h-screen relative transition-all duration-300 mt-14 lg:mt-4 flex flex-col">
              <main id="main-content" className="flex-1 relative">
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
