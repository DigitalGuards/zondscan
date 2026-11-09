import { learnArticles } from '../learn/articles';
import { translate } from './i18n';
import { PAIRING_NAME, SITE_NAME } from './site';

export function articleSummaries(): unknown {
  return learnArticles.map((article) => ({ ...article }));
}

const KEYS = [
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
  result.pairingName = PAIRING_NAME;
  return result;
}
