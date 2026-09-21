/** Semantic CSS colors keep SVG charts and inline styles in sync with appearance. */
export const palette = {
  background: 'var(--color-background)',
  backgroundSecondary: 'var(--color-background-secondary)',
  backgroundTertiary: 'var(--color-background-tertiary)',

  accent: 'var(--color-accent)',
  accentHover: 'var(--color-accent-hover)',
  accentDark: 'var(--color-accent-dark)',
  quantum: 'var(--color-quantum)',

  textPrimary: 'var(--color-text-primary)',
  textSecondary: 'var(--color-text-secondary)',
  textMuted: 'var(--color-text-muted)',

  success: 'var(--color-success)',
  warning: 'var(--color-warning)',
  error: 'var(--color-error)',
  info: 'var(--color-info)',
} as const;

/** Shared visx/SVG chart styling derived from the palette. */
export const chartTheme = {
  axis: palette.textMuted,
  tickLabel: palette.textSecondary,
  grid: 'var(--color-border)',
  line: palette.accent,
  area: palette.accent,
  fontFamily: 'IBM Plex Mono, ui-monospace, monospace',
  tooltip: {
    background: palette.backgroundSecondary,
    border: '1px solid var(--color-border)',
    color: palette.textPrimary,
    padding: '8px 12px',
    borderRadius: '10px',
    boxShadow: '0 12px 32px -12px rgba(0, 0, 0, 0.6)',
  },
} as const;
