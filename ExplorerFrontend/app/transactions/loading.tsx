const COLS = ['w-28', 'w-20', 'w-24', 'w-24', 'w-16', 'w-16'];

/* Mirrors the transactions list table so navigation doesn't swap layouts
   when data lands. */
export default function Loading(): JSX.Element {
  return (
    <div role="status" aria-label="Loading" className="py-4 sm:py-6 lg:py-8">
      <h1 className="section-title mb-4">Transactions</h1>
      <div className="mb-6">
        <div className="skeleton h-[60px] rounded-2xl" />
      </div>
      <div className="card-simple overflow-hidden mb-6">
        <div className="border-b border-border px-4 py-3 flex gap-6">
          {COLS.map((w, i) => (
            <div key={i} className={`skeleton h-3 ${w} hidden sm:block`} />
          ))}
        </div>
        {Array.from({ length: 10 }).map((_, i) => (
          <div key={i} className="border-b border-border last:border-b-0 px-4 py-3 flex gap-6 items-center">
            {COLS.map((w, j) => (
              <div key={j} className={`skeleton h-4 ${w} ${j > 1 ? 'hidden sm:block' : ''}`} />
            ))}
          </div>
        ))}
      </div>
    </div>
  );
}
