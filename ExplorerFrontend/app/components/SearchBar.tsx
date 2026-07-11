'use client';

import { useState, useCallback, useEffect, useRef } from 'react';
import type { ChangeEvent, FormEvent } from 'react';
import { useRouter } from 'next/navigation';
import { MagnifyingGlassIcon } from '@heroicons/react/24/outline';
import { resolveSearchPath } from '../lib/searchResolver';

export default function SearchBar(): JSX.Element {
  const [searchValue, setSearchValue] = useState<string>('');
  const [error, setError] = useState<string>('');
  const router = useRouter();
  const inputRef = useRef<HTMLInputElement>(null);

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

  useEffect(() => {
    const listener = (event: KeyboardEvent): void => {
      if ((event.metaKey || event.ctrlKey) && event.key === 'k') {
        event.preventDefault();
        inputRef.current?.focus();
      }
    };
    window.addEventListener("keydown", listener);
    return () => {
      window.removeEventListener("keydown", listener);
    };
  }, []);

  return (
    <div className="relative w-full">
      {/* One integrated pill: icon, input, kbd hint, action. The glow on
          focus-within is the page's primary "you are here" moment. */}
      <form
        onSubmit={(e: FormEvent<HTMLFormElement>) => {
          e.preventDefault();
          navigateHandler();
        }}
        className="group relative flex items-center gap-2 rounded-2xl
                   bg-background-secondary/80 backdrop-blur-sm
                   border border-border
                   shadow-card
                   transition-all duration-300
                   hover:border-border-hover
                   focus-within:border-accent/60
                   focus-within:shadow-[0_0_0_1px_rgba(255,167,41,0.25),0_0_40px_-8px_rgba(255,167,41,0.25)]
                   p-2 pl-4"
      >
        <MagnifyingGlassIcon
          className="w-5 h-5 flex-shrink-0 text-text-muted transition-colors
                     group-focus-within:text-accent"
          aria-hidden="true"
        />
        <input
          ref={inputRef}
          type="text"
          aria-label="Search by address, transaction hash, or block number"
          placeholder="Search by address / txn hash / block number"
          className="flex-1 min-w-0 bg-transparent py-2.5 text-sm sm:text-base
                     text-text-primary placeholder-text-muted
                     outline-none border-none focus:ring-0"
          value={searchValue}
          onChange={handleInputChange}
        />
        <kbd
          className="hidden md:inline-flex items-center gap-1 px-2 py-1 rounded-md
                     bg-surface-2 border border-border text-[11px] font-mono
                     text-text-muted select-none"
          aria-hidden="true"
        >
          Ctrl K
        </kbd>
        <button
          type="submit"
          className="btn-primary px-5 sm:px-7 py-2.5 text-sm sm:text-base rounded-xl
                     whitespace-nowrap"
        >
          Search
        </button>
      </form>
      {error && (
        <div
          className="mt-3 px-4 py-3 text-xs sm:text-sm text-error rounded-xl
                     bg-error/10 border border-error/25"
          role="alert"
        >
          <span className="font-medium">{error}</span>
        </div>
      )}
    </div>
  );
}
