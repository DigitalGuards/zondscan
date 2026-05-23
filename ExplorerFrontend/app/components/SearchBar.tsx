'use client';

import { useState, useCallback, useEffect, useRef } from 'react';
import type { ChangeEvent, FormEvent } from 'react';
import { useRouter } from 'next/navigation';
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
      <div className="relative bg-card-gradient rounded-2xl p-3 sm:p-6
                    shadow-xl border border-border hover:border-border-hover transition-colors">
        <form
          onSubmit={(e: FormEvent<HTMLFormElement>) => {
            e.preventDefault();
            navigateHandler();
          }}
          className="flex flex-col sm:flex-row gap-3 sm:gap-6">
          <input
            ref={inputRef}
            type="text"
            aria-label="Search by address, transaction hash, or block number"
            placeholder="Search by Address (Qxx) / Txn Hash / Block.."
            className="flex-1 py-3 sm:py-4 px-4 sm:px-6 text-sm sm:text-base text-gray-300
                     bg-background rounded-xl
                     border border-border
                     outline-none shadow-lg
                     focus:ring-2 focus:ring-accent focus:border-transparent
                     placeholder-gray-500 transition-all duration-300
                     hover:border-border-hover"
            value={searchValue}
            onChange={handleInputChange}
          />
          <button
            type="submit"
            className="px-8 sm:px-10 py-3 sm:py-4 bg-accent text-white text-sm sm:text-base
                     rounded-xl shadow-lg font-medium whitespace-nowrap
                     hover:bg-accent-dark hover:shadow-2xl hover:scale-105
                     active:scale-95 transition-all duration-300
                     sm:w-auto w-full"
          >
            Search
          </button>
        </form>
        {error && (
          <div className="mt-3 sm:mt-4">
            <div className="p-3 sm:p-4 mb-3 sm:mb-4 text-xs sm:text-sm text-red-400 rounded-xl bg-card-gradient border border-red-400 shadow-lg" role="alert">
              <span className="font-medium">{error}</span>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
