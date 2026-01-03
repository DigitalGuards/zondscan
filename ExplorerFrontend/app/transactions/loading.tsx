export default function Loading(): JSX.Element {
  return (
    <div className="p-8">
      <h1 className="section-title mb-6">Transactions</h1>
      <div className="space-y-4">
        {[...Array(5)].map((_, i) => (
          <div
            key={i}
            className="card p-6 animate-pulse"
          >
            <div className="flex flex-col md:flex-row items-center">
              <div className="w-48 flex flex-col items-center">
                <div className="skeleton w-8 h-8 rounded-lg mb-2"></div>
                <div className="skeleton h-4 w-20"></div>
              </div>
              <div className="flex-1 md:ml-8 space-y-2">
                <div className="skeleton h-6 w-32"></div>
                <div className="skeleton h-4 w-full"></div>
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}
