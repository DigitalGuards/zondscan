'use client';

import { useTranslation } from './InterfaceText';

import { Menu, MenuButton, MenuItem, MenuItems } from '@headlessui/react';
import { CheckIcon, GlobeAltIcon } from '@heroicons/react/24/outline';
import { EXPLORER_NETWORKS } from '../lib/navigation';

export default function NetworkMenu() {
  const { t } = useTranslation();
  return (
    <Menu>
      <MenuButton
        className="header-control"
        aria-label="Network: QRL Testnet v2"
        title={t('Explorer network')}
      >
        <GlobeAltIcon className="size-4" aria-hidden="true" />
      </MenuButton>
      <MenuItems
        anchor={{ to: 'bottom end', gap: 8, padding: 12 }}
        modal={false}
        className="header-menu w-64"
        aria-label={t('Explorer network')}
      >
        <div className="px-3 py-2 text-[11px] uppercase tracking-wider text-text-muted">
          {t('Explorer network')}
        </div>
        {EXPLORER_NETWORKS.map((network) => (
          <MenuItem
            key={network.id}
            as="button"
            type="button"
            disabled={network.status === 'planned'}
            className="menu-option disabled:cursor-default disabled:text-text-muted"
            aria-current={network.status === 'active' ? 'true' : undefined}
          >
            <span className="flex-1 text-left">{t(network.name)}</span>
            {network.status === 'active' ? (
              <CheckIcon className="size-4 text-accent" aria-hidden="true" />
            ) : (
              <span className="text-[10px] rounded border border-border px-1.5 py-0.5">
                {t(network.id === 'testnet-v3' ? 'Upcoming' : 'Coming later')}
              </span>
            )}
          </MenuItem>
        ))}
      </MenuItems>
    </Menu>
  );
}
