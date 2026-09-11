'use client';

import { useEffect } from 'react';
import { usePathname } from 'next/navigation';
import { usePreferences } from './PreferencesProvider';

/** Compare complete addresses. Only an address span or keyboard-focused link activates a match. */
export default function AddressHighlights() {
  const { preferences } = usePreferences();
  const pathname = usePathname();
  useEffect(() => {
    const clear = () =>
      document
        .querySelectorAll('[data-address-highlight]')
        .forEach((element) => element.removeAttribute('data-address-highlight'));
    if (!preferences.highlightAddresses) return clear;
    let pointerAddress: string | undefined;
    let focusAddress: string | undefined;
    let appliedAddress: string | undefined;
    const update = () => {
      const address = pointerAddress || focusAddress;
      if (address === appliedAddress) return;
      appliedAddress = address;
      clear();
      if (!address) return;
      document.querySelectorAll<HTMLElement>('[data-explorer-address]').forEach((element) => {
        if (element.dataset.explorerAddress === address)
          element.setAttribute('data-address-highlight', 'true');
      });
    };
    const addressAt = (target: EventTarget | null) =>
      target instanceof Element
        ? target.closest<HTMLElement>('[data-explorer-address]')?.dataset.explorerAddress
        : undefined;
    const onPointerOver = (event: PointerEvent) => {
      pointerAddress = addressAt(event.target);
      update();
    };
    const onPointerOut = (event: PointerEvent) => {
      pointerAddress = addressAt(event.relatedTarget);
      update();
    };
    const onFocusIn = (event: FocusEvent) => {
      const target = event.target;
      focusAddress =
        target instanceof HTMLElement && target.matches(':focus-visible')
          ? addressAt(target) ||
            target.querySelector<HTMLElement>('[data-explorer-address]')?.dataset.explorerAddress
          : undefined;
      update();
    };
    const onFocusOut = () => {
      focusAddress = undefined;
      update();
    };
    document.addEventListener('pointerover', onPointerOver);
    document.addEventListener('pointerout', onPointerOut);
    document.addEventListener('focusin', onFocusIn);
    document.addEventListener('focusout', onFocusOut);
    return () => {
      clear();
      document.removeEventListener('pointerover', onPointerOver);
      document.removeEventListener('pointerout', onPointerOut);
      document.removeEventListener('focusin', onFocusIn);
      document.removeEventListener('focusout', onFocusOut);
    };
  }, [pathname, preferences.highlightAddresses]);
  return null;
}
