'use client';

import { useState } from 'react';
import { useTranslation } from '../components/InterfaceText';
import { LANGUAGES } from '../lib/i18n';
import { CURRENCIES } from '../lib/currency';
import { useDisplayCurrency } from '../components/useDisplayCurrency';
import { ArrowPathIcon, CheckIcon } from '@heroicons/react/24/outline';
import { APPEARANCES } from '../components/AppearanceMenu';
import { usePreferences } from '../components/PreferencesProvider';
import { displayAddress, formatDateTime, type ExplorerPreferences } from '../lib/preferences';

function ToggleRow({
  id,
  title,
  description,
  checked,
  onChange,
  disabled,
}: {
  id: string;
  title: string;
  description: string;
  checked: boolean;
  onChange: (value: boolean) => void;
  disabled: boolean;
}) {
  return (
    <div className="flex items-center justify-between gap-6 py-5">
      <div>
        <p id={`${id}-title`} className="text-sm font-medium text-text-primary">
          {title}
        </p>
        <p
          id={`${id}-description`}
          className="mt-1 max-w-xl text-sm leading-relaxed text-text-muted"
        >
          {description}
        </p>
      </div>
      <button
        type="button"
        role="switch"
        disabled={disabled}
        aria-checked={checked}
        data-checked={checked ? '' : undefined}
        onClick={() => onChange(!checked)}
        aria-labelledby={`${id}-title`}
        aria-describedby={`${id}-description`}
        className="relative inline-flex h-6 w-11 shrink-0 items-center rounded-full border border-border bg-surface-3 transition-colors data-checked:bg-accent focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-accent"
      >
        <span
          className={`inline-block size-4.5 rounded-full bg-white shadow-sm transition-transform ${checked ? 'translate-x-5.5' : 'translate-x-0.5'}`}
        />
      </button>
    </div>
  );
}

const exampleAddress = `Q${'0123456789abcdef'.repeat(8)}`;
const exampleTimestamp = Date.UTC(2026, 8, 10, 12, 30, 0) / 1000;

