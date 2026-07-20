'use client';

import { NATIVE_UNIT } from '../../lib/helpers';

interface ValidatorStats {
  totalValidators: number;
  activeCount: number;
  pendingCount: number;
  exitedCount: number;
  slashedCount: number;
  totalStaked: string;
  currentEpoch: string;
}

interface ValidatorStatsCardsProps {
  stats: ValidatorStats | null;
  loading: boolean;
}

// Format staked amount (beacon chain stores effective balance in Shor, 1 QRL = 10^9 Shor)
function formatStakedAmount(amount: string): string {
  if (!amount || amount === '0') return '0';
  try {
    const value = BigInt(amount);
    const divisor = BigInt('1000000000'); // 10^9 (Shor to QRL)
    const qrlValue = Number(value / divisor);

    if (qrlValue >= 1000000) {
      return (qrlValue / 1000000).toFixed(2) + 'M';
    } else if (qrlValue >= 1000) {
      return (qrlValue / 1000).toFixed(2) + 'K';
    } else {
      return qrlValue.toLocaleString(undefined, { maximumFractionDigits: 2 });
    }
  } catch {
    return '0';
  }
}

export default function ValidatorStatsCards({ stats, loading }: ValidatorStatsCardsProps) {
  if (loading) {
    return (
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3 mb-6">
        {[...Array(6)].map((_, i) => (
          <div key={i} className="card p-4 animate-pulse">
            <div className="h-3 bg-surface-2 rounded w-2/3 mb-2"></div>
            <div className="h-6 bg-surface-2 rounded w-1/2"></div>
          </div>
        ))}
      </div>
    );
  }

  if (!stats) {
    return null;
  }

  const formattedStake = formatStakedAmount(stats.totalStaked);

  const statCards = [
    {
      label: 'Total Validators',
      value: stats.totalValidators.toLocaleString(),
      color: 'text-accent',
    },
    {
      label: 'Active',
      value: stats.activeCount.toLocaleString(),
      color: 'text-success',
    },
    {
      label: 'Pending',
      value: stats.pendingCount.toLocaleString(),
      color: 'text-warning',
    },
    {
      label: 'Exited',
      value: stats.exitedCount.toLocaleString(),
      color: 'text-text-secondary',
    },
    {
      label: 'Slashed',
      value: stats.slashedCount.toLocaleString(),
      color: 'text-error',
    },
    {
      label: 'Total Staked',
      value: `${formattedStake} ${NATIVE_UNIT}`,
      color: 'text-accent',
    },
  ];

  return (
    <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-3 mb-6">
      {statCards.map((card) => (
        <div
          key={card.label}
          className="card p-4"
        >
          <p className="text-xs uppercase tracking-wider text-text-secondary mb-1">{card.label}</p>
          <p className={`text-xl font-semibold ${card.color} truncate`}>{card.value}</p>
        </div>
      ))}
    </div>
  );
}
