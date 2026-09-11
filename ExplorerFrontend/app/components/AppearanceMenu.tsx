'use client';

import { useTranslation } from './InterfaceText';

import { Menu, MenuButton, MenuItem, MenuItems } from '@headlessui/react';
import {
  CheckIcon,
  ComputerDesktopIcon,
  MoonIcon,
  SunIcon,
  CloudIcon,
} from '@heroicons/react/24/outline';
import type { ThemePreference } from '../lib/preferences';
import { usePreferences } from './PreferencesProvider';

export const APPEARANCES = [
  { value: 'light', label: 'Light', icon: SunIcon },
  { value: 'dim', label: 'Dim', icon: CloudIcon },
  { value: 'dark', label: 'Dark', icon: MoonIcon },
  { value: 'system', label: 'Auto (system)', icon: ComputerDesktopIcon },
] as const;

export default function AppearanceMenu() {
  const { t } = useTranslation();
  const { preferences, updatePreferences } = usePreferences();
  const selected = APPEARANCES.find((item) => item.value === preferences.theme)!;
  const Icon = selected.icon;
  return (
    <Menu>
      <MenuButton
        className="header-control"
        aria-label={`${t('Appearance')}: ${t(selected.label)}`}
        title={t('Appearance')}
      >
        <Icon className="size-4" aria-hidden="true" />
      </MenuButton>
      <MenuItems
        anchor={{ to: 'bottom end', gap: 8, padding: 12 }}
        modal={false}
        className="header-menu w-52"
        aria-label={t('Appearance')}
      >
        <div className="px-3 py-2 text-[11px] uppercase tracking-wider text-text-muted">
          {t('Appearance')}
        </div>
        {APPEARANCES.map((item) => (
          <MenuItem
            key={item.value}
            as="button"
            type="button"
            aria-label={`${t(item.label)}${preferences.theme === item.value ? ' (current)' : ''}`}
            onClick={() => updatePreferences({ theme: item.value as ThemePreference })}
            className="menu-option"
          >
            <item.icon className="size-4 text-text-muted" aria-hidden="true" />
            <span className="flex-1 text-left">{t(item.label)}</span>
            {preferences.theme === item.value && (
              <CheckIcon className="size-4 text-accent" aria-hidden="true" />
            )}
          </MenuItem>
        ))}
      </MenuItems>
    </Menu>
  );
}
