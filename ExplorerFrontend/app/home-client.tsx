'use client';

import * as React from 'react';
import axios from "axios";
import { formatNumber, formatNumberWithCommas, toFixed } from "./lib/helpers";
import config from "../config.js"
import SearchBar from "./components/SearchBar"
import Charts from "./components/Charts"

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
  marketCap: StatsData;
  currentPrice: StatsData;
  validatorCount: StatsData;
  contractCount: StatsData;
  syncing: boolean;
  dataInitialized: boolean;
}

export default function HomeClient() {
  const [data, setData] = React.useState<DashboardData>({
    walletCount: { value: "0", isLoading: true, error: false },
    volume: { value: "0", isLoading: true, error: false },
    blockHeight: { value: "0", isLoading: true, error: false },
    totalTransactions: { value: "0", isLoading: true, error: false },
    marketCap: { value: "0", isLoading: true, error: false },
    currentPrice: { value: "0", isLoading: true, error: false },
    validatorCount: { value: "0", isLoading: true, error: false },
    contractCount: { value: "0", isLoading: true, error: false },
    syncing: true,
    dataInitialized: false
  });

  React.useEffect(() => {
    async function fetchData() {
      try {
        if (config.handlerUrl) {
          const [overviewResponse, latestBlockResponse, txsResponse] = await Promise.allSettled([
            axios.get(config.handlerUrl + "/overview"),
            axios.get(config.handlerUrl + "/latestblock"),
            axios.get(config.handlerUrl + "/txs?page=1")
          ]);

          setData(prevData => {
            const newData = { ...prevData };

            // Handle overview response
            if (overviewResponse.status === 'fulfilled') {
              newData.walletCount = { value: overviewResponse.value.data.countwallets.toString(), isLoading: false, error: false };
              newData.volume = { value: overviewResponse.value.data.volume.toString(), isLoading: false, error: false };
              newData.marketCap = { value: overviewResponse.value.data.marketcap.toString(), isLoading: false, error: false };
              newData.currentPrice = { value: overviewResponse.value.data.currentPrice.toString(), isLoading: false, error: false };
              newData.validatorCount = { value: overviewResponse.value.data.validatorCount.toString(), isLoading: false, error: false };
              newData.contractCount = { value: overviewResponse.value.data.contractCount.toString(), isLoading: false, error: false };
              newData.syncing = overviewResponse.value.data.status.syncing;
              newData.dataInitialized = overviewResponse.value.data.status.dataInitialized;
            } else {
              newData.walletCount = { value: "0", isLoading: false, error: true };
              newData.volume = { value: "0", isLoading: false, error: true };
              newData.marketCap = { value: "0", isLoading: false, error: true };
              newData.currentPrice = { value: "0", isLoading: false, error: true };
              newData.validatorCount = { value: "0", isLoading: false, error: true };
              newData.contractCount = { value: "0", isLoading: false, error: true };
            }

            // Handle latest block response
            if (latestBlockResponse.status === 'fulfilled') {
              newData.blockHeight.value = latestBlockResponse.value.data.response?.[0]?.result?.number?.toString() || "0";
              newData.blockHeight.isLoading = false;
              newData.blockHeight.error = false;
            } else {
              newData.blockHeight.error = true;
            }

            // Handle transactions response
            if (txsResponse.status === 'fulfilled') {
              newData.totalTransactions.value = txsResponse.value.data.total?.toString() || "0";
              newData.totalTransactions.isLoading = false;
              newData.totalTransactions.error = false;
            } else {
              newData.totalTransactions.error = true;
            }

            return newData;
          });
        }
      } catch (error) {
        console.error("Failed to fetch overview data:", error);
      }
    }

    fetchData();
  }, []);

  const blockchainStats = [
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
    },
    {
      data: formatNumberWithCommas(data.validatorCount.value),
      title: "Active Validators",
      loading: data.validatorCount.isLoading,
      error: data.validatorCount.error,
      icon: (
        <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 mb-2 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 12l2 2 4-4m6 2a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      )
    },
    {
      data: formatNumberWithCommas(data.contractCount.value),
      title: "Smart Contracts",
      loading: data.contractCount.isLoading,
      error: data.contractCount.error,
      icon: (
        <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 mb-2 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 20l4-16m4 4l4 4-4 4M6 16l-4-4 4-4" />
        </svg>
      )
    }
  ];

  const financialStats = [
    {
      data: "$" + formatNumberWithCommas(data.marketCap.value),
      title: "Market Cap",
      loading: data.marketCap.isLoading,
      error: data.marketCap.error,
      icon: (
        <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 mb-2 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      )
    },
    {
      data: "$" + parseFloat(data.currentPrice.value).toFixed(4),
      title: "Current Price",
      loading: data.currentPrice.isLoading,
      error: data.currentPrice.error,
      icon: (
        <svg xmlns="http://www.w3.org/2000/svg" className="h-6 w-6 mb-2 text-[#ffa729]" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
        </svg>
      )
    }
  ];

  const videoContainerStyle: React.CSSProperties = {
    position: 'fixed',
    top: 0,
    left: 0,
    width: '100%',
    height: '100%',
    zIndex: -1,
    overflow: 'hidden',
  };

  const videoStyle: React.CSSProperties = {
    width: '100%',
    height: '100%',
    objectFit: 'cover',
    opacity: 0.5, // Adjust this value to make the video more or less visible
  };

  const overlayStyle: React.CSSProperties = {
    position: 'fixed',
    top: 0,
    left: 0,
    width: '100%',
    height: '100%',
    backgroundColor: 'rgba(0, 0, 0, 0.5)', // Semi-transparent black overlay
    zIndex: -1,
  };

  const StatCard = ({ item }: { item: any }) => (
    <div className={`relative overflow-hidden rounded-2xl 
                   bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f]
                   border border-[#3d3d3d] shadow-xl
                   hover:border-[#ffa729] transition-all duration-300
                   group ${!data.dataInitialized ? 'opacity-50' : ''}`}>
      <div className="relative p-3 sm:p-6 text-center min-h-[120px] sm:min-h-[160px] flex flex-col justify-center">
        {item.loading ? (
          <div className="flex flex-col items-center justify-center space-y-2">
            <div className="w-20 sm:w-32 h-5 sm:h-8 bg-gray-700/50 rounded animate-pulse"></div>
            <div className="w-16 sm:w-24 h-3 sm:h-4 bg-gray-700/50 rounded animate-pulse"></div>
          </div>
        ) : item.error ? (
          <div className="flex flex-col items-center justify-center text-red-400">
            <svg xmlns="http://www.w3.org/2000/svg" className="h-5 sm:h-8 w-5 sm:w-8 mb-1 sm:mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 8v4m0 4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
            </svg>
            <span className="text-xs">Failed to load data</span>
          </div>
        ) : (
          <>
            <div className="flex justify-center">{item.icon}</div>
            <h4 className="text-lg sm:text-3xl font-bold mb-1 sm:mb-3 text-[#ffa729] 
                        group-hover:scale-110 transition-transform duration-300 break-words">
              {item.data}
            </h4>
            <p className="text-[11px] sm:text-sm text-gray-300 font-medium">
              {item.title}
            </p>
          </>
        )}
      </div>
    </div>
  );

  return (
    <div className="relative">
      {/* Video Background */}
      <div style={videoContainerStyle}>
        <video
          autoPlay
          loop
          muted
          playsInline
          style={videoStyle}
        >
          <source src="/qrl-video-dark.mp4" type="video/mp4" />
          Your browser does not support the video tag.
        </video>
        <div style={overlayStyle}></div>
      </div>
      
      {/* Existing content */}
      <div className="min-h-screen">
        <div className="max-w-[1200px] mx-auto px-3 sm:px-6 lg:px-8 py-3 sm:py-8">
          <div className="mb-4 sm:mb-10">
            <SearchBar />
          </div>
          
          {!data.dataInitialized && (
            <div className="mb-4 sm:mb-8 p-2 sm:p-4 rounded-lg bg-yellow-500/10 border border-yellow-500/20 text-yellow-500">
              <div className="flex items-center">
                <svg className="animate-spin -ml-1 mr-2 sm:mr-3 h-4 sm:h-5 w-4 sm:w-5" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
                  <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4"></circle>
                  <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
                </svg>
                <span className="text-xs sm:text-sm">Initializing explorer data... This may take a few minutes.</span>
              </div>
            </div>
          )}
          
          <div className="space-y-4 sm:space-y-8">
            {/* Blockchain Stats */}
            <div>
              <h2 className="text-base sm:text-xl font-bold mb-2 sm:mb-4 text-[#ffa729]">Blockchain Statistics</h2>
              <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-2 sm:gap-4">
                {blockchainStats.map((item, idx) => (
                  <StatCard key={idx} item={item} />
                ))}
              </div>
            </div>

            {/* Financial Stats */}
            <div>
              <h2 className="text-base sm:text-xl font-bold mb-2 sm:mb-4 text-[#ffa729]">Financial Statistics</h2>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-2 sm:gap-4 mb-4">
                {financialStats.map((item, idx) => (
                  <StatCard key={idx} item={item} />
                ))}
              </div>
              <Charts />
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
