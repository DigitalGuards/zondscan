/**
 * Route-level skeleton for /contracts. The page is server-rendered and
 * awaits the /contracts fetch, so without a fallback the page is briefly
 * blank between click and data.
 *
 * Shape mirrors ContractsClient: header + tab strip stub and a stack of
 * contract row placeholders.
 */
export default function Loading(): JSX.Element {
  return (
    <div role="status" aria-label="Loading contracts" className="max-w-7xl mx-auto p-4 sm:p-6 lg:p-8">
      <div className="mb-6">
        <h1 className="text-xl sm:text-2xl font-bold text-[#ffa729] mb-2">Smart Contracts</h1>
        <div className="flex gap-2">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-9 w-28 bg-[#2d2d2d] rounded-lg animate-pulse" />
          ))}
        </div>
      </div>

      <div className="p-4 space-y-4">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="h-16 bg-gray-700/30 rounded animate-pulse" />
        ))}
      </div>
    </div>
  );
}
