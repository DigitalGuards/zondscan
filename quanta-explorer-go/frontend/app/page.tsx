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
    syncing: true,
    dataInitialized: false
  };

  try {
    if (config.handlerUrl) {
      const response = await axios.get(config.handlerUrl + "/overview");
      data = {
        marketCapUSD: formatNumber(response.data.marketcap),
        walletCount: response.data.countwallets,
        volume: response.data.volume,
        syncing: response.data.status?.syncing ?? true,
        dataInitialized: response.data.status?.dataInitialized ?? false
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
        
        {!data.dataInitialized && (
          <div className="mb-8 p-4 rounded-lg bg-yellow-500/10 border border-yellow-500/20 text-yellow-500">
            <div className="flex items-center">
              <svg className="animate-spin -ml-1 mr-3 h-5 w-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
              </svg>
              <span>Initializing explorer data... This may take a few minutes.</span>
            </div>
          </div>
        )}
        
        <div className="grid grid-cols-1 md:grid-cols-2 gap-8">
          {stats.map((item, idx) => (
            <div key={idx} 
                className={`relative overflow-hidden rounded-2xl 
                         bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                         border border-[#3d3d3d] shadow-xl
                         hover:border-[#ffa729] transition-all duration-300
                         group ${!data.dataInitialized ? 'opacity-50' : ''}`}>
              <div className="absolute inset-0 bg-[url('/circuit-board.svg')] opacity-5"></div>
              <div className="relative p-10 text-center min-h-[200px] flex flex-col justify-center">
                {data.dataInitialized ? (
                  <>
                    <h4 className="text-5xl font-bold mb-4 text-[#ffa729] 
                                group-hover:scale-110 transition-transform duration-300">
                      {item.data}
                    </h4>
                    <p className="text-lg text-gray-300 font-medium">
                      {item.title}
                    </p>
                  </>
                ) : (
                  <div className="flex flex-col items-center justify-center space-y-2">
                    <div className="w-32 h-8 bg-gray-700/50 rounded animate-pulse"></div>
                    <div className="w-24 h-4 bg-gray-700/50 rounded animate-pulse"></div>
                  </div>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
