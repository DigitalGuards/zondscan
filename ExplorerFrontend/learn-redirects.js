// Learn articles that moved to the MyQRLWallet guides. Each old /learn path
// answers with a permanent redirect to the matching guide. A request with a
// trailing slash first gets Next's own 308 to the slashless path (the app does
// not set trailingSlash), which then lands on the same rule, so both forms end
// at the guide. No `has` host condition: shorscan.com and zondscan.com get the
// same answer.
const MOVED_LEARN_ARTICLES = [
  { slug: 'create-a-wallet', destination: 'https://myqrlwallet.com/guides/web-wallet/' },
  { slug: 'connect-dapp-wallet', destination: 'https://myqrlwallet.com/guides/connect-dapps/' },
  { slug: 'stake-with-quantapool', destination: 'https://myqrlwallet.com/guides/staking/' },
  { slug: 'swap-with-quantaswap', destination: 'https://myqrlwallet.com/guides/swaps/' },
];

const learnRedirects = MOVED_LEARN_ARTICLES.map(({ slug, destination }) => ({
  source: `/learn/${slug}`,
  destination,
  permanent: true,
}));

module.exports = { MOVED_LEARN_ARTICLES, learnRedirects };
