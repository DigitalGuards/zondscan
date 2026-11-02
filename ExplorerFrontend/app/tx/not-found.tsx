import EmptyState from '../components/EmptyState'

// Rendered when /tx/<hash> matches nothing. The tx page has already checked
// the hash against transactions, blocks and QNS name hashes.
export default function TransactionNotFound(): JSX.Element {
  return (
    <div className="min-h-[60vh] flex items-center justify-center">
      <EmptyState
        icon={
          <svg className="h-12 w-12 text-text-muted" fill="none" viewBox="0 0 24 24" strokeWidth={1.5} stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" d="m21 21-5.197-5.197m0 0A7.5 7.5 0 1 0 5.196 5.196a7.5 7.5 0 0 0 10.607 10.607Z" />
          </svg>
        }
        title="404, Page not found"
        description="This hash matched no transaction, block or QNS name."
        actionLabel="Return home"
        actionHref="/"
      />
    </div>
  )
}
