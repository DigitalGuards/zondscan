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
    <div role="status" aria-label="Loading contracts" className="py-4 sm:py-6 lg:py-8">
      <div className="mb-6">
        <h1 className="section-title mb-2">Smart Contracts</h1>
        <div className="flex gap-2">
          {[0, 1, 2].map((i) => (
            <div key={i} className="h-9 w-28 skeleton rounded-lg" />
          ))}
        </div>
      </div>

      <div className="p-4 space-y-4">
        {Array.from({ length: 5 }).map((_, i) => (
          <div key={i} className="h-16 skeleton" />
        ))}
      </div>
    </div>
  );
}
