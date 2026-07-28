'use client';

import { useEffect } from 'react';

/**
 * Root-layout error boundary. `app/error.tsx` catches errors raised
 * inside the layout, but if the layout itself blows up (a bug in a
 * provider, a SSR hydration mismatch in <html>/<body>) Next falls back
 * to global-error.tsx, which must render its own <html> + <body>.
 *
 * Kept dependency-free so even broken theme/provider trees can render
 * something readable.
 */
export default function GlobalError({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}): JSX.Element {
  useEffect(() => {
    console.error(error);
  }, [error]);

  return (
    <html lang="en">
      <body
        style={{
          margin: 0,
          minHeight: '100vh',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          backgroundColor: '#0b0d13',
          color: '#e9ecf2',
          fontFamily: 'system-ui, -apple-system, "Segoe UI", sans-serif',
          padding: '1rem',
        }}
      >
        <main
          role="main"
          style={{
            maxWidth: 480,
            textAlign: 'center',
            border: '1px solid rgba(255, 255, 255, 0.1)',
            borderRadius: 16,
            backgroundColor: 'rgba(255, 255, 255, 0.03)',
            padding: '2rem',
          }}
        >
          <h1 style={{ color: '#ffa729', fontSize: '1.5rem', marginTop: 0, marginBottom: '0.75rem' }}>
            Something went wrong
          </h1>
          <p style={{ color: '#9aa3b2', marginBottom: '1.5rem' }}>
            The application hit an unexpected error in its layout. You can try
            reloading, or go to the home page.
          </p>
          {error?.digest ? (
            <p style={{ color: '#667085', fontSize: '0.75rem', fontFamily: 'monospace', marginBottom: '1.5rem' }}>
              Error ID: {error.digest}
            </p>
          ) : null}
          <div style={{ display: 'flex', gap: 12, justifyContent: 'center' }}>
            <button
              type="button"
              onClick={reset}
              style={{
                padding: '0.5rem 1rem',
                borderRadius: 8,
                backgroundColor: '#ffa729',
                color: '#0b0d13',
                border: 'none',
                fontWeight: 500,
                cursor: 'pointer',
              }}
            >
              Try again
            </button>
            {/* Plain anchor (not next/link) on purpose: global-error
                renders when the layout / router tree itself has failed,
                so we must do a hard navigation rather than a client-side
                push. The lint rule's normal next/link guidance doesn't
                apply at this boundary. */}
            {/* eslint-disable-next-line @next/next/no-html-link-for-pages */}
            <a
              href="/"
              style={{
                padding: '0.5rem 1rem',
                borderRadius: 8,
                backgroundColor: 'rgba(255, 255, 255, 0.06)',
                color: '#e9ecf2',
                border: '1px solid rgba(255, 255, 255, 0.1)',
                textDecoration: 'none',
              }}
            >
              Go home
            </a>
          </div>
        </main>
      </body>
    </html>
  );
}
