import fs from 'node:fs';
import path from 'node:path';
import {
  LEARN_CATEGORY_ORDER,
  getLearnArticle,
  getPrevNext,
  learnArticles,
} from './articles';

const REMOVED_SLUGS = [
  'create-a-wallet',
  'connect-dapp-wallet',
  'stake-with-quantapool',
  'swap-with-quantaswap',
];

describe('learn registry', () => {
  it('has one page directory per article and no extra directories', () => {
    const dirs = fs
      .readdirSync(__dirname, { withFileTypes: true })
      .filter((e) => e.isDirectory() && fs.existsSync(path.join(__dirname, e.name, 'page.tsx')))
      .map((e) => e.name)
      .sort();
    expect(dirs).toEqual(learnArticles.map((a) => a.slug).sort());
  });

  it('has unique slugs and contiguous order within each category', () => {
    const slugs = learnArticles.map((a) => a.slug);
    expect(new Set(slugs).size).toBe(slugs.length);
    for (const category of LEARN_CATEGORY_ORDER) {
      const orders = learnArticles
        .filter((a) => a.category === category)
        .map((a) => a.order)
        .sort((a, b) => a - b);
      expect(orders).toEqual(orders.map((_, i) => i + 1));
    }
  });

  it('no longer lists the articles that moved to the MyQRLWallet guides', () => {
    for (const slug of REMOVED_SLUGS) {
      expect(learnArticles.some((a) => a.slug === slug)).toBe(false);
      expect(() => getLearnArticle(slug)).toThrow();
      expect(getPrevNext(slug)).toEqual({ prev: null, next: null });
    }
  });

  it('keeps the tutorials group on tokens and contracts', () => {
    const tutorials = learnArticles
      .filter((a) => a.category === 'tutorials')
      .sort((a, b) => a.order - b.order)
      .map((a) => a.slug);
    expect(tutorials).toEqual(['launch-a-qrc20', 'deploy-and-verify-contract']);
  });

  it('has no source link left to a removed article path', () => {
    const offenders: string[] = [];
    const walk = (dir: string): void => {
      for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
        const full = path.join(dir, entry.name);
        if (entry.isDirectory()) {
          if (entry.name !== '__fixtures__') walk(full);
        } else if (/\.(tsx?|json)$/.test(entry.name) && !entry.name.includes('.test.')) {
          const text = fs.readFileSync(full, 'utf8');
          for (const slug of REMOVED_SLUGS) {
            if (text.includes(`/learn/${slug}`)) offenders.push(`${full}: ${slug}`);
          }
        }
      }
    };
    walk(path.resolve(__dirname, '..'));
    expect(offenders).toEqual([]);
  });
});
