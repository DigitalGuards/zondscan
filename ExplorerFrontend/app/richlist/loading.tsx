/**
 * Route-level skeleton for /richlist. The page is server-rendered and
 * awaits the /richlist fetch (force-dynamic), so without a fallback the
 * page is briefly blank between click and data.
 *
 * Shape mirrors RichlistClient: header + subtitle and the three-column
 * (rank / address / balance) table card.
 */
import { NATIVE_UNIT } from '../lib/helpers';

export default function Loading(): JSX.Element {
  return (
    <div role="status" aria-label="Loading rich list" className="min-h-screen">
      <div className="page-content py-4 md:py-8">
        <div className="mb-6 md:mb-8">
          <h1 className="section-title">Richlist</h1>
          <p className="text-sm md:text-base text-text-secondary mt-2">
            Top 50 {NATIVE_UNIT} holders by balance
          </p>
        </div>

        <div className="card overflow-hidden">
          <div className="overflow-x-auto">
            <table className="min-w-full">
              <thead>
                <tr className="border-b border-border">
                  <th className="table-header">Rank</th>
                  <th className="table-header">Address</th>
                  <th className="px-3 md:px-6 py-3 md:py-4 text-right text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted">Balance</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {Array.from({ length: 10 }).map((_, i) => (
                  <tr key={i} className="border-b border-border">
                    <td className="px-3 md:px-6 py-3 md:py-4 whitespace-nowrap"><div className="h-4 w-8 skeleton" /></td>
                    <td className="px-3 md:px-6 py-3 md:py-4 whitespace-nowrap"><div className="h-4 w-64 max-w-full skeleton" /></td>
                    <td className="px-3 md:px-6 py-3 md:py-4 whitespace-nowrap"><div className="h-4 w-24 skeleton ml-auto" /></td>
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
