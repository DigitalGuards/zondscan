import { MOVED_LEARN_ARTICLES, learnRedirects } from './learn-redirects.js';

import nextConfig from './next.config.js';

describe('moved learn articles', () => {
  test('next.config serves every redirect from learn-redirects', async () => {
    expect(await nextConfig.redirects()).toEqual(learnRedirects);
  });

  test.each([
    ['create-a-wallet', 'https://myqrlwallet.com/guides/web-wallet/'],
    ['connect-dapp-wallet', 'https://myqrlwallet.com/guides/connect-dapps/'],
    ['stake-with-quantapool', 'https://myqrlwallet.com/guides/staking/'],
    ['swap-with-quantaswap', 'https://myqrlwallet.com/guides/swaps/'],
  ])('%s redirects permanently to its guide', (slug, destination) => {
    expect(learnRedirects).toContainEqual({
      source: `/learn/${slug}`,
      destination,
      permanent: true,
    });
  });

  test('the app does not set trailingSlash, so Next maps the slash form onto these rules', () => {
    expect(nextConfig.trailingSlash).toBeUndefined();
  });

  test('redirects do not depend on the host', () => {
    for (const rule of learnRedirects) {
      expect(rule).not.toHaveProperty('has');
      expect(rule).not.toHaveProperty('missing');
    }
  });

  test('exactly the four moved articles are covered', () => {
    expect(MOVED_LEARN_ARTICLES.map((a) => a.slug).sort()).toEqual([
      'connect-dapp-wallet',
      'create-a-wallet',
      'stake-with-quantapool',
      'swap-with-quantaswap',
    ]);
    expect(learnRedirects).toHaveLength(4);
  });
});
