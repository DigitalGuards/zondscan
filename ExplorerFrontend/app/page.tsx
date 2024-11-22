import * as React from 'react';
import axios from "axios";
import { formatNumber, formatNumberWithCommas, toFixed } from "./lib/helpers";
import config from "../config.js"
import SearchBar from "./components/SearchBar"

interface StatsData {
  value: string;
  isLoading: boolean;
  error: boolean;
}

interface DashboardData {
  walletCount: StatsData;
  volume: StatsData;
  blockHeight: StatsData;
  totalTransactions: StatsData;
  syncing: boolean;
  dataInitialized: boolean;
}

export default async function Home() {
  let data: DashboardData = {
    walletCount: { value: "0", isLoading: true, error: false },
    volume: { value: "0", isLoading: true, error: false },
    blockHeight: { value: "0", isLoading: true, error: false },
    totalTransactions: { value: "0", isLoading: true, error: false },
    syncing: true,
    dataInitialized: false
  };

  try {
    if (config.handlerUrl) {
      const [overviewResponse, latestBlockResponse, txsResponse] = await Promise.allSettled([
        axios.get(config.handlerUrl + "/overview"),
        axios.get(config.handlerUrl + "/latestblock"),
        axios.get(config.handlerUrl + "/txs?page=1")
      ]);

      // Handle overview response
      if (overviewResponse.status === 'fulfilled') {
        data.walletCount.value = overviewResponse.value.data.countwallets?.toString() || "0";
        data.volume.value = overviewResponse.value.data.volume?.toString() || "0";
        data.syncing = overviewResponse.value.data.status?.syncing ?? true;
        data.dataInitialized = overviewResponse.value.data.status?.dataInitialized ?? false;
      } else {
        data.walletCount.error = true;
        data.volume.error = true;
      }

      // Handle latest block response
      if (latestBlockResponse.status === 'fulfilled') {
        // Updated to use 'number' instead of 'height'
        data.blockHeight.value = latestBlockResponse.value.data.response?.[0]?.result?.number?.toString() || "0";
      } else {
        data.blockHeight.error = true;
      }

      // Handle transactions response
      if (txsResponse.status === 'fulfilled') {
        data.totalTransactions.value = txsResponse.value.data.total?.toString() || "0";
      } else {
        data.totalTransactions.error = true;
      }

      // Update loading states
      data.walletCount.isLoading = false;
      data.volume.isLoading = false;
      data.blockHeight.isLoading = false;
      data.totalTransactions.isLoading = false;
    }
  } catch (error) {
    console.error("Failed to fetch overview data:", error);
  }

  const stats = [
    {
      data: formatNumberWithCommas(data.walletCount.value),
      title: "Network Bagholder Count",
      loading: data.walletCount.isLoading,
      error: data.walletCount.error,
      icon: (
        <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 mb-2 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M17 9V7a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2m2 4h10a2 2 0 002-2v-6a2 2 0 00-2-2H9a2 2 0 00-2 2v6a2 2 0 002 2zm7-5a2 2 0 11-4 0 2 2 0 014 0z" />
        </svg>
      )
    },
    {
      data: toFixed(parseFloat(data.volume.value)) + " QRL",
      title: "Daily Transactions Volume",
      loading: data.volume.isLoading,
      error: data.volume.error,
      icon: (
        <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 mb-2 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
        </svg>
      )
    },
    {
      data: formatNumberWithCommas(data.blockHeight.value),
      title: "Block Height",
      loading: data.blockHeight.isLoading,
      error: data.blockHeight.error,
      icon: (
        <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 mb-2 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10" />
        </svg>
      )
    },
    {
      data: formatNumberWithCommas(data.totalTransactions.value),
      title: "Total Transactions",
      loading: data.totalTransactions.isLoading,
      error: data.totalTransactions.error,
      icon: (
        <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 mb-2 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 7h12m0 0l-4-4m4 4l-4 4m0 6H4m0 0l4 4m-4-4l4-4" />
        </svg>
      )
    }
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
        
        <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4">
          {stats.map((item, idx) => (
            <div key={idx} 
                className={`relative overflow-hidden rounded-2xl 
                         bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                         border border-[#3d3d3d] shadow-xl
                         hover:border-[#ffa729] transition-all duration-300
                         group ${!data.dataInitialized ? 'opacity-50' : ''}`}>
              <div className="relative p-6 text-center min-h-[160px] flex flex-col justify-center">
                {item.loading ? (
                  <div className="flex flex-col items-center justify-center space-y-2">
                    <div className="w-32 h-8 bg-gray-700/50 rounded animate-pulse"></div>
                    <div className="w-24 h-4 bg-gray-700/50 rounded animate-pulse"></div>
                  </div>
                ) : item.error ? (
                  <div className="flex flex-col items-center justify-center text-red-400">
                    <svg xmlns="http://www.w3.org/2000/svg" className="h-8 w-8 mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    <span className="text-sm">Failed to load data</span>
                  </div>
                ) : (
                  <>
                    <div className="flex justify-center">{item.icon}</div>
                    <h4 className="text-3xl font-bold mb-3 text-[#ffa729] 
                                group-hover:scale-110 transition-transform duration-300">
                      {item.data}
                    </h4>
                    <p className="text-sm text-gray-300 font-medium">
                      {item.title}
                    </p>
                  </>
                )}
              </div>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
