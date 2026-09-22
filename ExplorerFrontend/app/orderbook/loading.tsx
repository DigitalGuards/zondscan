export default function OrderBookLoading(): JSX.Element {
  return (
    <div
      className="page-content py-4 sm:py-6 lg:py-8"
      role="status"
      aria-label="Loading order book"
    >
      <span className="sr-only">Loading market data</span>
      <div className="h-8 w-64 skeleton rounded-lg mb-3" />
      <div className="h-4 w-96 max-w-full skeleton rounded mb-6" />
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 mb-4">
        {Array.from({ length: 4 }, (_, index) => (
          <div key={index} className="card h-24 skeleton" />
        ))}
      </div>
      <div className="grid gap-4 xl:grid-cols-[minmax(0,1.5fr)_minmax(0,1fr)]" aria-hidden="true">
        <div className="card h-96 skeleton" />
        <div className="card h-96 skeleton" />
      </div>
    </div>
  );
}
