'use client';

import { useState, useEffect } from 'react';
import type { EpochInfo } from '../../types';

interface EpochInfoPanelProps {
  epochInfo: EpochInfo | null;
  loading: boolean;
}

export default function EpochInfoPanel({ epochInfo, loading }: EpochInfoPanelProps) {
  const currentTimeToNextEpoch = epochInfo?.timeToNextEpoch ?? 0;

  // Sync timeRemaining from prop during render (React-recommended pattern)
  const [prevTime, setPrevTime] = useState(0);
  const [timeRemaining, setTimeRemaining] = useState(0);
  if (currentTimeToNextEpoch !== prevTime) {
    setPrevTime(currentTimeToNextEpoch);
    setTimeRemaining(currentTimeToNextEpoch);
  }

  // Countdown timer, only calls setState inside the interval callback (deferred)
  useEffect(() => {
    if (currentTimeToNextEpoch <= 0) return;
    const start = Date.now();

    const timer = setInterval(() => {
      const elapsed = Math.floor((Date.now() - start) / 1000);
      setTimeRemaining(Math.max(0, currentTimeToNextEpoch - elapsed));
    }, 1000);

    return () => clearInterval(timer);
  }, [currentTimeToNextEpoch]);

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
      <div className="card p-3 sm:p-4 mb-6 animate-pulse">
        <div className="h-5 sm:h-6 bg-surface-2 rounded w-1/3 mb-3 sm:mb-4"></div>
        <div className="h-3 sm:h-4 bg-surface-2 rounded w-full"></div>
      </div>
    );
  }

  if (!epochInfo) {
    return null;
  }

  return (
    <div className="card p-3 sm:p-4 mb-6 overflow-hidden">
      {/* Epoch Stats Grid */}
      <div className="grid grid-cols-2 md:grid-cols-4 gap-2 sm:gap-4 mb-3 sm:mb-4">
        <div className="min-w-0">
          <span className="text-text-secondary text-xs sm:text-sm">Current Epoch</span>
          <p className="text-base sm:font-display text-xl font-semibold text-text-primary truncate">
            {parseInt(epochInfo.headEpoch).toLocaleString()}
          </p>
        </div>
        <div className="min-w-0">
          <span className="text-text-secondary text-xs sm:text-sm">Current Slot</span>
          <p className="text-base sm:text-xl font-semibold text-text-primary truncate">
            {parseInt(epochInfo.headSlot).toLocaleString()}
          </p>
        </div>
        <div className="min-w-0">
          <span className="text-text-secondary text-xs sm:text-sm">Finalized</span>
          <p className="text-base sm:text-xl font-semibold text-success truncate">
            {parseInt(epochInfo.finalizedEpoch).toLocaleString()}
          </p>
        </div>
        <div className="min-w-0">
          <span className="text-text-secondary text-xs sm:text-sm">Justified</span>
          <p className="text-base sm:text-xl font-semibold text-info truncate">
            {parseInt(epochInfo.justifiedEpoch).toLocaleString()}
          </p>
        </div>
      </div>

      {/* Progress Bar */}
      <div className="mt-3 sm:mt-4">
        <div className="flex flex-col sm:flex-row sm:justify-between sm:items-center gap-1 sm:gap-0 mb-2">
          <span className="text-xs sm:text-sm text-text-secondary">
            Slot {epochInfo.slotInEpoch} of {epochInfo.slotsPerEpoch}
          </span>
          <span className="text-xs sm:text-sm text-text-secondary">
            Next epoch in: <span className="text-accent">{formatTime(timeRemaining)}</span>
          </span>
        </div>
        <div className="w-full bg-surface-2 rounded-full h-2 sm:h-2.5 border border-border">
          <div
            className="bg-gradient-to-r from-accent to-accent-dark h-2 sm:h-2.5 rounded-full transition-all duration-1000"
            style={{ width: `${progressPercent}%` }}
          ></div>
        </div>
      </div>
    </div>
  );
}
