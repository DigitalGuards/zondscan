'use client';

import { useTranslation } from './InterfaceText';

import {
  useEffect,
  useId,
  useLayoutEffect,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
  type PointerEvent as ReactPointerEvent,
} from 'react';
import Link from 'next/link';
import Image from 'next/image';
import { usePathname } from 'next/navigation';
import { Bars3Icon, ChevronDownIcon, Cog6ToothIcon, XMarkIcon } from '@heroicons/react/24/outline';
import { NAVIGATION_GROUPS, isNavigationActive } from '../lib/navigation';
import AppearanceMenu from './AppearanceMenu';
import NetworkMenu from './NetworkMenu';
import MarketSummary from './MarketSummary';
import SearchBar from './SearchBar';
import NavigationLink from './NavigationLink';
import MobileNavigation from './MobileNavigation';

interface DesktopMenuProps {
  group: (typeof NAVIGATION_GROUPS)[number];
  pathname: string;
  open: boolean;
  onOpen: () => void;
  onClose: () => void;
}

function focusMenuItem(panel: HTMLDivElement | null, item: HTMLAnchorElement | undefined) {
  if (!item || !panel) return;
  item.focus({ preventScroll: true });
  const itemBounds = item.getBoundingClientRect();
  const panelBounds = panel.getBoundingClientRect();
  if (itemBounds.top < panelBounds.top + 8) {
    panel.scrollTop += itemBounds.top - panelBounds.top - 8;
  } else if (itemBounds.bottom > panelBounds.bottom - 8) {
    panel.scrollTop += itemBounds.bottom - panelBounds.bottom + 8;
  }
}

