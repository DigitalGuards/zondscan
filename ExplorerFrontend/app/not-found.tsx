import Link from 'next/link'
import EmptyState from './components/EmptyState'

export default function NotFound(): JSX.Element {
  return (
    <div className="min-h-[60vh] flex items-center justify-center">
      <EmptyState
        icon={
          <svg className="h-12 w-12 text-gray-500" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
          </svg>
        }
        title="404 — Page not found"
        description="The block, transaction, or address you're looking for doesn't exist or may have been removed."
        actionLabel="Return home"
        actionHref="/"
      />
    </div>
  )
}
