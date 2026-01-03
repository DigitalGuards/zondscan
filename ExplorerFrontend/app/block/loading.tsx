export default function Loading(): JSX.Element {
  return (
    <div className="p-8">
      <div className="relative overflow-hidden rounded-2xl 
                    bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                    border border-[#3d3d3d] shadow-xl">
        <div className="p-6">
          {/* Header Skeleton */}
          <div className="flex items-center justify-between mb-8 pb-6 border-b border-gray-700">
            <div className="flex items-center">
              <div className="w-8 h-8 bg-gray-700 rounded-lg animate-pulse mr-3"></div>
              <div>
                <div className="h-8 w-48 bg-gray-700 rounded animate-pulse"></div>
                <div className="h-4 w-32 bg-gray-700 rounded animate-pulse mt-2"></div>
              </div>
            </div>
          </div>

          {/* Content Skeleton */}
          <div className="space-y-6">
            {/* Basic Info */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {[...Array(2)].map((_, i) => (
                <div key={i}>
                  <div className="h-4 w-24 bg-gray-700 rounded animate-pulse mb-2"></div>
                  <div className="h-6 w-full bg-gray-700 rounded animate-pulse"></div>
                </div>
              ))}
            </div>

            {/* Gas Info */}
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
              {[...Array(3)].map((_, i) => (
                <div key={i}>
                  <div className="h-4 w-24 bg-gray-700 rounded animate-pulse mb-2"></div>
                  <div className="h-6 w-full bg-gray-700 rounded animate-pulse"></div>
                </div>
              ))}
            </div>

            {/* Transactions Skeleton */}
            <div>
              <div className="h-6 w-32 bg-gray-700 rounded animate-pulse mb-4"></div>
              <div className="space-y-2">
                {[...Array(3)].map((_, i) => (
                  <div key={i} className="p-4 rounded-lg bg-gray-700 animate-pulse h-24"></div>
                ))}
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
