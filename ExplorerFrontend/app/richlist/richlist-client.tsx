'use client';

import React from "react";
import Link from "next/link";
import Badge from "../components/Badge";
import { NATIVE_UNIT, formatTimestamp, toFixed } from "../lib/helpers";

interface RichlistEntry {
  id: string;
  balance: string | number;
  // Newer /richlist responses only; optional so old cached payloads
  // degrade to "-" instead of rendering NaN / broken badges.
  isContract?: boolean;
  firstSeen?: number;
  supplyPercent?: number;
}

// Percent of circulating supply, 2 decimals. Values that would round to
// "0.00%" but are non-zero surface as "<0.01%"; missing data renders "-".
function formatSupplyPercent(pct: number | undefined): string {
  if (typeof pct !== "number" || !Number.isFinite(pct)) return "-";
  if (pct > 0 && pct < 0.005) return "<0.01%";
  return `${pct.toFixed(2)}%`;
}

function typeBadge(isContract: boolean | undefined): React.ReactNode {
  if (typeof isContract !== "boolean") return "-";
  return isContract ? (
    <Badge variant="info">Contract</Badge>
  ) : (
    <Badge variant="neutral">Wallet</Badge>
  );
}

interface RichlistProps {
  richlist: RichlistEntry[];
}

export default function RichlistClient({ richlist }: RichlistProps): JSX.Element {

  const safeRichlist = richlist || [];

  const [windowWidth, setWindowWidth] = React.useState(
    typeof window !== "undefined" ? window.innerWidth : 0
  );

  React.useEffect(() => {
    const handleResize = (): void => setWindowWidth(window.innerWidth);
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  const renderMobileView = (): JSX.Element => (
    <div className="space-y-4">
      {safeRichlist.map((item, index) => (
        <div
          key={item.id}
          className="p-4 rounded-lg border border-border bg-card-gradient"
        >
          <div className="flex items-center mb-3">
            <div className="flex items-center text-text-secondary">
              {index === 0 && (
                <svg
                  xmlns="http://www.w3.org/2000/svg"
                  className="h-5 w-5 mr-2 text-yellow-500"
                  fill="none"
                  viewBox="0 0 24 24"
                  stroke="currentColor"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth={2}
                    d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                  />
                </svg>
              )}
              <span className="text-sm font-medium">Rank {index + 1}</span>
            </div>
          </div>
          <div className="space-y-2">
            <div>
              <span className="text-accent text-sm">Address:</span>
              <Link
                href={`/address/${item.id}`}
                className="ml-2 text-text-primary hover:text-accent text-sm break-all"
              >
                {item.id}
              </Link>
            </div>
            <div>
              <span className="text-accent text-sm">Type:</span>
              <span className="ml-2 text-text-primary text-sm">
                {typeBadge(item.isContract)}
              </span>
            </div>
            <div>
              <span className="text-accent text-sm">First Seen:</span>
              <span className="ml-2 text-text-primary text-sm">
                {item.firstSeen ? formatTimestamp(item.firstSeen) : "-"}
              </span>
            </div>
            <div>
              <span className="text-accent text-sm">Balance:</span>
              <span className="ml-2 text-text-primary text-sm">
                {toFixed(item.balance)} {NATIVE_UNIT}
              </span>
            </div>
            <div>
              <span className="text-accent text-sm">% Supply:</span>
              <span className="ml-2 text-text-primary text-sm">
                {formatSupplyPercent(item.supplyPercent)}
              </span>
            </div>
          </div>
        </div>
      ))}
    </div>
  );

  const renderDesktopView = (): JSX.Element => (
    <div className="overflow-x-auto">
      <table aria-label="Top addresses by balance" className="min-w-full">
        <thead>
          <tr className="border-b border-border">
            <th scope="col" className="table-header">
              Rank
            </th>
            <th scope="col" className="table-header">
              Address
            </th>
            <th scope="col" className="table-header">
              Type
            </th>
            <th scope="col" className="table-header">
              First Seen
            </th>
            <th scope="col" className="px-3 md:px-6 py-3 md:py-4 text-right text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted">
              Balance
            </th>
            <th scope="col" className="px-3 md:px-6 py-3 md:py-4 text-right text-[11px] font-medium uppercase tracking-[0.12em] text-text-muted">
              % Supply
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-border">
          {safeRichlist.map((item, index) => (
            <tr
              key={item.id}
              className="border-b border-border hover:bg-surface transition-colors"
            >
              <td className="px-3 md:px-6 py-3 md:py-4 whitespace-nowrap text-text-secondary">
                <div className="flex items-center">
                  {index === 0 && (
                    <svg
                      xmlns="http://www.w3.org/2000/svg"
                      className="h-5 w-5 mr-2 text-yellow-500"
                      fill="none"
                      viewBox="0 0 24 24"
                      stroke="currentColor"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z"
                      />
                    </svg>
                  )}
                  {index + 1}
                </div>
              </td>
              <td className="px-3 md:px-6 py-3 md:py-4 whitespace-nowrap">
                <Link
                  href={`/address/${item.id}`}
                  className="text-accent hover:text-accent-hover transition-colors text-sm"
                >
                  {item.id}
                </Link>
              </td>
              <td className="px-3 md:px-6 py-3 md:py-4 whitespace-nowrap text-text-secondary text-sm">
                {typeBadge(item.isContract)}
              </td>
              <td className="px-3 md:px-6 py-3 md:py-4 whitespace-nowrap text-text-secondary text-sm">
                {item.firstSeen ? formatTimestamp(item.firstSeen) : "-"}
              </td>
              <td className="px-3 md:px-6 py-3 md:py-4 whitespace-nowrap text-right text-text-secondary text-sm">
                {toFixed(item.balance)} {NATIVE_UNIT}
              </td>
              <td className="px-3 md:px-6 py-3 md:py-4 whitespace-nowrap text-right text-text-secondary text-sm">
                {formatSupplyPercent(item.supplyPercent)}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );

  return (
    <div className="min-h-screen">
      <div className="page-content py-4 md:py-8">
        <div className="mb-6 md:mb-8">
          <h1 className="section-title">Richlist</h1>
          <p className="text-sm md:text-base text-text-secondary mt-2">
            Top 50 {NATIVE_UNIT} holders by balance
          </p>
        </div>

        <div className="card">
          {windowWidth < 768 ? renderMobileView() : renderDesktopView()}
        </div>

        <div className="mt-4 md:mt-6 text-center text-xs md:text-sm text-text-secondary">
          Note: This list is updated every block
        </div>
      </div>
    </div>
  );
}
