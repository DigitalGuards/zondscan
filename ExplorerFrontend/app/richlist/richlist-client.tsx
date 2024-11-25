'use client';

import React from "react";
import Link from "next/link";
import config from "../../config";
import { toFixed } from "../lib/helpers";

interface RichlistProps {
  richlist: any[];
}

function decodeBase64ToHexadecimal(rawData: string): string {
  const decoded = atob(rawData);
  let hex = '';
  for (let i = 0; i < decoded.length; i++) {
    const byte = decoded.charCodeAt(i).toString(16);
    hex += byte.length === 1 ? '0' + byte : byte;
  }
  return hex;
}

export default function RichlistClient({ richlist }: RichlistProps) {
  const data = richlist.map((item: any, index: any) => (
    <tr key={item.id} className="border-b border-[#3d3d3d] hover:bg-[rgba(255,167,41,0.05)] transition-colors">
      <td className="px-6 py-4 whitespace-nowrap text-gray-300">
        <div className="flex items-center">
          {index === 0 && (
            <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 mr-2 text-yellow-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
          )}
          {index + 1}
        </div>
      </td>
      <td className="px-6 py-4 whitespace-nowrap">
        <Link 
          href={config.siteUrl + "/address/" + "0x" + decodeBase64ToHexadecimal(item.id)}
          className="text-[#ffa729] hover:text-[#ffb954] transition-colors"
        >
          0x{decodeBase64ToHexadecimal(item.id)}
        </Link>
      </td>
      <td className="px-6 py-4 whitespace-nowrap text-right text-gray-300">
        {toFixed(item.balance)} QRL
      </td>
    </tr>
  ));

  return (
    <div className="min-h-screen">
      <div className="max-w-[1200px] mx-auto p-8">
        <div className="mb-8">
          <h1 className="text-2xl font-bold text-[#ffa729]">Richlist</h1>
          <p className="text-gray-400 mt-2">Top 50 QRL holders by balance</p>
        </div>

        <div className="overflow-hidden rounded-lg border border-[#3d3d3d] bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] shadow-xl">
          <div className="overflow-x-auto">
            <table className="min-w-full">
              <thead>
                <tr className="border-b border-[#3d3d3d]">
                  <th className="px-6 py-4 text-left text-sm font-medium text-[#ffa729]">Rank</th>
                  <th className="px-6 py-4 text-left text-sm font-medium text-[#ffa729]">Address</th>
                  <th className="px-6 py-4 text-right text-sm font-medium text-[#ffa729]">Balance</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[#3d3d3d]">
                {data}
              </tbody>
            </table>
          </div>
        </div>

        <div className="mt-6 text-center text-sm text-gray-400">
          Note: This list is updated every block
        </div>
      </div>
    </div>
  );
}
