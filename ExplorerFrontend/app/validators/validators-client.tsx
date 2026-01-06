'use client';

import { useState, useEffect, useCallback } from 'react';
import axios from 'axios';
import config from '../../config';
import EpochInfoPanel from './components/EpochInfoPanel';
import ValidatorStatsCards from './components/ValidatorStatsCards';
import ValidatorStatusChart from './components/ValidatorStatusChart';
import ValidatorHistoryChart from './components/ValidatorHistoryChart';
import ValidatorTable from './components/ValidatorTable';

interface Validator {
  index: string;
  address: string;
  status: string;
  age: number;
  stakedAmount: string;
  isActive: boolean;
}

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

interface ValidatorStats {
  totalValidators: number;
  activeCount: number;
  pendingCount: number;
  exitedCount: number;
  slashedCount: number;
  totalStaked: string;
  currentEpoch: string;
}

interface HistoryRecord {
  epoch: string;
  timestamp: number;
  validatorsCount: number;
  activeCount: number;
  totalStaked: string;
}

// Custom hook for responsive viewport dimensions (SSR-safe)
function useViewport() {
  const [viewport, setViewport] = useState({
    width: 800, // Default for SSR
    isMobile: false,
    chartWidth: 400,
    fullChartWidth: 800,
  });

  useEffect(() => {
    const updateViewport = () => {
      const width = window.innerWidth;
      const isMobile = width < 640;
      const padding = isMobile ? 64 : 48;
      const maxWidth = isMobile ? 500 : 600;
      const chartWidth = Math.max(Math.min(width - padding, maxWidth), 280);
      const fullChartWidth = Math.max(Math.min(width - 64, 1200), 300);

      setViewport({ width, isMobile, chartWidth, fullChartWidth });
    };

    updateViewport();
    window.addEventListener('resize', updateViewport);
    return () => window.removeEventListener('resize', updateViewport);
  }, []);

  return viewport;
}

export default function ValidatorsWrapper(): JSX.Element {
  const [validators, setValidators] = useState<Validator[]>([]);
  const [epochInfo, setEpochInfo] = useState<EpochInfo | null>(null);
  const [stats, setStats] = useState<ValidatorStats | null>(null);
  const [history, setHistory] = useState<HistoryRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Use viewport hook for responsive dimensions (SSR-safe)
  const { isMobile, chartWidth, fullChartWidth } = useViewport();

  const fetchData = useCallback(async () => {
    try {
      // Fetch all data in parallel
      const [validatorsRes, epochRes, statsRes, historyRes] = await Promise.all([
        axios.get(`${config.handlerUrl}/validators`).catch((err) => { console.error('Failed to fetch validators:', err); return { data: { validators: [] } }; }),
        axios.get(`${config.handlerUrl}/epoch`).catch((err) => { console.error('Failed to fetch epoch:', err); return { data: null }; }),
        axios.get(`${config.handlerUrl}/validators/stats`).catch((err) => { console.error('Failed to fetch validator stats:', err); return { data: null }; }),
        axios.get(`${config.handlerUrl}/validators/history?limit=100`).catch((err) => { console.error('Failed to fetch validator history:', err); return { data: { history: [] } }; }),
      ]);

      // Process validators - add Z prefix to addresses
      const processedValidators = (validatorsRes.data.validators || []).map((v: any) => ({
        ...v,
        address: v.address.startsWith('Z') ? v.address : 'Z' + v.address,
      }));

      setValidators(processedValidators);
      setEpochInfo(epochRes.data);
      setStats(statsRes.data);
      setHistory(historyRes.data?.history || []);
      setError(null);
    } catch (err) {
      console.error('Error fetching validator data:', err);
      setError('Failed to load validator data. Please try again later.');
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    fetchData();
    // Refresh data every 60 seconds
    const interval = setInterval(fetchData, 60000);
    return () => clearInterval(interval);
  }, [fetchData]);

  if (error && !validators.length) {
    return (
      <div className="max-w-7xl mx-auto p-4 sm:p-6 lg:p-8">
        <div className="bg-red-900/20 border border-red-800 rounded-xl p-6 text-center">
          <h2 className="text-xl font-semibold text-red-400 mb-2">Error</h2>
          <p className="text-gray-400">{error}</p>
          <button
            onClick={fetchData}
            className="mt-4 px-4 py-2 bg-[#2d2d2d] border border-[#3d3d3d] rounded-lg text-gray-300 hover:border-[#ffa729]"
          >
            Retry
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto p-4 sm:p-6 lg:p-8">
      {/* Page Header */}
      <div className="mb-6">
        <h1 className="text-xl sm:text-2xl font-bold text-[#ffa729] mb-2">Validators</h1>
        <p className="text-gray-400">
          View all validators on the QRL Zond network
        </p>
      </div>

      {/* Epoch Info Panel */}
      <EpochInfoPanel epochInfo={epochInfo} loading={loading} />

      {/* Stats Cards */}
      <ValidatorStatsCards stats={stats} loading={loading} />

      {/* Charts Section */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4 sm:gap-6 mb-6">
        {/* Status Distribution Chart */}
        <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] p-3 sm:p-4">
          <h3 className="text-base sm:text-lg font-semibold text-[#ffa729] mb-3 sm:mb-4">Status Distribution</h3>
          <div className="flex justify-center overflow-hidden">
            {loading ? (
              <div className="h-[250px] sm:h-[300px] flex items-center justify-center">
                <div className="animate-pulse text-gray-500">Loading chart...</div>
              </div>
            ) : (
              <ValidatorStatusChart
                activeCount={stats?.activeCount || 0}
                pendingCount={stats?.pendingCount || 0}
                exitedCount={stats?.exitedCount || 0}
                slashedCount={stats?.slashedCount || 0}
                width={Math.min(chartWidth, 350)}
                height={isMobile ? 250 : 300}
              />
            )}
          </div>
        </div>

        {/* Total Staked Chart */}
        <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] p-3 sm:p-4 overflow-hidden">
          <h3 className="text-base sm:text-lg font-semibold text-[#ffa729] mb-3 sm:mb-4">Total Staked Over Time</h3>
          <div className="overflow-x-auto">
            {loading ? (
              <div className="h-[250px] sm:h-[300px] flex items-center justify-center">
                <div className="animate-pulse text-gray-500">Loading chart...</div>
              </div>
            ) : (
              <ValidatorHistoryChart
                data={history}
                type="staked"
                width={Math.max(chartWidth - 24, 300)}
                height={isMobile ? 250 : 300}
              />
            )}
          </div>
        </div>
      </div>

      {/* Validator Count History */}
      <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] p-3 sm:p-4 mb-6 overflow-hidden">
        <h3 className="text-base sm:text-lg font-semibold text-[#ffa729] mb-3 sm:mb-4">Validator Count Over Time</h3>
        <div className="overflow-x-auto">
          {loading ? (
            <div className="h-[200px] sm:h-[250px] flex items-center justify-center">
              <div className="animate-pulse text-gray-500">Loading chart...</div>
            </div>
          ) : (
            <ValidatorHistoryChart
              data={history}
              type="count"
              width={fullChartWidth}
              height={isMobile ? 200 : 250}
            />
          )}
        </div>
      </div>

      {/* Validators Table */}
      <div className="mb-6">
        <h3 className="text-lg font-semibold text-[#ffa729] mb-4">All Validators</h3>
        <ValidatorTable validators={validators} loading={loading} />
      </div>
    </div>
  );
}
