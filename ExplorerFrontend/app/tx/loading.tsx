/* Full-width detail skeleton: the tx page fills page-content, so the
   loading state must too (a narrow card here reflowed ~800px on load). */
export default function Loading(): JSX.Element {
  return (
    <div role="status" aria-label="Loading" className="detail-content">
      <div className="skeleton h-4 w-40 mb-6" />
      <div className="card p-4 sm:p-6">
        <div className="flex items-center mb-6 gap-2">
          <div className="skeleton w-6 h-6 rounded-full" />
          <div className="skeleton h-7 w-48" />
        </div>

        <div className="space-y-4">
          {[...Array(7)].map((_, i) => (
            <div key={i} className="border-b border-border/70 last:border-b-0 pb-4">
              <div className="skeleton h-3.5 w-24 mb-2" />
              <div className="skeleton h-5 w-full max-w-2xl" />
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
