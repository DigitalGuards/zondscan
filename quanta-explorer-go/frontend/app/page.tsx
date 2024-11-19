import * as React from 'react';
import axios from "axios";
import { formatNumber, formatNumberWithCommas } from "./lib/helpers";
import config from "../config.js"
import { toFixed } from "./lib/helpers.js"
import SearchBar from "./components/SearchBar"

export default async function Home() {
  let data = {
    marketCapUSD: "0",
    walletCount: "0",
    volume: "0",
  };

  try {
    if (config.handlerUrl) {
      const response = await axios.get(config.handlerUrl + "/overview");
      data = {
        marketCapUSD: formatNumber(response.data.marketcap),
        walletCount: response.data.countwallets,
        volume: response.data.volume,
      };
    }
  } catch (error) {
    console.error("Failed to fetch overview data:", error);
  }

  const stats = [
    {
      data: data.walletCount,
      title: "Wallet count"
    },
    {
      data: toFixed(data.volume) + " QRL",
      title: "Daily Transactions Volume"
    },
  ]

  return (
    <div className="min-h-screen">
      <div className="max-w-[1200px] mx-auto p-8">
        <div className="mb-10">
          <SearchBar />
        </div>
        
        <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
          {stats.map((item, idx) => (
            <div key={idx} 
                className="relative overflow-hidden rounded-2xl 
                         bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                         border border-[#3d3d3d] shadow-xl
                         hover:border-[#ffa729] transition-all duration-300
                         group">
              <div className="absolute inset-0 bg-[url('/circuit-board.svg')] opacity-5"></div>
              <div className="relative p-10 text-center min-h-[200px] flex flex-col justify-center">
                <h4 className="text-5xl font-bold mb-4 text-[#ffa729] 
                             group-hover:scale-110 transition-transform duration-300">
                  {item.data}
                </h4>
                <p className="text-lg text-gray-300 font-medium">
                  {item.title}
                </p>
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
};
