import { translate } from './i18n';
import { translations } from './translations';
import { parsePreferences } from './preferences';

it('localizes supported controls and substitutes dates without altering unknown text', () => {
  expect(translate('Site settings', 'es')).toBe('Ajustes del sitio');
  expect(translate('Transactions', 'ru')).not.toBe('Transactions');
  expect(translate('Home', 'zh')).not.toBe('Home');
  expect(translate('Exchange rates updated {date}.', 'es', { date: '2026-09-10' })).toContain(
    '2026-09-10'
  );
  expect(translate('Qabc123', 'zh')).toBe('Qabc123');
  for (const text of ['__proto__', 'constructor', 'toString'])
    expect(translate(text, 'es')).toBe(text);
});
it('keeps translation keys and interpolation placeholders aligned across languages', () => {
  const keys = Object.keys(translations.zh).sort();
  for (const dictionary of Object.values(translations)) {
    expect(Object.keys(dictionary).sort()).toEqual(keys);
    for (const [source, text] of Object.entries(dictionary)) {
      expect(text.trim()).not.toBe('');
      expect((text.match(/\{\w+\}/g) || []).sort()).toEqual(
        (source.match(/\{\w+\}/g) || []).sort()
      );
    }
  }
});
it('accepts all four locales and supported currencies and rejects unrecognized choices', () => {
  for (const locale of ['en', 'zh', 'es', 'ru'])
    expect(parsePreferences(JSON.stringify({ locale, currency: 'EUR' }))).toMatchObject({
      locale,
      currency: 'EUR',
    });
  expect(parsePreferences('{"locale":"invalid","currency":"ZZZ"}')).toMatchObject({
    locale: 'en',
    currency: 'USD',
  });
});
