export default function Loading(): JSX.Element {
  return (
    <div role="status" aria-label="Loading" className="p-8">
      <div className="relative overflow-hidden rounded-2xl
                                        border border-border">
        <div className="p-6">
          {/* Header Skeleton */}
          <div className="flex items-center justify-between mb-8 pb-6 border-b border-border">
            <div className="flex items-center">
              <div className="w-8 h-8 skeleton rounded-lg mr-3"></div>
              <div>
                <div className="h-8 w-48 skeleton"></div>
                <div className="h-4 w-32 skeleton mt-2"></div>
              </div>
            </div>
          </div>

          {/* Content Skeleton */}
          <div className="space-y-6">
            {/* Basic Info */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {[...Array(2)].map((_, i) => (
                <div key={i}>
                  <div className="h-4 w-24 skeleton mb-2"></div>
                  <div className="h-6 w-full skeleton"></div>
                </div>
              ))}
            </div>

            {/* Gas Info */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
              {[...Array(3)].map((_, i) => (
                <div key={i}>
                  <div className="h-4 w-24 skeleton mb-2"></div>
                  <div className="h-6 w-full skeleton"></div>
                </div>
              ))}
            </div>

            {/* Transactions Skeleton */}
            <div>
              <div className="h-6 w-32 skeleton mb-4"></div>
              <div className="space-y-2">
                {[...Array(3)].map((_, i) => (
                  <div key={i} className="p-4 rounded-lg bg-surface-2 animate-pulse h-24"></div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
