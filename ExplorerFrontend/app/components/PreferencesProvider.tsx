'use client';

import { createContext, useCallback, useContext, useEffect, useSyncExternalStore } from 'react';
import {
  DEFAULT_PREFERENCES,
  PREFERENCES_EVENT,
  PREFERENCES_KEY,
  parsePreferences,
  resolveTheme,
  type ExplorerPreferences,
} from '../lib/preferences';

const serverSnapshot = { preferences: DEFAULT_PREFERENCES, ready: false, persistent: true };
let snapshot = { ...serverSnapshot, ready: true };
let previousRaw: string | null | undefined;
let memoryOnly = false;

function getSnapshot() {
  try {
    const raw = localStorage.getItem(PREFERENCES_KEY);
    if (!memoryOnly && raw !== previousRaw) {
      previousRaw = raw;
      snapshot = { preferences: parsePreferences(raw), ready: true, persistent: true };
    }
  } catch {
    if (snapshot.persistent) snapshot = { ...snapshot, persistent: false };
  }
  return snapshot;
}

function subscribe(listener: () => void) {
  const onStorage = (event: StorageEvent) => {
    if (event.key === PREFERENCES_KEY || event.key === null) {
      memoryOnly = false;
      previousRaw = undefined;
      listener();
    }
  };
  window.addEventListener('storage', onStorage);
  window.addEventListener(PREFERENCES_EVENT, listener);
  return () => {
    window.removeEventListener('storage', onStorage);
    window.removeEventListener(PREFERENCES_EVENT, listener);
  };
}

const PreferencesContext = createContext({
  ...serverSnapshot,
  updatePreferences: (_changes: Partial<ExplorerPreferences>): boolean => false,
  resetPreferences: (): boolean => false,
});

export function usePreferences() {
  return useContext(PreferencesContext);
}

function subscribeSystemTheme(listener: () => void) {
  const media = window.matchMedia('(prefers-color-scheme: dark)');
  media.addEventListener('change', listener);
  return () => media.removeEventListener('change', listener);
}

export function useResolvedTheme() {
  const { preferences } = usePreferences();
  const systemDark = useSyncExternalStore(
    subscribeSystemTheme,
    () => window.matchMedia('(prefers-color-scheme: dark)').matches,
    () => true
  );
  return resolveTheme(preferences.theme, systemDark);
}

export default function PreferencesProvider({ children }: { children: React.ReactNode }) {
  const current = useSyncExternalStore(subscribe, getSnapshot, () => serverSnapshot);
  const updatePreferences = useCallback((changes: Partial<ExplorerPreferences>): boolean => {
    const preferences = parsePreferences(
      JSON.stringify({ ...getSnapshot().preferences, ...changes })
    );
    const raw = JSON.stringify(preferences);
    let persistent = true;
    try {
      localStorage.setItem(PREFERENCES_KEY, raw);
      previousRaw = raw;
      memoryOnly = false;
    } catch {
      persistent = false;
      memoryOnly = true;
    }
    snapshot = { preferences, ready: true, persistent };
    window.dispatchEvent(new Event(PREFERENCES_EVENT));
    return persistent;
  }, []);
  const resetPreferences = useCallback(
    () => updatePreferences(DEFAULT_PREFERENCES),
    [updatePreferences]
  );

  useEffect(() => {
    if (!current.ready) return;
    const media = window.matchMedia('(prefers-color-scheme: dark)');
    const applyTheme = () => {
      const theme = resolveTheme(current.preferences.theme, media.matches);
      document.documentElement.dataset.theme = theme;
      document.documentElement.classList.toggle('dark', theme !== 'light');
      document.documentElement.style.colorScheme = theme === 'light' ? 'light' : 'dark';
    };
    applyTheme();
    media.addEventListener('change', applyTheme);
    return () => media.removeEventListener('change', applyTheme);
  }, [current.preferences.theme, current.ready]);

  useEffect(() => {
    if (current.ready) document.documentElement.lang = current.preferences.locale;
  }, [current.preferences.locale, current.ready]);

  return (
    <PreferencesContext.Provider value={{ ...current, updatePreferences, resetPreferences }}>
      {children}
    </PreferencesContext.Provider>
  );
}
