'use client';

import { useId } from 'react';
import Link from 'next/link';
import { Disclosure, DisclosureButton, DisclosurePanel } from '@headlessui/react';
import {
  CheckIcon,
  ChevronDownIcon,
  Cog6ToothIcon,
  GlobeAltIcon,
} from '@heroicons/react/24/outline';
import { EXPLORER_NETWORKS, NAVIGATION_GROUPS, isNavigationActive } from '../lib/navigation';
import { APPEARANCES } from './AppearanceMenu';
import { useTranslation } from './InterfaceText';
import NavigationLink from './NavigationLink';
import { usePreferences } from './PreferencesProvider';

export interface MobileNavigationProps {
  pathname: string;
  onNavigate: () => void;
}

const MOBILE_GROUPS = NAVIGATION_GROUPS.map((group) => ({
  ...group,
  items: group.items.filter((item) => item.href !== '/settings'),
}));

const rowClassName =
  'flex min-h-11 w-full items-center gap-3 rounded-md px-3 py-2.5 text-left hover:bg-surface focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-accent';
const disclosureClassName = `${rowClassName} group justify-between font-medium text-text-primary`;
const panelClassName = 'mb-1 ml-3 border-l border-border pl-2';

// Render below the brand row so the menu expands with the page on narrow screens.
export default function MobileNavigation({ pathname, onNavigate }: MobileNavigationProps) {
  const { t } = useTranslation();
  const { preferences, updatePreferences } = usePreferences();
  const appearanceName = useId();
  const selectedAppearance = APPEARANCES.find((item) => item.value === preferences.theme)!;
  const AppearanceIcon = selectedAppearance.icon;

  return (
    <nav
      aria-label={t('Mobile navigation')}
      className="mx-auto max-w-screen-2xl px-1 pb-3 pt-1 text-base sm:px-3"
    >
      <Link
        href="/"
        onClick={onNavigate}
        className={`${rowClassName} font-medium ${pathname === '/' ? 'text-accent' : 'text-text-primary'}`}
        aria-current={pathname === '/' ? 'page' : undefined}
      >
        {t('Home')}
      </Link>
      {MOBILE_GROUPS.map((group) => (
        <Disclosure
          key={`${pathname}:${group.name}`}
          defaultOpen={group.items.some((item) => isNavigationActive(pathname, item.href))}
        >
          <DisclosureButton
            className={`${disclosureClassName} ${group.items.some((item) => isNavigationActive(pathname, item.href)) ? 'text-accent' : ''}`}
          >
            <span>{t(group.name)}</span>
            <ChevronDownIcon
              className="size-4 shrink-0 group-data-open:rotate-180"
              aria-hidden="true"
            />
          </DisclosureButton>
          <DisclosurePanel className={panelClassName}>
            {group.items.map((item) => (
              <NavigationLink
                key={item.href}
                item={item}
                active={isNavigationActive(pathname, item.href)}
                onClick={onNavigate}
                className={`${rowClassName} text-sm [&>span>span+span]:hidden ${isNavigationActive(pathname, item.href) ? 'text-accent' : 'text-text-secondary'}`}
              />
            ))}
          </DisclosurePanel>
        </Disclosure>
      ))}
      <div className="mt-2 border-t border-border pt-2">
        <Link
          href="/settings"
          onClick={onNavigate}
          className={`${rowClassName} font-medium ${pathname === '/settings' ? 'text-accent' : 'text-text-primary'}`}
          aria-current={pathname === '/settings' ? 'page' : undefined}
        >
          <Cog6ToothIcon className="size-4 shrink-0" aria-hidden="true" />
          {t('Site settings')}
        </Link>
        <Disclosure>
          <DisclosureButton className={disclosureClassName}>
            <span className="flex items-center gap-3">
              <AppearanceIcon className="size-4 shrink-0" aria-hidden="true" />
              {t('Appearance')}
            </span>
            <ChevronDownIcon
              className="size-4 shrink-0 group-data-open:rotate-180"
              aria-hidden="true"
            />
          </DisclosureButton>
          <DisclosurePanel className={panelClassName}>
            <fieldset>
              <legend className="sr-only">{t('Appearance')}</legend>
              {APPEARANCES.map((item) => (
                <label
                  key={item.value}
                  className={`${rowClassName} cursor-pointer text-text-secondary has-focus-visible:outline-2 has-focus-visible:outline-offset-2 has-focus-visible:outline-accent`}
                >
                  <item.icon className="size-4 shrink-0 text-text-muted" aria-hidden="true" />
                  <span className="flex-1">{t(item.label)}</span>
                  <input
                    type="radio"
                    name={appearanceName}
                    value={item.value}
                    checked={preferences.theme === item.value}
                    onChange={() => updatePreferences({ theme: item.value })}
                    className="size-4 shrink-0 accent-accent"
                  />
                </label>
              ))}
            </fieldset>
          </DisclosurePanel>
        </Disclosure>
        <Disclosure>
          <DisclosureButton className={disclosureClassName}>
            <span className="flex items-center gap-3">
              <GlobeAltIcon className="size-4 shrink-0" aria-hidden="true" />
              {t('Explorer network')}
            </span>
            <ChevronDownIcon
              className="size-4 shrink-0 group-data-open:rotate-180"
              aria-hidden="true"
            />
          </DisclosureButton>
          <DisclosurePanel className={panelClassName}>
            {EXPLORER_NETWORKS.map((network) => {
              const content = (
                <>
                  <span className="min-w-0 flex-1">{t(network.name)}</span>
                  {network.status === 'active' ? (
                    <CheckIcon className="size-4 shrink-0 text-accent" aria-hidden="true" />
                  ) : (
                    <span className="shrink-0 rounded border border-border px-1.5 py-0.5 text-[10px]">
                      {t(network.id === 'testnet-v3' ? 'Upcoming' : 'Coming later')}
                    </span>
                  )}
                </>
              );

              if (network.status === 'active') {
                return (
                  <span
                    key={network.id}
                    aria-current="true"
                    className={`${rowClassName} text-accent`}
                  >
                    {content}
                  </span>
                );
              }

              return (
                <button
                  key={network.id}
                  type="button"
                  disabled
                  className={`${rowClassName} cursor-default text-text-muted disabled:hover:bg-transparent`}
                >
                  {content}
                </button>
              );
            })}
          </DisclosurePanel>
        </Disclosure>
      </div>
    </nav>
  );
}
