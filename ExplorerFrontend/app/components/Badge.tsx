interface BadgeProps {
  variant: 'success' | 'warning' | 'error' | 'info' | 'neutral' | 'brand'
  children: React.ReactNode
  size?: 'sm' | 'md'
  dot?: boolean
}

const VARIANT_STYLES = {
  success: 'bg-green-900/30 text-green-400 border-green-800',
  warning: 'bg-yellow-900/30 text-yellow-400 border-yellow-800',
  error: 'bg-red-900/30 text-red-400 border-red-800',
  info: 'bg-blue-900/30 text-blue-400 border-blue-800',
  neutral: 'bg-gray-900/30 text-gray-400 border-gray-700',
  brand: 'bg-[#ffa729]/20 text-[#ffa729] border-[#ffa729]/30',
} as const

const DOT_COLORS = {
  success: 'bg-green-400',
  warning: 'bg-yellow-400',
  error: 'bg-red-400',
  info: 'bg-blue-400',
  neutral: 'bg-gray-400',
  brand: 'bg-[#ffa729]',
} as const

const SIZE_STYLES = {
  sm: 'px-2 py-0.5 text-xs',
  md: 'px-3 py-1 text-sm',
} as const

export default function Badge({ variant, children, size = 'sm', dot = false }: BadgeProps): JSX.Element {
  return (
    <span className={`inline-flex items-center rounded-full font-medium border ${VARIANT_STYLES[variant]} ${SIZE_STYLES[size]}`}>
      {dot && <span className={`w-1.5 h-1.5 rounded-full ${DOT_COLORS[variant]} mr-1.5`} />}
      {children}
    </span>
  )
}
