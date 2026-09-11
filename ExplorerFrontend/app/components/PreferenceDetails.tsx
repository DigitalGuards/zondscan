'use client';

import { useState } from 'react';
import { usePreferences } from './PreferencesProvider';

export default function PreferenceDetails({
  children,
  className = '',
}: {
  children: React.ReactNode;
  className?: string;
}) {
  const { preferences } = usePreferences();
  const [open, setOpen] = useState(preferences.expandDetails);
  const [previousDefault, setPreviousDefault] = useState(preferences.expandDetails);
  if (previousDefault !== preferences.expandDetails) {
    setPreviousDefault(preferences.expandDetails);
    setOpen(preferences.expandDetails);
  }
  return (
    <details
      className={className}
      open={open}
      onToggle={(event) => setOpen(event.currentTarget.open)}
    >
      {children}
    </details>
  );
}
