"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";
import PreferencesProvider from './components/PreferencesProvider';
import AddressHighlights from './components/AddressHighlights';

export default function Providers({ children }: { children: React.ReactNode }): JSX.Element {
  const [queryClient] = useState(() => new QueryClient());

  return (
    <QueryClientProvider client={queryClient}>
      <PreferencesProvider>
        <AddressHighlights />
        {children}
      </PreferencesProvider>
    </QueryClientProvider>
  );
}
