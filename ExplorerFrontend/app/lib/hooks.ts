'use client';

import { useEffect, useState } from 'react';

/**
 * Mobile / desktop switch for components that render a card list under the
 * breakpoint and a `<table>` (or wider layout) above it. SSR-safe: starts
 * false on the server and resyncs on mount + every resize.
 *
 * Single shared implementation so the address-page table panels and the
 * transaction detail view use the same threshold instead of redefining the
 * resize listener each time.
 */
export const useIsMobile = (breakpointPx = 768): boolean => {
  const [isMobile, setIsMobile] = useState(false);
  useEffect(() => {
    const check = (): void => setIsMobile(window.innerWidth < breakpointPx);
    check();
    window.addEventListener('resize', check);
    return () => window.removeEventListener('resize', check);
  }, [breakpointPx]);
  return isMobile;
};
