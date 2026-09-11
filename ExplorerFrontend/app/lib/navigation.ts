export interface NavigationItem {
  name: string;
  href: string;
  description: string;
  external?: boolean;
  hardNavigation?: boolean;
}
export interface NavigationGroup {
  name: string;
  items: NavigationItem[];
}

export const NAVIGATION_GROUPS: NavigationGroup[] = [
  {
    name: 'Blockchain',
    items: [
      {
        name: 'Transactions',
        href: '/transactions/1',
        description: 'Latest transfers and contract calls',
      },
      {
        name: 'Pending transactions',
        href: '/pending/1',
        description: 'Transactions awaiting inclusion',
      },
      { name: 'Blocks', href: '/blocks/1', description: 'Explore the execution chain' },
      { name: 'Smart contracts', href: '/contracts', description: 'Browse deployed contracts' },
      { name: 'Gas tracker', href: '/gas', description: 'Network fees and activity' },
      { name: 'Rich list', href: '/richlist', description: 'Addresses ranked by balance' },
    ],
  },
  {
    name: 'Consensus',
    items: [
      { name: 'Epochs', href: '/epochs/1', description: 'Slots, proposals, and participation' },
      { name: 'Validators', href: '/validators', description: 'The validators securing QRL 2.0' },
    ],
  },
  {
    name: 'Tools',
    items: [
      {
        name: 'Order Book Arena',
        href: '/orderbook',
        description: 'QRL markets and trading activity',
      },
      { name: 'Balance checker', href: '/checker', description: 'Look up an address balance' },
      { name: 'Testnet faucet', href: '/faucet', description: 'Get Quanta for testnet' },
      {
        name: 'Staking calculator',
        href: '/staking-calculator',
        description: 'Explore validator rewards',
      },
      {
        name: 'Unit converter',
        href: '/converter',
        description: 'Convert Quanta, Shor, and Planck',
      },
      {
        name: 'Verify a contract',
        href: '/verify-contract',
        description: 'Publish verified source code',
      },
    ],
  },
  {
    name: 'Resources',
    items: [
      { name: 'Learn', href: '/learn', description: 'Guides to QRL 2.0 and the explorer' },
      { name: 'FAQ', href: '/faq', description: 'Answers to common questions' },
      {
        name: 'API explorer',
        href: '/api-explorer',
        description: 'Documentation and API playground',
      },
      { name: 'Site settings', href: '/settings', description: 'Make the explorer work your way' },
    ],
  },
  {
    name: 'Ecosystem',
    items: [
      {
        name: 'MyQRLWallet',
        href: 'https://myqrlwallet.com',
        description: 'Wallets and tools for QRL',
        external: true,
      },
      {
        name: 'QRL documentation',
        href: 'https://docs.theqrl.org',
        description: 'Explore the QRL protocol',
        external: true,
      },
      {
        name: 'dApp example',
        href: '/dapp-example/',
        description: 'Try connecting a QRL wallet',
        hardNavigation: true,
      },
    ],
  },
];

export function isNavigationActive(pathname: string, href: string): boolean {
  if (pathname === href) return true;
  const base = href.replace(/\/\d+$/, '');
  if (base !== href && pathname.startsWith(`${base}/`)) return true;
  if (href === '/blocks/1' && pathname.startsWith('/block/')) return true;
  if (href === '/transactions/1' && pathname.startsWith('/tx/')) return true;
  if (href === '/epochs/1' && pathname.startsWith('/epoch/')) return true;
  if (href === '/validators' && pathname.startsWith('/validators/')) return true;
  return ['/learn', '/contracts', '/pending/1'].includes(href) && pathname.startsWith(`${base}/`);
}

export const EXPLORER_NETWORKS = [
  { id: 'testnet-v2', name: 'QRL Testnet v2', status: 'active' },
  { id: 'testnet-v3', name: 'QRL Testnet v3', status: 'planned' },
  { id: 'mainnet', name: 'QRL Mainnet', status: 'planned' },
] as const;
