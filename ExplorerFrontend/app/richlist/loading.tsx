/**
 * Route-level skeleton for /richlist. The page is server-rendered and
 * awaits the /richlist fetch (force-dynamic), so without a fallback the
 * page is briefly blank between click and data.
 *
 * Shape mirrors RichlistClient: header + subtitle and the three-column
 * (rank / address / balance) table card.
 */
export default function Loading(): JSX.Element {
  return (
    <div role="status" aria-label="Loading rich list" className="min-h-screen">
      <div className="max-w-[1200px] mx-auto p-4 md:p-8">
        <div className="mb-6 md:mb-8">
          <h1 className="text-xl md:text-2xl font-bold text-[#ffa729]">Richlist</h1>
          <p className="text-sm md:text-base text-gray-400 mt-2">
            Top 50 QRL holders by balance
          </p>
        </div>

        <div className="rounded-lg border border-[#3d3d3d] bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] shadow-xl overflow-hidden">
          <div className="overflow-x-auto">
            <table className="min-w-full">
              <thead>
                <tr className="border-b border-[#3d3d3d]">
                  <th className="px-3 md:px-6 py-3 md:py-4 text-left text-xs md:text-sm font-medium text-[#ffa729]">Rank</th>
                  <th className="px-3 md:px-6 py-3 md:py-4 text-left text-xs md:text-sm font-medium text-[#ffa729]">Address</th>
                  <th className="px-3 md:px-6 py-3 md:py-4 text-right text-xs md:text-sm font-medium text-[#ffa729]">Balance</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[#3d3d3d]">
                {Array.from({ length: 10 }).map((_, i) => (
                  <tr key={i} className="border-b border-[#3d3d3d]">
                    <td className="px-3 md:px-6 py-3 md:py-4 whitespace-nowrap"><div className="h-4 w-8 bg-[#3d3d3d] rounded animate-pulse" /></td>
                    <td className="px-3 md:px-6 py-3 md:py-4 whitespace-nowrap"><div className="h-4 w-64 max-w-full bg-[#3d3d3d] rounded animate-pulse" /></td>
                    <td className="px-3 md:px-6 py-3 md:py-4 whitespace-nowrap"><div className="h-4 w-24 bg-[#3d3d3d] rounded animate-pulse ml-auto" /></td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      </div>
    </div>
  );
}
