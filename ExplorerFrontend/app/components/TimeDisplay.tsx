'use client';

import { useSyncExternalStore } from 'react';
import { useTranslation } from './InterfaceText';
import { formatDateTime, unixSeconds } from '../lib/preferences';
import { usePreferences } from './PreferencesProvider';

// One clock serves all relative timestamps and stops when the last view unmounts.
let clockSeconds = 0;
let clockTimer: ReturnType<typeof setInterval> | undefined;
const clockListeners = new Set<() => void>();
const getClock = () => clockSeconds;
const zeroClock = () => 0;
const noClock = () => () => {};
function subscribeClock(listener: () => void) {
  if (clockListeners.size === 0) {
    clockSeconds = Math.floor(Date.now() / 1000);
    clockTimer = setInterval(() => {
      if (document.hidden) return;
      clockSeconds = Math.floor(Date.now() / 1000);
      clockListeners.forEach((notify) => notify());
    }, 30_000);
  }
  clockListeners.add(listener);
  return () => {
    clockListeners.delete(listener);
    if (clockListeners.size === 0) clearInterval(clockTimer);
  };
}

export default function TimeDisplay({
  timestamp,
  relative = false,
  clockOnly = false,
  dateOnly = false,
}: {
  timestamp: string | number | null | undefined;
  relative?: boolean;
  clockOnly?: boolean;
  dateOnly?: boolean;
}) {
  const { preferences } = usePreferences();
  const { t, locale } = useTranslation();
  const now = useSyncExternalStore(
    relative ? subscribeClock : noClock,
    relative ? getClock : zeroClock,
    zeroClock
  );
  const seconds = unixSeconds(timestamp);
  if (seconds === null) return <span>{t('Unknown')}</span>;
  const formatted = formatDateTime(seconds, preferences.timeZone, locale);
  const age = Math.max(0, Math.floor(now - seconds));
  const relativeUnit: Intl.RelativeTimeFormatUnit =
    age < 60 ? 'second' : age < 3600 ? 'minute' : age < 86400 ? 'hour' : 'day';
  const relativeAmount = Math.floor(
    age / { second: 1, minute: 60, hour: 3600, day: 86400 }[relativeUnit]
  );
  return (
    <time lang={locale} dateTime={new Date(seconds * 1000).toISOString()} title={formatted}>
      {relative
        ? now === 0
          ? '...'
          : locale === 'en'
            ? `${relativeAmount}${{ second: 's', minute: 'm', hour: 'h', day: 'd' }[relativeUnit]} ago`
            : new Intl.RelativeTimeFormat(locale, { style: 'short' }).format(
                -relativeAmount,
                relativeUnit
              )
        : dateOnly
          ? new Intl.DateTimeFormat(locale === 'en' ? 'en-GB' : locale, {
              day: '2-digit',
              month: 'short',
              year: 'numeric',
              timeZone: preferences.timeZone === 'utc' ? 'UTC' : undefined,
            }).format(new Date(seconds * 1000))
          : clockOnly
            ? new Intl.DateTimeFormat(locale === 'en' ? 'en-GB' : locale, {
                hour: '2-digit',
                minute: '2-digit',
                second: '2-digit',
                hourCycle: 'h23',
                timeZone: preferences.timeZone === 'utc' ? 'UTC' : undefined,
                timeZoneName: 'short',
              }).format(new Date(seconds * 1000))
            : formatted}
    </time>
  );
}
