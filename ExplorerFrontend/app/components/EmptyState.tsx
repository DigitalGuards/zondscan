import Link from 'next/link'

interface EmptyStateProps {
  icon?: React.ReactNode
  title: string
  description?: string
  actionLabel?: string
  actionHref?: string
}

const DefaultIcon = (): JSX.Element => (
  <svg xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24" strokeWidth={1} stroke="currentColor" className="w-12 h-12 text-gray-600">
    <path strokeLinecap="round" strokeLinejoin="round" d="M20.25 7.5l-.625 10.632a2.25 2.25 0 01-2.247 2.118H6.622a2.25 2.25 0 01-2.247-2.118L3.75 7.5M10 11.25h4M3.375 7.5h17.25c.621 0 1.125-.504 1.125-1.125v-1.5c0-.621-.504-1.125-1.125-1.125H3.375c-.621 0-1.125.504-1.125 1.125v1.5c0 .621.504 1.125 1.125 1.125z" />
  </svg>
)

export default function EmptyState({ icon, title, description, actionLabel, actionHref }: EmptyStateProps): JSX.Element {
  return (
    <div className="flex flex-col items-center justify-center py-12 px-4 text-center">
      <div className="mb-4">
        {icon || <DefaultIcon />}
      </div>
      <h3 className="text-lg font-medium text-gray-300 mb-2">{title}</h3>
      {description && (
        <p className="text-sm text-gray-500 max-w-md mb-4">{description}</p>
      )}
      {actionLabel && actionHref && (
        <Link
          href={actionHref}
          className="inline-flex items-center px-4 py-2 text-sm font-medium text-[#ffa729] bg-[#ffa729]/10 rounded-lg hover:bg-[#ffa729]/20 transition-colors"
        >
          {actionLabel}
        </Link>
      )}
    </div>
  )
}
