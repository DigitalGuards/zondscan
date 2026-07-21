'use client';

import { useCallback } from 'react';
import { useSearchParams } from 'next/navigation';

type HistoryMode = 'replace' | 'push';

/**
 * Write query-param updates to the URL via the native History API.
 * Next.js 15 syncs useSearchParams from history.pushState/replaceState, so
 * this is a shallow update: no server-component re-render, no refetch, no
 * scroll jump. Pass null (or '') to remove a key. Use a single call when
 * several params must change together so Back restores them atomically.
 */
export function setUrlParams(
  updates: Record<string, string | null>,
  history: HistoryMode = 'replace'
): void {
  const url = new URL(window.location.href);
  for (const [key, value] of Object.entries(updates)) {
    if (value === null || value === '') {
      url.searchParams.delete(key);
    } else {
      url.searchParams.set(key, value);
    }
  }
  if (history === 'push') {
    window.history.pushState(null, '', url.toString());
  } else {
    window.history.replaceState(null, '', url.toString());
  }
}

/**
 * A single URL query param as state. The URL is the source of truth (so
 * browser Back/Forward restores it); the setter writes shallowly via the
 * History API. Setting the default value removes the key, keeping canonical
 * URLs clean. Components using this must render under a Suspense boundary
 * when statically prerendered (useSearchParams requirement).
 */
export function useUrlParam(
  key: string,
  defaultValue: string,
  opts: { history?: HistoryMode } = {}
): [string, (next: string) => void] {
  const searchParams = useSearchParams();
  const value = searchParams.get(key) ?? defaultValue;
  const { history = 'replace' } = opts;

  const setValue = useCallback(
    (next: string) => {
      setUrlParams({ [key]: next === defaultValue ? null : next }, history);
    },
    [key, defaultValue, history]
  );

  return [value, setValue];
}

/**
 * Integer variant of useUrlParam for page numbers and the like. Malformed or
 * out-of-range values in the URL fall back to the default instead of NaN.
 */
export function useUrlIntParam(
  key: string,
  defaultValue: number,
  opts: { history?: HistoryMode; min?: number } = {}
): [number, (next: number) => void] {
  const { min = 1 } = opts;
  const [raw, setRaw] = useUrlParam(key, String(defaultValue), opts);
  const parsed = Number.parseInt(raw, 10);
  const value = Number.isFinite(parsed) && parsed >= min ? parsed : defaultValue;

  const setValue = useCallback(
    (next: number) => {
      setRaw(String(next));
    },
    [setRaw]
  );

  return [value, setValue];
}
