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

export default function ValidatorsWrapper(): JSX.Element {
  const [validators, setValidators] = useState<Validator[]>([]);
  const [epochInfo, setEpochInfo] = useState<EpochInfo | null>(null);
  const [stats, setStats] = useState<ValidatorStats | null>(null);
  const [history, setHistory] = useState<HistoryRecord[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [chartWidth, setChartWidth] = useState(400);

  // Update chart width on resize
  useEffect(() => {
    const updateWidth = () => {
      const width = Math.min(window.innerWidth - 48, 600);
      setChartWidth(width);
    };
    updateWidth();
    window.addEventListener('resize', updateWidth);
    return () => window.removeEventListener('resize', updateWidth);
  }, []);

  const fetchData = useCallback(async () => {
    try {
      // Fetch all data in parallel
      const [validatorsRes, epochRes, statsRes, historyRes] = await Promise.all([
        axios.get(`${config.handlerUrl}/validators`).catch(() => ({ data: { validators: [] } })),
        axios.get(`${config.handlerUrl}/epoch`).catch(() => ({ data: null })),
        axios.get(`${config.handlerUrl}/validators/stats`).catch(() => ({ data: null })),
        axios.get(`${config.handlerUrl}/validators/history?limit=100`).catch(() => ({ data: { history: [] } })),
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
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6 mb-6">
        {/* Status Distribution Chart */}
        <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] p-4">
          <h3 className="text-lg font-semibold text-[#ffa729] mb-4">Status Distribution</h3>
          <div className="flex justify-center">
            {loading ? (
              <div className="h-[300px] flex items-center justify-center">
                <div className="animate-pulse text-gray-500">Loading chart...</div>
              </div>
            ) : (
              <ValidatorStatusChart
                activeCount={stats?.activeCount || 0}
                pendingCount={stats?.pendingCount || 0}
                exitedCount={stats?.exitedCount || 0}
                slashedCount={stats?.slashedCount || 0}
                width={Math.min(chartWidth, 350)}
                height={300}
              />
            )}
          </div>
        </div>

        {/* Total Staked Chart */}
        <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] p-4">
          <h3 className="text-lg font-semibold text-[#ffa729] mb-4">Total Staked Over Time</h3>
          {loading ? (
            <div className="h-[300px] flex items-center justify-center">
              <div className="animate-pulse text-gray-500">Loading chart...</div>
            </div>
          ) : (
            <ValidatorHistoryChart
              data={history}
              type="staked"
              width={chartWidth}
              height={300}
            />
          )}
        </div>
      </div>

      {/* Validator Count History */}
      <div className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] p-4 mb-6">
        <h3 className="text-lg font-semibold text-[#ffa729] mb-4">Validator Count Over Time</h3>
        {loading ? (
          <div className="h-[250px] flex items-center justify-center">
            <div className="animate-pulse text-gray-500">Loading chart...</div>
          </div>
        ) : (
          <ValidatorHistoryChart
            data={history}
            type="count"
            width={Math.min(window.innerWidth - 48, 1200)}
            height={250}
          />
        )}
      </div>

      {/* Validators Table */}
      <div className="mb-6">
        <h3 className="text-lg font-semibold text-[#ffa729] mb-4">All Validators</h3>
        <ValidatorTable validators={validators} loading={loading} />
      </div>
    </div>
  );
}
