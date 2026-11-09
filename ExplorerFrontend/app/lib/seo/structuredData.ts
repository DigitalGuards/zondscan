import { SITE_NAME, SITE_ORIGIN, siteId, siteUrl } from '../site';

// Structured data for search engines. One @graph carrying:
//   - Organization: DigitalGuards, the publisher.
//   - WebSite: the explorer with a SearchAction wired to the real /search
//     redirect route (app/search/route.ts), so Google can surface a
//     sitelinks search box that resolves addresses / tx hashes / blocks.
//   - WebApplication: the explorer itself, with its key sections and
//     related ecosystem apps.
// Serialized once at module scope; rendered with a plain <script> tag
// (the Next.js-documented pattern for JSON-LD in the App Router).
export const structuredData = {
  '@context': 'https://schema.org',
  '@graph': [
    {
      '@type': 'Organization',
      '@id': siteId('organization'),
      name: 'DigitalGuards',
      url: 'https://digitalguards.nl',
      logo: siteUrl('/logo512.png'),
    },
    {
      '@type': 'WebSite',
      '@id': siteId('website'),
      name: SITE_NAME,
      alternateName: 'QRL 2.0 Explorer',
      description:
        'Blockchain explorer for QRL 2.0, the post-quantum blockchain. Track transactions, blocks, smart contracts, and validators.',
      url: SITE_ORIGIN,
      publisher: { '@id': siteId('organization') },
      inLanguage: 'en',
      potentialAction: {
        '@type': 'SearchAction',
        target: {
          '@type': 'EntryPoint',
          urlTemplate: siteUrl('/search?q={search_term_string}'),
        },
        'query-input': 'required name=search_term_string',
      },
    },
    {
      '@type': 'WebApplication',
      '@id': siteId('webapplication'),
      name: SITE_NAME,
      description:
        'Blockchain explorer for QRL 2.0. Track transactions, blocks, smart contracts, and validators on the quantum-resistant proof-of-stake network.',
      url: SITE_ORIGIN,
      applicationCategory: 'Blockchain Explorer',
      operatingSystem: 'All',
      browserRequirements: 'Requires JavaScript',
      publisher: { '@id': siteId('organization') },
      isPartOf: { '@id': siteId('website') },
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
          url: siteUrl('/transactions/1'),
        },
        {
          '@type': 'WebPage',
          name: 'Pending Transactions',
          description: 'View pending mempool transactions',
          url: siteUrl('/pending/1'),
        },
        {
          '@type': 'WebPage',
          name: 'Latest Blocks',
          description: 'View recently synced blocks',
          url: siteUrl('/blocks/1'),
        },
        {
          '@type': 'WebPage',
          name: 'Smart Contracts',
          description: 'View QRL 2.0 smart contracts',
          url: siteUrl('/contracts'),
        },
        {
          '@type': 'WebPage',
          name: 'Validators',
          description: 'Network validators',
          url: siteUrl('/validators'),
        },
        {
          '@type': 'WebPage',
          name: 'Testnet Faucet',
          description: 'Claim QRL 2.0 testnet funds',
          url: siteUrl('/faucet'),
        },
        {
          '@type': 'WebPage',
          name: 'Balance Checker',
          description: 'Check account balances',
          url: siteUrl('/checker'),
        },
        {
          '@type': 'WebPage',
          name: 'Unit Converter',
          description: 'Convert Quanta to Shor',
          url: siteUrl('/converter'),
        },
        {
          '@type': 'WebPage',
          name: 'Richlist',
          description: 'Top QRL 2.0 holders',
          url: siteUrl('/richlist'),
        },
        {
          '@type': 'WebPage',
          name: 'Learn QRL 2.0',
          description: 'Plain English guides to QRL 2.0, wallets, staking, and smart contracts',
          url: siteUrl('/learn'),
        },
        {
          '@type': 'WebPage',
          name: 'FAQ',
          description: `Frequently asked questions about QRL 2.0 and ${SITE_NAME}`,
          url: siteUrl('/faq'),
        },
        {
          '@type': 'WebPage',
          name: 'API Documentation',
          description: `Interactive reference for the ${SITE_NAME} REST API`,
          url: siteUrl('/api-explorer'),
        },
        {
          '@type': 'WebPage',
          name: 'Gas Tracker',
          description: 'Live gas prices and network usage on QRL 2.0',
          url: siteUrl('/gas'),
        },
        {
          '@type': 'WebPage',
          name: 'QRL Order Book',
          description: 'MEXC QRL/USDT market depth, recent trades and fund flow',
          url: siteUrl('/orderbook'),
        },
        {
          '@type': 'WebPage',
          name: 'Verify Contract',
          description: 'Publish verified smart contract source code',
          url: siteUrl('/verify-contract'),
        },
      ],
    },
  ],
};
