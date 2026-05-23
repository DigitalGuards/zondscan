/**
 * Route-level skeleton for /address/<addr>. The page is server-rendered
 * and fans out into several Mongo aggregations (transactions list,
 * internal tx list, contract metadata, holdings) so the wait between
 * clicking an address link and seeing data can be a couple of seconds.
 * The Suspense fallback here shows a stand-in panel + transactions
 * table immediately so the navigation feels instant.
 *
 * Shape mirrors the actual page: a header strip with the address +
 * balance card, a holdings stub (matches HoldingsDisplay layout), and
 * a transactions table stub.
 */
export default function Loading(): JSX.Element {
  return (
    <div role="status" aria-label="Loading address" className="p-4 max-w-6xl mx-auto">
      {/* Header strip: address + rank chip */}
      <div className="bg-[#2d2d2d] shadow rounded-lg p-6 mb-4">
        <div className="flex flex-col lg:flex-row lg:items-center justify-between pb-3 border-b border-gray-700 mb-6">
          <div className="flex items-center gap-4">
            <div className="w-10 h-10 rounded-full bg-[#ffa729]/20 animate-pulse" />
            <div>
              <div className="h-3 w-16 bg-gray-700/40 rounded mb-2 animate-pulse" />
              <div className="h-5 w-72 bg-[#ffa729]/20 rounded animate-pulse" />
            </div>
          </div>
          <div className="h-7 w-20 bg-[#ffa729]/20 rounded-xl animate-pulse mt-3 lg:mt-0" />
        </div>
        {/* Stats grid: balance + activity */}
        <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
          {[0, 1].map((i) => (
            <div key={i} className="border border-gray-700/40 rounded-lg p-4">
              <div className="h-3 w-24 bg-gray-700/40 rounded mb-3 animate-pulse" />
              <div className="h-7 w-40 bg-[#ffa729]/20 rounded animate-pulse" />
            </div>
          ))}
        </div>
      </div>

      {/* Holdings stub */}
      <div className="space-y-3 mb-6">
        <div className="h-5 w-32 bg-[#ffa729]/20 rounded animate-pulse" />
        <div className="rounded-xl border border-gray-700/40 overflow-hidden">
          {[0, 1, 2].map((i) => (
            <div key={i} className="px-4 py-3 border-b border-gray-700/40 last:border-b-0 flex items-center justify-between">
              <div className="h-4 w-48 bg-gray-700/40 rounded animate-pulse" />
              <div className="h-4 w-24 bg-gray-700/40 rounded animate-pulse" />
            </div>
          ))}
        </div>
      </div>

      {/* Transactions table stub */}
      <div className="space-y-3">
        <div className="h-5 w-36 bg-[#ffa729]/20 rounded animate-pulse" />
        <div className="rounded-xl border border-gray-700/40 overflow-hidden">
          {[0, 1, 2, 3, 4].map((i) => (
            <div key={i} className="px-4 py-3 border-b border-gray-700/40 last:border-b-0 flex items-center gap-4">
              <div className="h-4 w-20 bg-gray-700/40 rounded animate-pulse" />
              <div className="h-4 w-40 bg-gray-700/40 rounded animate-pulse hidden sm:block" />
              <div className="h-4 w-40 bg-gray-700/40 rounded animate-pulse hidden sm:block ml-auto" />
              <div className="h-4 w-16 bg-gray-700/40 rounded animate-pulse ml-auto sm:ml-0" />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
