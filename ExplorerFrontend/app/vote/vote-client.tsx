'use client';

import React from "react";
import { decodeBase64ToHexadecimal } from "../lib/helpers";

interface VoteClientProps {
  richlist: any[];
}

export default function VoteClient({ richlist }: VoteClientProps) {
  const data = richlist.map((item, index) => (
    <tr key={item.id} className="border-t hover:bg-gray-100">
      <td className="px-4 py-2">{index + 1}</td>
      <td className="px-4 py-2">0x{decodeBase64ToHexadecimal(item.id)}</td>
      <td className="px-4 py-2">{item.balance}</td>
    </tr>
  ));

  return (
    <div className="overflow-x-auto bg-gray-100 min-h-screen flex items-center justify-center">
      <div className="bg-white p-6 rounded-lg shadow-md w-full lg:w-5/6">
        <h1 className="text-5xl font-bold mb-8 border-b pb-4">Richlist</h1>
        <table className="min-w-max w-full table-auto">
          <thead>
            <tr className="bg-gray-200 text-gray-600 uppercase text-sm leading-normal">
              <th className="py-3 px-6 text-left">Rank</th>
              <th className="py-3 px-6 text-left">Address</th>
              <th className="py-3 px-6 text-right">Amount</th>
            </tr>
          </thead>
          <tbody className="text-gray-600 text-sm font-light">
            {data}
          </tbody>
        </table>
      </div>
    </div>
  );
}
