'use client';

import { useCallback } from 'react';
import { translate } from '../lib/i18n';
import { usePreferences } from './PreferencesProvider';

export function useTranslation() {
  const { preferences } = usePreferences();
  const locale = preferences.locale;
  const t = useCallback(
    (text: string, values?: Record<string, string | number>) => translate(text, locale, values),
    [locale]
  );
  return { t, locale };
}

export default function InterfaceText({ text }: { text: string }) {
  const { t, locale } = useTranslation();
  const translated = t(text);
  return <span lang={translated === text ? 'en' : locale}>{translated}</span>;
}
