interface BadgeProps {
  variant: 'success' | 'warning' | 'error' | 'info' | 'neutral' | 'brand'
  children: React.ReactNode
  size?: 'sm' | 'md'
  dot?: boolean
}

/* One badge recipe across the app: soft status fill, hairline border in the
   same hue, status-colored text. StatusBadge/MethodBadge follow the same
   pattern so "status" reads identically on every page. */
const VARIANT_STYLES = {
  success: 'bg-success/10 text-success border-success/25',
  warning: 'bg-warning/10 text-warning border-warning/25',
  error: 'bg-error/10 text-error border-error/25',
  info: 'bg-info/10 text-info border-info/25',
  neutral: 'bg-surface-2 text-text-secondary border-border',
  brand: 'bg-accent/10 text-accent border-accent/25',
} as const

const DOT_COLORS = {
  success: 'bg-success',
  warning: 'bg-warning',
  error: 'bg-error',
  info: 'bg-info',
  neutral: 'bg-text-muted',
  brand: 'bg-accent',
} as const

const SIZE_STYLES = {
  sm: 'px-2 py-0.5 text-xs',
  md: 'px-3 py-1 text-sm',
} as const

export default function Badge({ variant, children, size = 'sm', dot = false }: BadgeProps): JSX.Element {
  return (
    <span className={`inline-flex items-center rounded-md font-medium border ${VARIANT_STYLES[variant]} ${SIZE_STYLES[size]}`}>
      {dot && <span className={`w-1.5 h-1.5 rounded-full ${DOT_COLORS[variant]} mr-1.5`} />}
      {children}
    </span>
  )
}