function DesktopMenu({ group, pathname, open, onOpen, onClose }: DesktopMenuProps) {
  const { t } = useTranslation();
  const menuId = useId();
  const containerRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const closeTimer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const pendingFocus = useRef<'first' | 'last' | undefined>(undefined);
  const interaction = useRef<'hover' | 'pointer' | 'keyboard'>('hover');
  const clearCloseTimer = () => {
    clearTimeout(closeTimer.current);
    closeTimer.current = undefined;
  };
  const focusItem = (position: 'first' | 'last') => {
    const items = panelRef.current?.querySelectorAll<HTMLAnchorElement>('[role="menuitem"]');
    if (items?.length)
      focusMenuItem(panelRef.current, items[position === 'first' ? 0 : items.length - 1]);
  };
  const openWithFocus = (position: 'first' | 'last') => {
    interaction.current = 'keyboard';
    clearCloseTimer();
    if (open) focusItem(position);
    else {
      pendingFocus.current = position;
      onOpen();
    }
  };
  const handlePointerEnter = (event: ReactPointerEvent) => {
    if (event.pointerType !== 'mouse' || !window.matchMedia('(any-hover: hover)').matches) return;
    // Preserve the focused menu until an explicit click or keyboard action.
    if (
      document.activeElement?.closest('[role="menu"]') &&
      !containerRef.current?.contains(document.activeElement)
    )
      return;
    clearCloseTimer();
    if (!open) interaction.current = 'hover';
    onOpen();
  };
  const handlePointerLeave = (event: ReactPointerEvent) => {
    if (event.pointerType !== 'mouse') return;
    clearCloseTimer();
    closeTimer.current = setTimeout(() => {
      // A keyboard user can keep navigating while the pointer moves away.
      if (
        interaction.current !== 'keyboard' ||
        !containerRef.current?.contains(document.activeElement)
      )
        onClose();
    }, 180);
  };

  useLayoutEffect(() => {
    const panel = panelRef.current;
    if (open && panel) {
      panel.style.maxHeight = `${Math.max(0, window.innerHeight - panel.getBoundingClientRect().top - 12)}px`;
    }
  }, [open]);
  useEffect(() => () => clearTimeout(closeTimer.current), []);
  useEffect(() => {
    if (!open) return;
    if (pendingFocus.current) {
      focusItem(pendingFocus.current);
      pendingFocus.current = undefined;
    }
    const closeOutside = (event: Event) => {
      if (event.target instanceof Node && !containerRef.current?.contains(event.target)) onClose();
    };
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key !== 'Escape') return;
      if (containerRef.current?.contains(document.activeElement)) {
        event.preventDefault();
        buttonRef.current?.focus();
      }
      onClose();
    };
    document.addEventListener('pointerdown', closeOutside);
    document.addEventListener('focusin', closeOutside);
    document.addEventListener('keydown', closeOnEscape);
    window.addEventListener('scroll', onClose, { passive: true });
    window.addEventListener('resize', onClose);
    return () => {
      document.removeEventListener('pointerdown', closeOutside);
      document.removeEventListener('focusin', closeOutside);
      document.removeEventListener('keydown', closeOnEscape);
      window.removeEventListener('scroll', onClose);
      window.removeEventListener('resize', onClose);
    };
  }, [open, onClose]);

  const handleMenuKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    interaction.current = 'keyboard';
    const items = Array.from(
      event.currentTarget.querySelectorAll<HTMLAnchorElement>('[role="menuitem"]')
    );
    const current = items.indexOf(document.activeElement as HTMLAnchorElement);
    let next: number | undefined;
    if (event.key === 'ArrowDown') next = (current + 1) % items.length;
    if (event.key === 'ArrowUp') next = (current - 1 + items.length) % items.length;
    if (event.key === 'Home' || event.key === 'PageUp') next = 0;
    if (event.key === 'End' || event.key === 'PageDown') next = items.length - 1;
    if (next !== undefined) {
      event.preventDefault();
      focusMenuItem(panelRef.current, items[next]);
    } else if (event.key === ' ') {
      event.preventDefault();
      items[current]?.click();
    } else if (event.key === 'Tab') {
      const focusable = Array.from(
        document.querySelectorAll<HTMLElement>(
          'a[href], button, input, select, textarea, [tabindex]'
        )
      ).filter(
        (element) =>
          element.tabIndex >= 0 &&
          !element.matches(':disabled') &&
          !element.closest('[inert]') &&
          element.getClientRects().length > 0
      );
      const index = focusable.indexOf(buttonRef.current!);
      const target = focusable[index + (event.shiftKey ? -1 : 1)];
      if (target) {
        event.preventDefault();
        target.focus();
      }
      onClose();
    } else if (event.key.length === 1 && !event.ctrlKey && !event.metaKey && !event.altKey) {
      const matchingItem = [...items.slice(current + 1), ...items.slice(0, current + 1)].find(
        (item) => item.textContent?.trim().toLowerCase().startsWith(event.key.toLowerCase())
      );
      if (matchingItem) {
        event.preventDefault();
        focusMenuItem(panelRef.current, matchingItem);
      }
    }
  };

  return (
    <div
      ref={containerRef}
      className="relative"
      onPointerEnter={handlePointerEnter}
      onPointerLeave={handlePointerLeave}
    >
      <button
        ref={buttonRef}
        type="button"
        id={`${menuId}-button`}
        aria-expanded={open}
        aria-haspopup="menu"
        aria-controls={open ? menuId : undefined}
        className={`nav-trigger ${open || group.items.some((item) => isNavigationActive(pathname, item.href)) ? 'text-accent' : ''}`}
        onClick={(event) => {
          clearCloseTimer();
          if (event.detail === 0) openWithFocus('first');
          else if (open && interaction.current !== 'hover') onClose();
          else {
            interaction.current = 'pointer';
            onOpen();
          }
        }}
        onKeyDown={(event) => {
          if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
            event.preventDefault();
            openWithFocus(event.key === 'ArrowDown' ? 'first' : 'last');
          }
        }}
      >
        {t(group.name)}
        <ChevronDownIcon className="size-3.5" aria-hidden="true" />
      </button>
      {open && (
        <div className="absolute right-0 top-full z-50 w-80 pt-3.5">
          <div
            ref={panelRef}
            id={menuId}
            role="menu"
            aria-labelledby={`${menuId}-button`}
            className="header-menu overscroll-contain"
            onKeyDown={handleMenuKeyDown}
          >
            {group.items.map((item) => (
              <NavigationLink
                key={item.href}
                item={item}
                role="menuitem"
                tabIndex={-1}
                active={isNavigationActive(pathname, item.href)}
                onClick={onClose}
                className={`flex items-center gap-3 rounded-md px-3 py-2.5 text-sm hover:bg-surface-2 focus-visible:bg-surface-2 focus-visible:outline-2 focus-visible:outline-accent ${isNavigationActive(pathname, item.href) ? 'text-accent' : 'text-text-secondary hover:text-text-primary'}`}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

export default function SiteHeader() {
  const { t } = useTranslation();
  const pathname = usePathname();
  const headerRef = useRef<HTMLElement>(null);
  const mobileButtonRef = useRef<HTMLButtonElement>(null);
  const restoreMobileFocus = useRef(false);
  const mobileMenuId = useId();
  const [searchVisibility, setSearchVisibility] = useState({ pathname, visible: false });
  const [searchFocused, setSearchFocused] = useState(false);
  const [mobileOpen, setMobileOpen] = useState(false);
  const [desktopMenu, setDesktopMenu] = useState<string | null>(null);
  const [previousPath, setPreviousPath] = useState(pathname);
  if (previousPath !== pathname) {
    setPreviousPath(pathname);
    setMobileOpen(false);
    setDesktopMenu(null);
    setSearchFocused(false);
  }
  const pageSearchVisible = searchVisibility.pathname === pathname && searchVisibility.visible;
  const showHeaderSearch = !pageSearchVisible || searchFocused;

  useLayoutEffect(() => {
    if (!mobileOpen && restoreMobileFocus.current) {
      restoreMobileFocus.current = false;
      mobileButtonRef.current?.focus({ preventScroll: true });
    }
  }, [mobileOpen]);

  useEffect(() => {
    const desktop = window.matchMedia('(min-width: 64rem)');
    const closeOnDesktop = () => {
      if (desktop.matches) setMobileOpen(false);
    };
    desktop.addEventListener('change', closeOnDesktop);
    return () => desktop.removeEventListener('change', closeOnDesktop);
  }, []);

  useEffect(() => {
    const header = headerRef.current;
    const main = document.getElementById('main-content');
    if (!header || !main) return;
    let observer: IntersectionObserver | undefined;
    const update = () => {
      const visible = Array.from(main.querySelectorAll('[data-search-placement="page"]')).some(
        (element) => {
          const bounds = element.getBoundingClientRect();
          return (
            bounds.width > 0 &&
            bounds.bottom > header.getBoundingClientRect().bottom &&
            bounds.top < innerHeight
          );
        }
      );
      setSearchVisibility((previous) =>
        previous.pathname === pathname && previous.visible === visible
          ? previous
          : { pathname, visible }
      );
    };
    const observe = () => {
      document.documentElement.style.setProperty(
        '--explorer-header-height',
        `${header.offsetHeight}px`
      );
      observer?.disconnect();
      observer = new IntersectionObserver(update, {
        rootMargin: `-${header.offsetHeight}px 0px 0px 0px`,
      });
      main
        .querySelectorAll('[data-search-placement="page"]')
        .forEach((element) => observer!.observe(element));
      update();
    };
    const mutations = new MutationObserver(observe);
    mutations.observe(main, { childList: true, subtree: true });
    const resize = new ResizeObserver(observe);
    resize.observe(header);
    const frame = requestAnimationFrame(observe);
    return () => {
      cancelAnimationFrame(frame);
      observer?.disconnect();
      mutations.disconnect();
      resize.disconnect();
    };
  }, [pathname]);

  useEffect(() => {
    const listener = (event: KeyboardEvent) => {
      if (!((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') || mobileOpen)
        return;
      event.preventDefault();
      const placement = pageSearchVisible ? 'page' : 'header';
      document
        .querySelector<HTMLInputElement>(`[data-search-placement="${placement}"] input`)
        ?.focus();
    };
    window.addEventListener('keydown', listener);
    return () => window.removeEventListener('keydown', listener);
  }, [pageSearchVisible, mobileOpen]);

  return (
    <>
      <header
        ref={headerRef}
        className="sticky top-0 z-40 border-b border-border bg-background-secondary shadow-[0_1px_8px_rgba(0,0,0,0.04)]"
        data-site-header
      >
        <div>
          <div className="mx-auto grid max-w-screen-2xl grid-cols-[minmax(0,1fr)_auto] items-center gap-x-3 gap-y-2 px-4 py-2 sm:px-6 md:grid-cols-[auto_minmax(0,1fr)_auto] lg:px-8">
            <MarketSummary />
            <div
              className={`order-3 col-span-2 min-w-0 md:order-2 md:col-span-1 md:ml-auto md:w-full md:max-w-md ${showHeaderSearch ? '' : 'hidden md:block md:invisible'}`}
              inert={!showHeaderSearch}
              aria-hidden={!showHeaderSearch}
              onFocusCapture={() => setSearchFocused(true)}
              onBlurCapture={(event) => {
                if (!event.currentTarget.contains(event.relatedTarget)) setSearchFocused(false);
              }}
            >
              <SearchBar placement="header" />
            </div>
            <div className="order-2 flex items-center gap-1.5 md:order-3">
              <Link
                href="/settings"
                className="header-control"
                aria-label={t('Site settings')}
                title={t('Site settings')}
              >
                <Cog6ToothIcon className="size-4" aria-hidden="true" />
              </Link>
              <AppearanceMenu />
              <NetworkMenu />
            </div>
          </div>
        </div>
      </header>
      <div
        className="relative z-30 border-b border-border bg-background-secondary"
        data-site-navigation
        onKeyDown={(event) => {
          if (mobileOpen && event.key === 'Escape') {
            event.preventDefault();
            mobileButtonRef.current?.focus();
            setMobileOpen(false);
          }
        }}
      >
        <div className="mx-auto flex min-h-16 max-w-screen-2xl items-center justify-between gap-6 px-4 sm:px-6 lg:px-8">
          <Link
            href="/"
            className="flex shrink-0 items-center gap-2.5 rounded focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-accent"
            aria-label={t('ZondScan home')}
          >
            <Image src="/ZondScan_Logo_static.webp" width={38} height={38} alt="" priority />
            <span>
              <span className="block font-display text-xl font-semibold tracking-wide text-text-primary">
                ZondScan
              </span>
              <span className="block text-[10px] uppercase tracking-[0.13em] text-text-muted">
                {t('QRL Testnet v2')}
              </span>
            </span>
          </Link>
          <nav aria-label={t('Main navigation')} className="hidden items-center gap-1 lg:flex">
            <Link
              href="/"
              className={`nav-trigger ${pathname === '/' ? 'text-accent' : ''}`}
              aria-current={pathname === '/' ? 'page' : undefined}
            >
              {t('Home')}
            </Link>
            {NAVIGATION_GROUPS.map((group) => (
              <DesktopMenu
                key={`${pathname}:${t(group.name)}`}
                group={group}
                pathname={pathname}
                open={desktopMenu === group.name}
                onOpen={() => setDesktopMenu(group.name)}
                onClose={() =>
                  setDesktopMenu((current) => (current === group.name ? null : current))
                }
              />
            ))}
          </nav>
          <button
            ref={mobileButtonRef}
            data-mobile-navigation-toggle
            className="header-control lg:hidden"
            type="button"
            aria-label={t(mobileOpen ? 'Close navigation' : 'Open navigation')}
            aria-expanded={mobileOpen}
            aria-controls={mobileMenuId}
            onClick={() => {
              setDesktopMenu(null);
              setMobileOpen((open) => !open);
            }}
          >
            {mobileOpen ? (
              <XMarkIcon className="size-5" aria-hidden="true" />
            ) : (
              <Bars3Icon className="size-5" aria-hidden="true" />
            )}
          </button>
        </div>
        <div id={mobileMenuId} hidden={!mobileOpen} className="border-t border-border lg:hidden">
          {mobileOpen && (
            <MobileNavigation
              pathname={pathname}
              onNavigate={() => {
                restoreMobileFocus.current = true;
                setMobileOpen(false);
              }}
            />
          )}
        </div>
      </div>
    </>
  );
}
