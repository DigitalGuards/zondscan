/**
 * Canonical palette for JS-land consumers (SVG charts, third-party widgets,
 * inline styles) that can't read the Tailwind tokens in globals.css.
 * Keep these in sync with the @theme block; components must import from here
 * instead of hardcoding hex values.
 */
export const palette = {
  background: '#0b0d13',
  backgroundSecondary: '#12151d',
  backgroundTertiary: '#0e1118',

  accent: '#ffa729',
  accentHover: '#ffbc57',
  accentDark: '#f29414',
  quantum: '#2ee6c8',

  textPrimary: '#e9ecf2',
  textSecondary: '#9aa3b2',
  textMuted: '#667085',

  success: '#34d399',
  warning: '#fbbf24',
  error: '#f87171',
  info: '#60a5fa',
} as const;

/** Shared visx/SVG chart styling derived from the palette. */
export const chartTheme = {
  axis: palette.textMuted,
  tickLabel: palette.textSecondary,
  grid: 'rgba(255, 255, 255, 0.07)',
  line: palette.accent,
  area: palette.accent,
  fontFamily: 'IBM Plex Mono, ui-monospace, monospace',
  tooltip: {
    background: 'rgba(18, 21, 29, 0.96)',
    border: '1px solid rgba(255, 255, 255, 0.1)',
    color: palette.textPrimary,
    padding: '8px 12px',
    borderRadius: '10px',
    boxShadow: '0 12px 32px -12px rgba(0, 0, 0, 0.6)',
  },
} as const;
