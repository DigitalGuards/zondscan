'use client';

import React from "react";
import Link from "next/link";
import config from "../../config";
import { toFixed } from "../lib/helpers";

interface RichlistProps {
  richlist: any[];
}

export default function RichlistClient({ richlist }: RichlistProps) {

  const safeRichlist = richlist || [];

  const [windowWidth, setWindowWidth] = React.useState(
    typeof window !== "undefined" ? window.innerWidth : 0
  );

  React.useEffect(() => {
    const handleResize = () => setWindowWidth(window.innerWidth);
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  const renderMobileView = () => (
    <div className="space-y-4">
      {safeRichlist.map((item: any, index: number) => (
        <div
          key={item.id}
          className="p-4 rounded-lg border border-[#3d3d3d] bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]"
        >
          <div className="flex items-center mb-3">
            <div className="flex items-center text-gray-300">
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
              <span className="text-[#ffa729] text-sm">Address:</span>
              <Link
                href={`/address/${item.id}`}
                className="ml-2 text-white hover:text-[#ffa729] text-sm break-all"
              >
                {item.id}
              </Link>
            </div>
            <div>
              <span className="text-[#ffa729] text-sm">Balance:</span>
              <span className="ml-2 text-white text-sm">
                {toFixed(item.balance)} QRL
              </span>
            </div>
          </div>
        </div>
      ))}
    </div>
  );

  const renderDesktopView = () => (
    <div className="overflow-x-auto">
      <table className="min-w-full">
        <thead>
          <tr className="border-b border-[#3d3d3d]">
            <th className="px-3 md:px-6 py-3 md:py-4 text-left text-xs md:text-sm font-medium text-[#ffa729]">
              Rank
            </th>
            <th className="px-3 md:px-6 py-3 md:py-4 text-left text-xs md:text-sm font-medium text-[#ffa729]">
              Address
            </th>
            <th className="px-3 md:px-6 py-3 md:py-4 text-right text-xs md:text-sm font-medium text-[#ffa729]">
              Balance
            </th>
          </tr>
        </thead>
        <tbody className="divide-y divide-[#3d3d3d]">
          {safeRichlist.map((item: any, index: number) => (
            <tr
              key={item.id}
              className="border-b border-[#3d3d3d] hover:bg-[rgba(255,167,41,0.05)] transition-colors"
            >
              <td className="px-3 md:px-6 py-3 md:py-4 whitespace-nowrap text-gray-300">
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
                  className="text-[#ffa729] hover:text-[#ffb954] transition-colors text-sm"
                >
                  {item.id}
                </Link>
              </td>
              <td className="px-3 md:px-6 py-3 md:py-4 whitespace-nowrap text-right text-gray-300 text-sm">
                {toFixed(item.balance)} QRL
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );

  return (
    <div className="min-h-screen">
      <div className="max-w-[1200px] mx-auto p-4 md:p-8">
        <div className="mb-6 md:mb-8">
          <h1 className="text-xl md:text-2xl font-bold text-[#ffa729]">Richlist</h1>
          <p className="text-sm md:text-base text-gray-400 mt-2">
            Top 50 QRL holders by balance
          </p>
        </div>

        <div className="rounded-lg border border-[#3d3d3d] bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] shadow-xl">
          {windowWidth < 768 ? renderMobileView() : renderDesktopView()}
        </div>

        <div className="mt-4 md:mt-6 text-center text-xs md:text-sm text-gray-400">
          Note: This list is updated every block
        </div>
      </div>
    </div>
  );
}
