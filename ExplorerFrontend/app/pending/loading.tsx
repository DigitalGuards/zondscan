/**
 * Route-level skeleton shared by /pending/<page> (the mempool list)
 * and /pending/tx/<hash> (a single pending tx detail). Both routes
 * do server-side fetches; without a fallback the page is briefly
 * blank between click and data.
 *
 * Layout matches the most visible parts of the pending tx detail
 * (header + live status strip + key-value rows + token-call card)
 * so the transition reads as "the data is filling in" rather than
 * "the page reloaded".
 */
export default function Loading(): JSX.Element {
  return (
    <div role="status" aria-label="Loading pending transaction" className="py-4 sm:py-6 lg:py-8">
      {/* Header card */}
      <div className="card overflow-hidden mb-6">
        <div className="flex items-center justify-between p-4 sm:p-6 border-b border-border">
          <div className="flex items-center gap-3">
            <div className="w-6 h-6 rounded-full skeleton" />
            <div className="h-7 w-56 skeleton" />
          </div>
          <div className="h-6 w-20 skeleton rounded-full" />
        </div>

        {/* Live status strip stub */}
        <div className="px-4 sm:px-6 py-3 border-b border-border bg-background/40">
          <div className="flex flex-wrap gap-2">
            <div className="h-5 w-32 skeleton rounded-full" />
            <div className="h-5 w-36 skeleton rounded-full" />
            <div className="h-5 w-28 skeleton rounded-full" />
          </div>
        </div>

        {/* Detail rows stub */}
        <div className="p-4 sm:p-6 space-y-4">
          {[...Array(7)].map((_, i) => (
            <div key={i} className="border-b border-border/50 pb-3 last:border-b-0">
              <div className="h-3 w-24 skeleton mb-2" />
              <div className="h-5 w-full max-w-md skeleton" />
            </div>
          ))}
        </div>
      </div>

      {/* Token call card stub */}
      <div className="card overflow-hidden">
        <div className="px-4 sm:px-6 py-4 border-b border-border flex items-center gap-2">
          <div className="h-5 w-40 skeleton" />
          <div className="h-5 w-16 skeleton rounded-full" />
        </div>
        <div className="p-4 sm:p-6 space-y-3">
          {[0, 1, 2].map((i) => (
            <div key={i} className="flex items-center gap-3">
              <div className="h-3 w-20 skeleton" />
              <div className="h-3 w-56 skeleton" />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
