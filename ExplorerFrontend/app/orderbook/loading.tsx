export default function OrderBookLoading(): JSX.Element {
  return (
    <div className="page-content py-4 sm:py-6 lg:py-8" aria-label="Loading order book">
      <div className="h-8 w-64 skeleton rounded-lg mb-3" />
      <div className="h-4 w-96 max-w-full skeleton rounded mb-6" />
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 mb-4">
        {Array.from({ length: 4 }, (_, index) => (
          <div key={index} className="card h-24 skeleton" />
        ))}
      </div>
      <div className="card aspect-video skeleton" />
    </div>
  );
}
