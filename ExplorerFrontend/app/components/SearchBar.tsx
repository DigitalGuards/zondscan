'use client';

import { useTranslation } from './InterfaceText';

import { useState, useCallback, useId } from 'react';
import type { ChangeEvent, FormEvent } from 'react';
import { useRouter } from 'next/navigation';
import { MagnifyingGlassIcon } from '@heroicons/react/24/outline';
import { resolveSearchPath } from '../lib/searchResolver';

export default function SearchBar({
  placement = 'page',
}: {
  placement?: 'page' | 'header';
}): JSX.Element {
  const [searchValue, setSearchValue] = useState<string>('');
  const [error, setError] = useState<string>('');
  const router = useRouter();
  const errorId = useId();
  const compact = placement === 'header';
  const { t, locale } = useTranslation();

  function handleInputChange(event: ChangeEvent<HTMLInputElement>): void {
    setSearchValue(event.target.value);
    setError('');
  }

  // resolveSearchPath handles trim, paste-noise, missing 0x prefix on tx
  // hashes, hex block numbers, and per-shape error messages; the component
  // is just the form chrome.
  const navigateHandler = useCallback((): void => {
    const result = resolveSearchPath(searchValue);
    if ('error' in result) {
      setError(result.error);
      return;
    }
    router.push(result.path);
  }, [searchValue, router]);

  return (
    <div lang={locale} className="relative w-full">
      {/* One integrated pill: icon, input, kbd hint, action. The glow on
          focus-within is the page's primary "you are here" moment. */}
      <form
        role="search"
        aria-label={t(compact ? 'Header search' : 'Page search')}
        data-search-placement={placement}
        onSubmit={(e: FormEvent<HTMLFormElement>) => {
          e.preventDefault();
          navigateHandler();
        }}
        className={`group relative flex items-center gap-2
                   bg-background-secondary/80 backdrop-blur-sm
                   border border-border
                   transition-all duration-300
                   hover:border-border-hover
                   focus-within:border-accent/60
                   focus-within:shadow-[0_0_0_1px_rgba(255,167,41,0.25),0_0_40px_-8px_rgba(255,167,41,0.25)]
                   ${compact ? 'h-9 rounded-lg px-2.5' : 'rounded-2xl p-2 pl-4 shadow-card'}`}
      >
        <MagnifyingGlassIcon
          className={`${compact ? 'size-4' : 'size-5'} flex-shrink-0 text-text-muted transition-colors group-focus-within:text-accent`}
          aria-hidden="true"
        />
        <input
          type="text"
          aria-label={t('Search by address, transaction hash, or block number')}
          placeholder={t(
            compact ? 'Search address / txn / block' : 'Search by address / txn hash / block number'
          )}
          aria-invalid={Boolean(error)}
          aria-describedby={error ? errorId : undefined}
          className={`flex-1 min-w-0 bg-transparent ${compact ? 'py-1.5 text-xs' : 'py-2.5 text-sm sm:text-base'}
                     text-text-primary placeholder-text-muted
                     outline-none border-none focus:ring-0`}
          value={searchValue}
          onChange={handleInputChange}
        />
        <kbd
          className={`${compact ? 'hidden xl:inline-flex' : 'hidden md:inline-flex'} items-center gap-1 px-2 py-1 rounded-md
                     bg-surface-2 border border-border text-[11px] font-mono
                     text-text-muted select-none`}
          aria-hidden="true"
        >
          Ctrl K
        </kbd>
        <button
          type="submit"
          aria-label={compact ? t('Submit header search') : undefined}
          className={
            compact
              ? 'rounded p-1 text-text-muted hover:text-accent focus-visible:outline-2 focus-visible:outline-accent'
              : 'btn-primary px-5 sm:px-7 py-2.5 text-sm sm:text-base rounded-xl whitespace-nowrap'
          }
        >
          {compact ? <MagnifyingGlassIcon className="size-4" aria-hidden="true" /> : t('Search')}
        </button>
      </form>
      {error && (
        <div
          id={errorId}
          lang="en"
          className={`${compact ? 'absolute left-0 right-0 top-full z-10 mt-2 bg-background-secondary shadow-lg' : 'mt-3 bg-error/10'} px-4 py-3 text-xs sm:text-sm text-error rounded-xl border border-error/25`}
          role="alert"
        >
          <span className="font-medium">{error}</span>
        </div>
      )}
    </div>
  );
}
