'use client';

import { NATIVE_UNIT } from '../../lib/helpers';
import {
  PROPOSER_WEIGHT,
  SYNC_REWARD_WEIGHT,
  TIMELY_HEAD_WEIGHT,
  TIMELY_SOURCE_WEIGHT,
  TIMELY_TARGET_WEIGHT,
  WEIGHT_DENOMINATOR,
  type StakingResult,
} from '../../lib/staking';
import { num } from '../format';

interface RewardBreakdownProps {
  result: StakingResult;
}

interface Duty {
  key: keyof StakingResult['breakdown'];
  label: string;
  weight: number | null;
  note: string;
}

const DUTIES: Duty[] = [
  {
    key: 'target',
    label: 'Target vote',
    weight: TIMELY_TARGET_WEIGHT,
    note: 'The heaviest duty, and penalised when missed',
  },
  {
    key: 'source',
    label: 'Source vote',
    weight: TIMELY_SOURCE_WEIGHT,
    note: 'Penalised when missed',
  },
  {
    key: 'head',
    label: 'Head vote',
    weight: TIMELY_HEAD_WEIGHT,
    note: 'Paid when correct, never penalised',
  },
  {
    key: 'proposer',
    label: 'Block proposals',
    weight: PROPOSER_WEIGHT,
    note: 'Lottery, averaged over a year',
  },
  {
    key: 'sync',
    label: 'Sync committee',
    weight: SYNC_REWARD_WEIGHT,
    note: 'Lottery, arrives in bursts',
  },
  {
    key: 'fees',
    label: 'Priority fees',
    weight: null,
    note: 'Tips captured when proposing',
  },
];

/**
 * Where a validator's pay comes from, per year, for one validator.
 *
 * Bars are scaled against the largest positive component so the mix stays
 * readable. A negative row means penalties outweigh rewards for that duty,
 * which is what heavy downtime looks like on source and target.
 */
export default function RewardBreakdown({ result }: RewardBreakdownProps): JSX.Element {
  const rows = DUTIES.map((duty) => ({ ...duty, value: result.breakdown[duty.key] })).filter(
    (row) => row.value !== 0 || row.key !== 'fees',
  );

  const scale = Math.max(...rows.map((r) => Math.abs(r.value)), 1e-9);
  const total = result.totalRewardPerValidator;

  return (
    <section aria-label="Reward breakdown" className="card p-5">
      <div className="flex items-baseline justify-between gap-3 flex-wrap mb-4">
        <h3 className="text-xs uppercase tracking-wider text-text-secondary">Where the reward comes from</h3>
        <span className="text-[11px] text-text-muted">
          per validator per year, {WEIGHT_DENOMINATOR}ths of base reward
        </span>
      </div>

      <div className="space-y-2.5">
        {rows.map((row) => {
          const width = Math.min(100, (Math.abs(row.value) / scale) * 100);
          const negative = row.value < 0;
          const share = total > 0 ? row.value / total : 0;
          return (
            <div key={row.key}>
              <div className="flex items-baseline justify-between gap-3 text-xs mb-1">
                <span className="text-text-primary">
                  {row.label}
                  {row.weight !== null ? (
                    <span className="text-text-muted ml-2">
                      {row.weight}/{WEIGHT_DENOMINATOR}
                    </span>
                  ) : null}
                </span>
                <span className={`font-display font-medium ${negative ? 'text-error' : 'text-text-secondary'}`}>
                  {num(row.value, 1)} {NATIVE_UNIT}
                  {total > 0 ? (
                    <span className="text-text-muted ml-2">{(share * 100).toFixed(0)}%</span>
                  ) : null}
                </span>
              </div>
              <div className="h-1.5 rounded-full bg-surface-2 overflow-hidden">
                <div
                  className={`h-full rounded-full ${negative ? 'bg-error' : 'bg-accent'}`}
                  style={{ width: `${width}%` }}
                />
              </div>
              <p className="text-[11px] text-text-muted mt-1">{row.note}</p>
            </div>
          );
        })}
      </div>

      <div className="mt-4 pt-4 border-t border-border flex items-baseline justify-between gap-3">
        <span className="text-xs text-text-secondary">Total per validator</span>
        <span className="font-display text-sm font-semibold text-accent">
          {num(total, 1)} {NATIVE_UNIT}
        </span>
      </div>
    </section>
  );
}
