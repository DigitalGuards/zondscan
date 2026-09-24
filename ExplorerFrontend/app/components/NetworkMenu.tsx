'use client';

import { useTranslation } from './InterfaceText';

import { Menu, MenuButton, MenuItem, MenuItems } from '@headlessui/react';
import { CheckIcon, GlobeAltIcon } from '@heroicons/react/24/outline';
import { CURRENT_NETWORK_NAME, NETWORK_CONFIG, explorerNetworks } from '../lib/networks';

export default function NetworkMenu() {
  const { t } = useTranslation();
  return (
    <Menu>
      <MenuButton
        className="header-control"
        aria-label={t('Network: {network}', { network: t(CURRENT_NETWORK_NAME) })}
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
        {explorerNetworks(NETWORK_CONFIG).map((network) => {
          const content = (
            <>
              <span className="flex-1 text-left">{t(network.name)}</span>
              {network.current ? (
                <CheckIcon className="size-4 text-accent" aria-hidden="true" />
              ) : (
                !network.href && (
                  <span className="text-[10px] rounded border border-border px-1.5 py-0.5">
                    {t(network.id === 'v3' ? 'Upcoming' : 'Coming later')}
                  </span>
                )
              )}
            </>
          );
          // A full navigation starts a fresh deployment and query cache. Only
          // the destination root is carried across network boundaries.
          return network.href && !network.current ? (
            <MenuItem key={network.id}>
              <a href={network.href} className="menu-option">
                {content}
              </a>
            </MenuItem>
          ) : (
            <MenuItem
              key={network.id}
              as="button"
              type="button"
              disabled={!network.current}
              className="menu-option disabled:cursor-default disabled:text-text-muted"
              aria-current={network.current ? 'true' : undefined}
            >
              {content}
            </MenuItem>
          );
        })}
      </MenuItems>
    </Menu>
  );
}
