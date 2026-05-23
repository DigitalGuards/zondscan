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
    <div role="status" aria-label="Loading pending transaction" className="p-4 sm:p-8 max-w-6xl mx-auto">
      {/* Header card */}
      <div className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-xl overflow-hidden mb-6">
        <div className="flex items-center justify-between p-4 sm:p-6 border-b border-[#3d3d3d]">
          <div className="flex items-center gap-3">
            <div className="w-6 h-6 rounded-full bg-[#ffa729]/20 animate-pulse" />
            <div className="h-7 w-56 bg-[#ffa729]/20 rounded animate-pulse" />
          </div>
          <div className="h-6 w-20 bg-yellow-500/20 rounded-full animate-pulse" />
        </div>

        {/* Live status strip stub */}
        <div className="px-4 sm:px-6 py-3 border-b border-[#3d3d3d] bg-[#1a1a1a]/40">
          <div className="flex flex-wrap gap-2">
            <div className="h-5 w-32 bg-yellow-500/20 rounded-full animate-pulse" />
            <div className="h-5 w-36 bg-blue-500/20 rounded-full animate-pulse" />
            <div className="h-5 w-28 bg-gray-700/40 rounded-full animate-pulse" />
          </div>
        </div>

        {/* Detail rows stub */}
        <div className="p-4 sm:p-6 space-y-4">
          {[...Array(7)].map((_, i) => (
            <div key={i} className="border-b border-[#3d3d3d]/50 pb-3 last:border-b-0">
              <div className="h-3 w-24 bg-gray-700/40 rounded mb-2 animate-pulse" />
              <div className="h-5 w-full max-w-md bg-gray-700/40 rounded animate-pulse" />
            </div>
          ))}
        </div>
      </div>

      {/* Token call card stub */}
      <div className="rounded-xl bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] border border-[#3d3d3d] shadow-xl overflow-hidden">
        <div className="px-4 sm:px-6 py-4 border-b border-[#3d3d3d] flex items-center gap-2">
          <div className="h-5 w-40 bg-[#ffa729]/20 rounded animate-pulse" />
          <div className="h-5 w-16 bg-[#ffa729]/20 rounded-full animate-pulse" />
        </div>
        <div className="p-4 sm:p-6 space-y-3">
          {[0, 1, 2].map((i) => (
            <div key={i} className="flex items-center gap-3">
              <div className="h-3 w-20 bg-gray-700/40 rounded animate-pulse" />
              <div className="h-3 w-56 bg-gray-700/40 rounded animate-pulse" />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
