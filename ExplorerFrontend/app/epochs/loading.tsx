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
    <div role="status" aria-label="Loading epochs" className="p-4 sm:p-8 max-w-6xl mx-auto">
      <h1 className="text-xl sm:text-2xl font-bold mb-4 text-[#ffa729]">Epochs</h1>

      <div className="mb-6">
        <div className="h-11 w-full bg-[#1e1e1e] border border-[#2a2a2a] rounded-lg animate-pulse" />
      </div>

      <div className="rounded-xl bg-[#1e1e1e] border border-[#2a2a2a] overflow-hidden mb-6">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="border-b border-[#2a2a2a]">
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Epoch</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Time</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider">Status</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden sm:table-cell">Validators</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden sm:table-cell">Active</th>
                <th className="text-left px-4 py-3 text-[11px] font-normal text-gray-600 uppercase tracking-wider hidden md:table-cell">Total Staked</th>
              </tr>
            </thead>
            <tbody>
              {Array.from({ length: 15 }).map((_, i) => (
                <tr key={i} className="border-b border-[#2a2a2a] last:border-b-0">
                  <td className="px-4 py-3"><div className="h-4 w-12 bg-[#2a2a2a] rounded animate-pulse" /></td>
                  <td className="px-4 py-3"><div className="h-4 w-16 bg-[#2a2a2a] rounded animate-pulse" /></td>
                  <td className="px-4 py-3"><div className="h-4 w-16 bg-[#2a2a2a] rounded animate-pulse" /></td>
                  <td className="px-4 py-3 hidden sm:table-cell"><div className="h-4 w-10 bg-[#2a2a2a] rounded animate-pulse" /></td>
                  <td className="px-4 py-3 hidden sm:table-cell"><div className="h-4 w-10 bg-[#2a2a2a] rounded animate-pulse" /></td>
                  <td className="px-4 py-3 hidden md:table-cell"><div className="h-4 w-24 bg-[#2a2a2a] rounded animate-pulse" /></td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
