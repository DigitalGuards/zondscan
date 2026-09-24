'use client';

import { forwardRef, type ComponentPropsWithoutRef } from 'react';
import Link from 'next/link';
import { ArrowUpRightIcon } from '@heroicons/react/24/outline';
import type { NavigationItem } from '../lib/navigation';
import { useTranslation } from './InterfaceText';

const NavigationLink = forwardRef<
  HTMLAnchorElement,
  ComponentPropsWithoutRef<'a'> & {
    item: NavigationItem;
    active: boolean;
  }
>(function NavigationLink({ item, active, className, ...rest }, ref) {
  const { t } = useTranslation();
  const content = (
    <>
      <span className="min-w-0 flex-1">
        <span className="block font-medium">{t(item.name)}</span>
        <span className="mt-0.5 block text-xs font-normal text-text-muted">
          {t(item.description)}
        </span>
      </span>
      {item.external && (
        <ArrowUpRightIcon className="size-3.5 shrink-0 text-text-muted" aria-hidden="true" />
      )}
    </>
  );
  const props = { ...rest, ref, className, 'aria-current': active ? ('page' as const) : undefined };
  return item.external || item.hardNavigation ? (
    <a
      href={item.href}
      target={item.external ? '_blank' : undefined}
      rel={item.external ? 'noopener noreferrer' : undefined}
      {...props}
    >
      {content}
    </a>
  ) : (
    <Link href={item.href} {...props}>
      {content}
    </Link>
  );
});

export default NavigationLink;
