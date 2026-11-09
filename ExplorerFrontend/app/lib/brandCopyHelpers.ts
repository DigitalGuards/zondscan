import { learnArticles } from '../learn/articles';
import { translate } from './i18n';
import { SITE_NAME } from './site';
import { dappDisplayName } from './qrlConnect';

export function articleSummaries(): unknown {
  return learnArticles.map((article) => ({ ...article }));
}

const KEYS = [
  'Explore {name}',
  '{name} home',
  'Choose how {name} looks and how you read the chain. Changes apply immediately.',
] as const;

/** Every translated string that names the site, per language. */
export function translatedSiteStrings(): Record<string, string> {
  const result: Record<string, string> = {};
  for (const locale of ['en', 'zh', 'es', 'ru'] as const) {
    KEYS.forEach((key, index) => {
      result[`${locale}:${index}`] = translate(key, locale, { name: SITE_NAME });
    });
  }
  result.pairingName = dappDisplayName();
  return result;
}
