'use client';

import { useState, useEffect } from 'react';

interface EpochInfo {
  headEpoch: string;
  headSlot: string;
  finalizedEpoch: string;
  justifiedEpoch: string;
  slotsPerEpoch: number;
  secondsPerSlot: number;
  slotInEpoch: number;
  timeToNextEpoch: number;
  updatedAt: number;
}

interface EpochInfoPanelProps {
  epochInfo: EpochInfo | null;
  loading: boolean;
}

export default function EpochInfoPanel({ epochInfo, loading }: EpochInfoPanelProps) {
  const [timeRemaining, setTimeRemaining] = useState<number>(0);

  useEffect(() => {
    if (epochInfo?.timeToNextEpoch) {
      setTimeRemaining(epochInfo.timeToNextEpoch);
    }
  }, [epochInfo?.timeToNextEpoch]);

  // Countdown timer
  useEffect(() => {
    if (timeRemaining <= 0) return;

    const timer = setInterval(() => {
      setTimeRemaining((prev) => Math.max(0, prev - 1));
    }, 1000);

    return () => clearInterval(timer);
  }, [timeRemaining]);

  const formatTime = (seconds: number): string => {
    const hours = Math.floor(seconds / 3600);
    const mins = Math.floor((seconds % 3600) / 60);
    const secs = seconds % 60;

    if (hours > 0) {
      return `${hours}h ${mins}m ${secs}s`;
    }
    return `${mins}m ${secs}s`;
  };

  const progressPercent = epochInfo
    ? ((epochInfo.slotInEpoch / epochInfo.slotsPerEpoch) * 100)
    : 0;

  if (loading) {
    return (
      <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] p-3 sm:p-4 mb-6 animate-pulse">
        <div className="h-5 sm:h-6 bg-gray-700 rounded w-1/3 mb-3 sm:mb-4"></div>
        <div className="h-3 sm:h-4 bg-gray-700 rounded w-full"></div>
      </div>
    );
  }

  if (!epochInfo) {
    return null;
  }

  return (
    <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] p-3 sm:p-4 mb-6">
      {/* Epoch Stats Grid */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-2 sm:gap-4 mb-3 sm:mb-4">
        <div>
          <span className="text-gray-400 text-xs sm:text-sm">Current Epoch</span>
          <p className="text-base sm:text-xl font-semibold text-[#ffa729]">
            {parseInt(epochInfo.headEpoch).toLocaleString()}
          </p>
        </div>
        <div>
          <span className="text-gray-400 text-xs sm:text-sm">Current Slot</span>
          <p className="text-base sm:text-xl font-semibold text-gray-200">
            {parseInt(epochInfo.headSlot).toLocaleString()}
          </p>
        </div>
        <div>
          <span className="text-gray-400 text-xs sm:text-sm">Finalized</span>
          <p className="text-base sm:text-xl font-semibold text-green-400">
            {parseInt(epochInfo.finalizedEpoch).toLocaleString()}
          </p>
        </div>
        <div>
          <span className="text-gray-400 text-xs sm:text-sm">Justified</span>
          <p className="text-base sm:text-xl font-semibold text-blue-400">
            {parseInt(epochInfo.justifiedEpoch).toLocaleString()}
          </p>
        </div>
      </div>

      {/* Progress Bar */}
      <div className="mt-3 sm:mt-4">
        <div className="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-1 sm:gap-0 mb-2">
          <span className="text-xs sm:text-sm text-gray-400">
            Slot {epochInfo.slotInEpoch} of {epochInfo.slotsPerEpoch}
          </span>
          <span className="text-xs sm:text-sm text-gray-400">
            Next epoch in: <span className="text-[#ffa729]">{formatTime(timeRemaining)}</span>
          </span>
        </div>
        <div className="w-full bg-[#1f1f1f] rounded-full h-2 sm:h-2.5 border border-[#3d3d3d]">
          <div
            className="bg-gradient-to-r from-[#ffa729] to-[#ff8c00] h-2 sm:h-2.5 rounded-full transition-all duration-1000"
            style={{ width: `${progressPercent}%` }}
          ></div>
        </div>
      </div>
    </div>
  );
}
