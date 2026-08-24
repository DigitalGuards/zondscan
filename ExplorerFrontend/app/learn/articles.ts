// Registry for the /learn knowledge base. Every article page under
// app/learn/<slug>/page.tsx has exactly one row here; the row drives the
// index page cards, per-article metadata, JSON-LD, prev/next navigation,
// and the sitemap. Adding an article means adding a row plus a page
// directory; nothing else needs to change.

export type LearnCategory = 'basics' | 'explorer' | 'tutorials';

export interface LearnArticleMeta {
  slug: string;
  title: string;
  description: string;
  category: LearnCategory;
  /** Position within its category, 1-based. */
  order: number;
  /** ISO date (YYYY-MM-DD) of the last substantive content update. */
  updated: string;
  /** Estimated reading time shown on cards and article headers. */
  readingMinutes: number;
}

/** Display order of the categories on the index page and in prev/next. */
export const LEARN_CATEGORY_ORDER: readonly LearnCategory[] = [
  'basics',
  'explorer',
  'tutorials',
];

export const LEARN_CATEGORIES: Record<
  LearnCategory,
  { label: string; blurb: string }
> = {
  basics: {
    label: 'QRL 2.0 basics',
    blurb:
      'The core ideas behind the chain: proof of stake consensus, post-quantum signatures, and the units amounts are counted in.',
  },
  explorer: {
    label: 'Using the explorer',
    blurb:
      'How to read on-chain data on ZondScan: transactions, validators, epochs, and the testnet faucet.',
  },
  tutorials: {
    label: 'Hands-on tutorials',
    blurb:
      'Step by step guides to wallets, staking, swaps, tokens, and smart contracts on the QRL 2.0 testnet.',
  },
};

export const learnArticles: LearnArticleMeta[] = [
  {
    slug: 'what-is-qrl-2',
    title: 'What is QRL 2.0?',
    description:
      'A plain English introduction to QRL 2.0: proof of stake consensus, EVM smart contracts, ML-DSA-87 signatures, and how the new chain relates to legacy QRL.',
    category: 'basics',
    order: 1,
    updated: '2026-08-24',
    readingMinutes: 6,
  },
  {
    slug: 'why-post-quantum',
    title: 'Why QRL is built for the quantum era',
    description:
      "How quantum computers threaten the signatures securing today's blockchains, and how QRL answers with hash based XMSS and lattice based ML-DSA-87.",
    category: 'basics',
    order: 2,
    updated: '2026-08-24',
    readingMinutes: 7,
  },
  {
    slug: 'quanta-shor-planck',
    title: 'Quanta, Shor and Planck: units on QRL 2.0',
    description:
      'What the three units mean, how they convert, and how to read amounts on ZondScan. One Quanta is 10^9 Shor and 10^18 Planck.',
    category: 'basics',
    order: 3,
    updated: '2026-08-05',
    readingMinutes: 4,
  },
  {
    slug: 'read-a-transaction',
    title: 'How to read a transaction on ZondScan',
    description:
      'Status, confirmations, gas, token transfers and internal calls: every field on a ZondScan transaction page explained.',
    category: 'explorer',
    order: 1,
    updated: '2026-08-05',
    readingMinutes: 6,
  },
  {
    slug: 'validators-and-epochs',
    title: 'Validators, epochs and staking on QRL 2.0',
    description:
      'How proof of stake works on QRL 2.0 and how to follow validators, epochs and rewards on ZondScan.',
    category: 'explorer',
    order: 2,
    updated: '2026-08-05',
    readingMinutes: 7,
  },
  {
    slug: 'get-testnet-quanta',
    title: 'Get testnet Quanta from the ZondScan faucet',
    description:
      'Fund a testnet address in under a minute with the built in faucet, then watch the transaction confirm.',
    category: 'explorer',
    order: 3,
    updated: '2026-08-05',
    readingMinutes: 4,
  },
  {
    slug: 'create-a-wallet',
    title: 'Create a QRL wallet with MyQRLWallet',
    description:
      'Set up a post-quantum wallet in the browser, back it up correctly, and open it on desktop and mobile.',
    category: 'tutorials',
    order: 1,
    updated: '2026-08-05',
    readingMinutes: 6,
  },
  {
    slug: 'connect-dapp-wallet',
    title: 'Connect your wallet to a QRL dApp',
    description:
      'Pair MyQRLWallet with a dApp using a QR code or the browser extension, approve requests, and disconnect cleanly.',
    category: 'tutorials',
    order: 2,
    updated: '2026-08-05',
    readingMinutes: 6,
  },
  {
    slug: 'stake-with-quantapool',
    title: 'Liquid staking with QuantaPool',
    description:
      'Stake Quanta through QuantaPool, receive a liquid staking token, and track your position on ZondScan.',
    category: 'tutorials',
    order: 3,
    updated: '2026-08-05',
    readingMinutes: 8,
  },
  {
    slug: 'swap-with-quantaswap',
    title: 'Atomic swaps with QuantaSwap',
    description:
      'Swap between Sepolia ETH and QRL with hash time locked contracts and no custodian in the middle.',
    category: 'tutorials',
    order: 4,
    updated: '2026-08-05',
    readingMinutes: 8,
  },
  {
    slug: 'launch-a-qrc20',
    title: 'Launch a QRC20 token from your wallet',
    description:
      'Create your own token on QRL 2.0 straight from MyQRLWallet and find it on ZondScan.',
    category: 'tutorials',
    order: 5,
    updated: '2026-08-05',
    readingMinutes: 6,
  },
  {
    slug: 'deploy-and-verify-contract',
    title: 'Deploy a smart contract and verify it on ZondScan',
    description:
      'Compile with Hyperion, deploy to the QRL 2.0 testnet, and publish verified source code on ZondScan.',
    category: 'tutorials',
    order: 6,
    updated: '2026-08-05',
    readingMinutes: 8,
  },
];

function categoryRank(category: LearnCategory): number {
  return LEARN_CATEGORY_ORDER.indexOf(category);
}

/** All articles in reading order: category order, then order within it. */
function orderedArticles(): LearnArticleMeta[] {
  return [...learnArticles].sort(
    (a, b) => categoryRank(a.category) - categoryRank(b.category) || a.order - b.order,
  );
}

/**
 * Look up one article by slug. Throws on an unknown slug so a typo in a
 * page file fails the build. A silent fallback would render an empty header.
 */
export function getLearnArticle(slug: string): LearnArticleMeta {
  const article = learnArticles.find((a) => a.slug === slug);
  if (!article) {
    throw new Error(`Unknown learn article slug: ${slug}`);
  }
  return article;
}

/** Neighbors of an article in reading order, for prev/next cards. */
export function getPrevNext(slug: string): {
  prev: LearnArticleMeta | null;
  next: LearnArticleMeta | null;
} {
  const ordered = orderedArticles();
  const index = ordered.findIndex((a) => a.slug === slug);
  if (index === -1) {
    return { prev: null, next: null };
  }
  return {
    prev: index > 0 ? ordered[index - 1] : null,
    next: index < ordered.length - 1 ? ordered[index + 1] : null,
  };
}