export default function SettingsClient() {
  const { t, locale } = useTranslation();
  const displayCurrency = useDisplayCurrency();
  const { preferences, updatePreferences, resetPreferences, ready, persistent } = usePreferences();
  const [message, setMessage] = useState('');
  const save = (changes: Partial<ExplorerPreferences>) => {
    const saved = updatePreferences(changes);
    setMessage(
      saved ? 'Preferences saved.' : 'Applied for this tab. Browser storage is unavailable.'
    );
  };
  const localZone = ready ? Intl.DateTimeFormat().resolvedOptions().timeZone : 'your device';

  return (
    <div lang={locale} className="mx-auto max-w-screen-xl px-4 py-8 sm:px-6 lg:px-8 lg:py-12">
      <div className="mb-9 flex flex-wrap items-start justify-between gap-5">
        <div>
          <p className="mb-2 text-[11px] font-medium uppercase tracking-[0.16em] text-accent">
            {t('Your explorer')}
          </p>
          <h1 className="font-display text-3xl font-semibold tracking-tight text-text-primary">
            {t('Site settings')}
          </h1>
          <p className="mt-3 max-w-2xl text-sm leading-relaxed text-text-secondary">
            {t('Choose how ZondScan looks and how you read the chain. Changes apply immediately.')}
          </p>
        </div>
        <button
          type="button"
          onClick={() => {
            const saved = resetPreferences();
            setMessage(
              saved
                ? 'Default preferences restored.'
                : 'Defaults restored for this tab. Browser storage is unavailable.'
            );
          }}
          disabled={!ready}
          className="btn-secondary inline-flex items-center gap-2 text-xs"
        >
          <ArrowPathIcon className="size-4" aria-hidden="true" />
          {t('Reset defaults')}
        </button>
      </div>
      <div className="grid gap-8 lg:grid-cols-[200px_minmax(0,1fr)]">
        <aside aria-label={t('Settings sections')} className="self-start">
          <nav className="flex flex-wrap gap-2 lg:flex-col">
            {['Appearance', 'Language & currency', 'Reading & display', 'Transactions'].map(
              (label, index) => (
                <a
                  key={t(label)}
                  href={['#appearance', '#localization', '#display', '#transactions'][index]}
                  className="rounded-lg px-3 py-2 text-sm text-text-secondary hover:bg-surface-2 hover:text-accent"
                >
                  {t(label)}
                </a>
              )
            )}
          </nav>
          <p className="mt-5 hidden px-3 text-xs leading-relaxed text-text-muted lg:block">
            {t('Your preferences are saved in this browser.')}
          </p>
        </aside>
        <div className="min-w-0 space-y-6">
          <section
            id="appearance"
            aria-labelledby="appearance-heading"
            className="settings-card scroll-mt-4"
          >
            <h2
              id="appearance-heading"
              className="font-display text-lg font-semibold text-text-primary"
            >
              {t('Appearance')}
            </h2>
            <p className="mt-1 text-sm text-text-muted">
              {t('Find a comfortable view for every time of day.')}
            </p>
            <fieldset className="mt-6">
              <legend className="sr-only">{t('Choose appearance')}</legend>
              <div className="grid grid-cols-2 gap-3 sm:grid-cols-4">
                {APPEARANCES.map((item) => (
                  <label key={item.value} className="cursor-pointer">
                    <input
                      type="radio"
                      disabled={!ready}
                      name="appearance"
                      value={item.value}
                      checked={preferences.theme === item.value}
                      onChange={() => save({ theme: item.value })}
                      className="peer sr-only"
                    />
                    <span className="block overflow-hidden rounded-xl border border-border p-2.5 transition-colors peer-checked:border-accent peer-checked:ring-1 peer-checked:ring-accent peer-focus-visible:outline-2 peer-focus-visible:outline-offset-4 peer-focus-visible:outline-accent">
                      <span
                        className={`theme-preview theme-preview-${item.value}`}
                        aria-hidden="true"
                      >
                        <span className="theme-preview-bar" />
                        <span className="theme-preview-body">
                          <span />
                          <span />
                          <span />
                        </span>
                      </span>
                      <span className="mt-3 flex items-center justify-between gap-1 text-xs text-text-primary">
                        {t(item.label)}
                        {preferences.theme === item.value && (
                          <CheckIcon className="size-3.5 text-accent" aria-hidden="true" />
                        )}
                      </span>
                    </span>
                  </label>
                ))}
              </div>
            </fieldset>
            <p className="mt-4 text-xs text-text-muted">
              {t("Auto follows your device's light or dark appearance.")}
            </p>
          </section>

          <section
            id="localization"
            aria-labelledby="localization-heading"
            className="settings-card scroll-mt-4"
          >
            <h2
              id="localization-heading"
              className="font-display text-lg font-semibold text-text-primary"
            >
              {t('Language & currency')}
            </h2>
            <div className="mt-2 divide-y divide-border">
              <div className="grid items-center gap-4 py-5 sm:grid-cols-[minmax(0,1fr)_auto]">
                <div>
                  <label
                    htmlFor="interface-language"
                    className="text-sm font-medium text-text-primary"
                  >
                    {t('Language')}
                  </label>
                  <p className="mt-1 max-w-xl text-sm text-text-muted">
                    {t(
                      'Explorer controls and dates follow your language. Guides and on-chain content keep their original language.'
                    )}
                  </p>
                </div>
                <select
                  disabled={!ready}
                  id="interface-language"
                  value={locale}
                  onChange={(event) =>
                    save({ locale: event.target.value as ExplorerPreferences['locale'] })
                  }
                  className="form-select w-full sm:w-44"
                >
                  {LANGUAGES.map((language) => (
                    <option key={language.value} value={language.value} lang={language.value}>
                      {language.label}
                    </option>
                  ))}
                </select>
              </div>
              <div className="grid items-center gap-4 py-5 sm:grid-cols-[minmax(0,1fr)_auto]">
                <div>
                  <label
                    htmlFor="display-currency"
                    className="text-sm font-medium text-text-primary"
                  >
                    {t('Currency')}
                  </label>
                  <p className="mt-1 max-w-xl text-sm text-text-muted">
                    {t(
                      'Use your preferred currency for market prices and estimates. Trading pairs keep their quote currency.'
                    )}
                  </p>
                  <p className="mt-2 text-xs text-text-muted" role="status">
                    {t(
                      displayCurrency.status === 'loading'
                        ? 'Exchange rates are loading. Estimates currently use USD.'
                        : displayCurrency.status === 'unavailable'
                          ? 'Exchange rates are unavailable. Estimates currently use USD.'
                          : displayCurrency.rateDate
                            ? 'Exchange rates updated {date}.'
                            : 'Market prices use USD.',
                      { date: displayCurrency.rateDate || '' }
                    )}
                  </p>
                </div>
                <select
                  disabled={!ready}
                  id="display-currency"
                  value={preferences.currency}
                  onChange={(event) =>
                    save({ currency: event.target.value as ExplorerPreferences['currency'] })
                  }
                  className="form-select w-full sm:w-64 lg:w-72"
                >
                  {CURRENCIES.map((currency) => (
                    <option key={currency.code} value={currency.code}>
                      {currency.code} -{' '}
                      {new Intl.DisplayNames([locale], { type: 'currency' }).of(currency.code)}
                    </option>
                  ))}
                </select>
              </div>
            </div>
          </section>

          <section
            id="display"
            aria-labelledby="display-heading"
            className="settings-card scroll-mt-4"
          >
            <h2
              id="display-heading"
              className="font-display text-lg font-semibold text-text-primary"
            >
              {t('Reading & display')}
            </h2>
            <div className="mt-2 divide-y divide-border">
              <div className="grid items-center gap-4 py-5 sm:grid-cols-[minmax(0,1fr)_auto]">
                <div>
                  <label
                    htmlFor="address-display"
                    className="text-sm font-medium text-text-primary"
                  >
                    {t('Address display')}
                  </label>
                  <p className="mt-1 text-sm text-text-muted">
                    {t('Choose which part of a shortened address stays visible.')}
                  </p>
                  <p
                    className="mt-3 font-mono text-xs text-accent"
                    aria-label={t('Address display example')}
                  >
                    {displayAddress(exampleAddress, preferences.addressDisplay)}
                  </p>
                </div>
                <select
                  disabled={!ready}
                  id="address-display"
                  value={preferences.addressDisplay}
                  onChange={(event) =>
                    save({
                      addressDisplay: event.target.value as ExplorerPreferences['addressDisplay'],
                    })
                  }
                  className="form-select w-full sm:w-44"
                >
                  <option value="middle">{t('Middle truncation')}</option>
                  <option value="back">{t('End truncation')}</option>
                </select>
              </div>
              <div className="grid items-center gap-4 py-5 sm:grid-cols-[minmax(0,1fr)_auto]">
                <div>
                  <label htmlFor="time-zone" className="text-sm font-medium text-text-primary">
                    {t('Date and time')}
                  </label>
                  <p className="mt-1 text-sm text-text-muted">
                    {t('Use UTC or your local time zone ({zone}).', { zone: localZone })}
                  </p>
                  <p className="mt-3 text-xs text-accent" aria-label={t('Date and time example')}>
                    {formatDateTime(exampleTimestamp, preferences.timeZone, locale)}
                  </p>
                </div>
                <select
                  disabled={!ready}
                  id="time-zone"
                  value={preferences.timeZone}
                  onChange={(event) =>
                    save({ timeZone: event.target.value as ExplorerPreferences['timeZone'] })
                  }
                  className="form-select w-full sm:w-44"
                >
                  <option value="utc">{t('UTC')}</option>
                  <option value="local">{t('Local time')}</option>
                </select>
              </div>
              <ToggleRow
                disabled={!ready}
                id="highlight-addresses"
                title={t('Highlight matching addresses')}
                description={t(
                  'Highlight the same full address across a view when you hover over or focus a shortened address.'
                )}
                checked={preferences.highlightAddresses}
                onChange={(highlightAddresses) => save({ highlightAddresses })}
              />
            </div>
          </section>

          <section
            id="transactions"
            aria-labelledby="transactions-heading"
            className="settings-card scroll-mt-4"
          >
            <h2
              id="transactions-heading"
              className="font-display text-lg font-semibold text-text-primary"
            >
              {t('Transactions')}
            </h2>
            <div className="mt-2 divide-y divide-border">
              <ToggleRow
                disabled={!ready}
                id="expand-details"
                title={t('Expand technical details')}
                description={t(
                  'Open collapsible transaction and block data by default. You can still close individual panels.'
                )}
                checked={preferences.expandDetails}
                onChange={(expandDetails) => save({ expandDetails })}
              />
              <ToggleRow
                disabled={!ready}
                id="hide-zero-transfers"
                title={t('Hide zero-quantity token transfers')}
                description={t(
                  'Filter QRC-20 transfers with an amount of zero. Native transactions and NFT transfers stay visible.'
                )}
                checked={preferences.hideZeroTokenTransfers}
                onChange={(hideZeroTokenTransfers) => save({ hideZeroTokenTransfers })}
              />
            </div>
          </section>
          <div className="flex flex-wrap justify-between gap-3 text-xs text-text-muted">
            <p>
              {t(
                persistent
                  ? 'Preferences are saved in this browser.'
                  : 'Browser storage is unavailable. Preferences apply to this tab.'
              )}
            </p>
            <p role="status" className="text-accent">
              {t(message)}
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
