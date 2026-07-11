/**
 * Route-level skeleton for /epochs/<page>. The page is server-rendered
 * and awaits the beacon-chain epochs fetch (force-dynamic), so without a
 * fallback the page is briefly blank between click and data.
 *
 * Shape mirrors EpochsClient: header + search bar stub + the six-column
 * epochs table so the transition reads as "the data is filling in".
 */
export default function Loading(): JSX.Element {
  return (
    <div role="status" aria-label="Loading epochs" className="page-content py-4 sm:py-6 lg:py-8">
      <h1 className="section-title mb-4">Epochs</h1>

      <div className="mb-6">
        <div className="h-11 w-full bg-surface border border-border rounded-lg animate-pulse" />
      </div>

      <div className="card-simple overflow-hidden mb-6">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-border">
                <th className="text-left px-4 py-3 text-[11px] font-normal text-text-muted uppercase tracking-wider">Epoch</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-text-muted uppercase tracking-wider">Time</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-text-muted uppercase tracking-wider">Status</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-text-muted uppercase tracking-wider hidden sm:table-cell">Validators</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-text-muted uppercase tracking-wider hidden sm:table-cell">Active</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-text-muted uppercase tracking-wider hidden md:table-cell">Total Staked</th>
              </tr>
            </thead>
            <tbody>
              {Array.from({ length: 15 }).map((_, i) => (
                <tr key={i} className="border-b border-border last:border-b-0">
                  <td className="px-4 py-3"><div className="h-4 w-12 skeleton" /></td>
                  <td className="px-4 py-3"><div className="h-4 w-16 skeleton" /></td>
                  <td className="px-4 py-3"><div className="h-4 w-16 skeleton" /></td>
                  <td className="px-4 py-3 hidden sm:table-cell"><div className="h-4 w-10 skeleton" /></td>
                  <td className="px-4 py-3 hidden sm:table-cell"><div className="h-4 w-10 skeleton" /></td>
                  <td className="px-4 py-3 hidden md:table-cell"><div className="h-4 w-24 skeleton" /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
