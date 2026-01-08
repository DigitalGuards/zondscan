'use client';

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
      <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-2 sm:gap-4 mb-6">
        {[...Array(6)].map((_, i) => (
          <div key={i} className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] p-2 sm:p-4 animate-pulse">
            <div className="h-3 sm:h-4 bg-gray-700 rounded w-2/3 mb-1 sm:mb-2"></div>
            <div className="h-6 sm:h-8 bg-gray-700 rounded w-1/2"></div>
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
      color: 'text-[#ffa729]',
    },
    {
      label: 'Active',
      value: stats.activeCount.toLocaleString(),
      color: 'text-green-400',
    },
    {
      label: 'Pending',
      value: stats.pendingCount.toLocaleString(),
      color: 'text-yellow-400',
    },
    {
      label: 'Exited',
      value: stats.exitedCount.toLocaleString(),
      color: 'text-gray-400',
    },
    {
      label: 'Slashed',
      value: stats.slashedCount.toLocaleString(),
      color: 'text-red-400',
    },
    {
      label: 'Total Staked',
      value: `${formattedStake} QRL`,
      color: 'text-[#ffa729]',
    },
  ];

  return (
    <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-6 gap-2 sm:gap-4 mb-6">
      {statCards.map((card, index) => (
        <div
          key={index}
          className="bg-gradient-to-br from-[#2d2d2d] to-[#1f1f1f] rounded-xl border border-[#3d3d3d] p-2 sm:p-4"
        >
          <h3 className="text-xs sm:text-sm font-medium text-gray-400 mb-1">{card.label}</h3>
          <p className={`text-lg sm:text-2xl font-semibold ${card.color} break-words`}>{card.value}</p>
        </div>
      ))}
    </div>
  );
}
