import type { LocalePreference } from './preferences';
import { translations } from './translations';

export const LANGUAGES: { value: LocalePreference; label: string }[] = [
  { value: 'en', label: 'English' },
  { value: 'zh', label: '简体中文' },
  { value: 'es', label: 'Español' },
  { value: 'ru', label: 'Русский' },
];

const canonicalKeys = new Map(Object.keys(translations.zh).map((key) => [key.toLowerCase(), key]));

export function translate(
  text: string,
  locale: LocalePreference,
  values: Record<string, string | number> = {}
): string {
  const dictionary = locale === 'en' ? null : translations[locale];
  const translationKey =
    dictionary && Object.hasOwn(dictionary, text) ? text : canonicalKeys.get(text.toLowerCase());
  const translated =
    dictionary && translationKey && Object.hasOwn(dictionary, translationKey)
      ? dictionary[translationKey]
      : undefined;
  let result = typeof translated === 'string' ? translated : text;
  for (const [key, value] of Object.entries(values))
    result = result.split(`{${key}}`).join(String(value));
  return result;
}
