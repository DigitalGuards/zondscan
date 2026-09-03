'use client';

import { useCallback, useEffect, useMemo, useState } from 'react';
import { CheckIcon, InformationCircleIcon, LinkIcon } from '@heroicons/react/24/outline';
import { useSearchParams } from 'next/navigation';
import axios from 'axios';
import config from '../../config';
import { setUrlParams } from '../lib/use-url-param';
import { NATIVE_UNIT } from '../lib/helpers';
import {
  EJECTION_BALANCE_QUANTA,
  EPOCHS_PER_SYNC_COMMITTEE_PERIOD,
  EPOCHS_PER_YEAR,
  MAX_EFFECTIVE_BALANCE_QUANTA,
  MAX_SUPPLY_QUANTA,
  SECONDS_PER_EPOCH,
  SECONDS_PER_SLOT,
  SECONDS_PER_YEAR,
  SHOR_PER_QUANTA,
  SLOTS_PER_EPOCH,
  SYNC_COMMITTEE_SIZE,
  computeStaking,
  quantaToValidators,
} from '../lib/staking';
import { compact, humanDuration, humanSeconds, num, pct, usd } from './format';
import AprCurve from './components/AprCurve';
import RewardBreakdown from './components/RewardBreakdown';

interface ValidatorStats {
  activeCount: number;
  totalStaked: string;
  currentEpoch: string;
}

interface NetworkSnapshot {
  validators: number;
  circulatingSupply: number;
  priceUsd: number;
  epoch: string;
}

const FALLBACK: NetworkSnapshot = {
  validators: 512,
  circulatingSupply: 80_388_299,
  priceUsd: 0,
  epoch: '',
};

const DEFAULT_UPTIME = 0.99;
const DEFAULT_PARTICIPATION = 0.99;

const INPUT_CLASSES =
  'w-full px-3 py-2 bg-background text-text-primary rounded-lg border border-border text-sm focus:outline-none focus:border-accent transition-colors';

const quanta = (n: number, digits = 2): string => `${num(n, digits)} ${NATIVE_UNIT}`;

function Field({
  label,
  hint,
  children,
}: {
  label: string;
  hint?: string;
  children: React.ReactNode;
}): JSX.Element {
  return (
    <div>
      <div className="flex items-baseline justify-between gap-2 mb-1.5">
        <span className="text-sm font-medium text-text-secondary">{label}</span>
        {hint ? <span className="text-[11px] text-text-muted">{hint}</span> : null}
      </div>
      {children}
    </div>
  );
}

function Stat({
  label,
  value,
  sub,
  accent,
}: {
  label: string;
  value: string;
  sub?: string;
  accent?: boolean;
}): JSX.Element {
  return (
    <div className="card p-4">
      <p className="text-xs uppercase tracking-wider text-text-secondary mb-1">{label}</p>
      <p
        className={`font-display text-xl font-semibold truncate ${
          accent ? 'text-accent' : 'text-text-primary'
        }`}
      >
        {value}
      </p>
      {sub ? <p className="text-xs text-text-muted mt-1">{sub}</p> : null}
    </div>
  );
}

export default function StakingCalculatorClient(): JSX.Element {
  const [snapshot, setSnapshot] = useState<NetworkSnapshot>(FALLBACK);
  const [live, setLive] = useState(false);
  const [loading, setLoading] = useState(true);

  // Scenario inputs. Validator counts are the primary lever: activation
  // requires exactly MAX_EFFECTIVE_BALANCE, so stake and validator count are
  // the same number in different units.
  const searchParams = useSearchParams();
  const readParam = useCallback(
    (key: string, fallback: number): number => {
      const raw = searchParams.get(key);
      if (raw === null) return fallback;
      const parsed = Number(raw);
      return Number.isFinite(parsed) && parsed >= 0 ? parsed : fallback;
    },
    [searchParams],
  );

  const [networkValidators, setNetworkValidators] = useState(() =>
    readParam('validators', FALLBACK.validators),
  );
  const [ownValidators, setOwnValidators] = useState(() => readParam('own', 1));
  const [uptime, setUptime] = useState(() => readParam('uptime', DEFAULT_UPTIME));
  const [participation, setParticipation] = useState(() =>
    readParam('participation', DEFAULT_PARTICIPATION),
  );
  const [priorityFee, setPriorityFee] = useState(() => readParam('fee', 0));
  const [supply, setSupply] = useState(FALLBACK.circulatingSupply);
  const [price, setPrice] = useState(0);
  const [showAssumptions, setShowAssumptions] = useState(false);

  // A shared link pins whatever it names. Live data still seeds the rest, so
  // ?validators=1000 keeps its 1000 once the snapshot lands.
  const [pinned] = useState<ReadonlySet<string>>(
    () => new Set(['validators', 'own', 'uptime', 'participation', 'fee'].filter((k) => searchParams.get(k) !== null)),
  );

  const applySnapshot = useCallback(
    (next: NetworkSnapshot) => {
      if (!pinned.has('validators')) setNetworkValidators(next.validators);
      setSupply(next.circulatingSupply);
      setPrice(next.priceUsd);
    },
    [pinned],
  );

  // Mirror the scenario into the URL once the user actually changes something,
  // so the page stays linkable without rewriting the URL on every page view.
  const [touched, setTouched] = useState(false);
  const touch = useCallback(() => setTouched(true), []);

  useEffect(() => {
    if (!touched) return;
    setUrlParams({
      validators: String(networkValidators),
      own: String(ownValidators),
      uptime: uptime === DEFAULT_UPTIME ? null : uptime.toFixed(3),
      participation: participation === DEFAULT_PARTICIPATION ? null : participation.toFixed(3),
      fee: priorityFee === 0 ? null : String(priorityFee),
    });
  }, [touched, networkValidators, ownValidators, uptime, participation, priorityFee]);

  useEffect(() => {
    let cancelled = false;

    const load = async (): Promise<void> => {
      try {
        const [statsRes, overviewRes] = await Promise.all([
          axios.get<ValidatorStats>(`${config.handlerUrl}/validators/stats`, { timeout: 15000 }),
          axios.get<{ circulating?: string; currentPrice?: number }>(
            `${config.handlerUrl}/overview`,
            { timeout: 15000 },
          ),
        ]);
        if (cancelled) return;

        const stats = statsRes.data;
        // totalStaked is Shor-denominated; derive the validator count from it
        // so a partially-slashed set still lands on a sane number.
        const stakedQuanta = Number(BigInt(stats.totalStaked || '0') / SHOR_PER_QUANTA);
        const derived = stats.activeCount || quantaToValidators(stakedQuanta) || FALLBACK.validators;
        const circulating = Number(overviewRes.data.circulating || 0) || FALLBACK.circulatingSupply;

        const next: NetworkSnapshot = {
          validators: derived,
          circulatingSupply: circulating,
          priceUsd: overviewRes.data.currentPrice || 0,
          epoch: stats.currentEpoch || '',
        };
        setSnapshot(next);
        applySnapshot(next);
        setLive(true);
      } catch {
        // Keep the fallback scenario. The calculator is fully usable without
        // the network, it just starts from last-known figures.
        if (!cancelled) setLive(false);
      } finally {
        if (!cancelled) setLoading(false);
      }
    };

    void load();
    return () => {
      cancelled = true;
    };
  }, [applySnapshot]);

  // A validator locks 40,000 Quanta, so the supply is a hard ceiling on how
  // large the active set can ever get.
  const maxValidators = Math.max(1, quantaToValidators(supply));

  const result = useMemo(
    () =>
      computeStaking({
        networkValidators,
        ownValidators,
        uptime,
        participation,
        priorityFeePerBlock: priorityFee,
        circulatingSupply: supply,
        priceUsd: price,
      }),
    [networkValidators, ownValidators, uptime, participation, priorityFee, supply, price],
  );

  const liveResult = useMemo(
    () =>
      computeStaking({
        networkValidators: snapshot.validators,
        ownValidators: 1,
        uptime: 1,
        participation: 1,
        priorityFeePerBlock: 0,
        circulatingSupply: snapshot.circulatingSupply,
        priceUsd: 0,
      }),
    [snapshot],
  );

  const isDirty =
    networkValidators !== snapshot.validators ||
    supply !== snapshot.circulatingSupply ||
    price !== snapshot.priceUsd;

  const [copied, setCopied] = useState(false);
  const handleCopyLink = useCallback(() => {
    // The effect above only writes the URL after an edit, so build the link
    // from current state to cover an untouched page too.
    const url = new URL(window.location.href);
    url.searchParams.set('validators', String(networkValidators));
    url.searchParams.set('own', String(ownValidators));
    if (uptime !== DEFAULT_UPTIME) url.searchParams.set('uptime', uptime.toFixed(3));
    if (participation !== DEFAULT_PARTICIPATION) {
      url.searchParams.set('participation', participation.toFixed(3));
    }
    if (priorityFee !== 0) url.searchParams.set('fee', String(priorityFee));

    void navigator.clipboard
      .writeText(url.toString())
      .then(() => {
        setCopied(true);
        window.setTimeout(() => setCopied(false), 2000);
      })
      .catch(() => {
        // Clipboard access can be denied; the URL bar still carries the state.
      });
  }, [networkValidators, ownValidators, uptime, participation, priorityFee]);

  const proposalGap = result.proposalsPerYear > 0 ? EPOCHS_PER_YEAR / result.proposalsPerYear : 0;
  const syncSeatShare = Math.min(1, SYNC_COMMITTEE_SIZE / networkValidators);

  return (
    <div className="max-w-7xl mx-auto px-4 sm:px-6 py-8 space-y-6">
      <header className="space-y-3">
        <h2 className="section-title">Staking Calculator</h2>
        <p className="text-sm text-text-secondary max-w-3xl leading-relaxed">
          Validator rewards on QRL 2.0 come from the beacon chain, where issuance grows with the
          square root of total stake. Yield therefore falls as the validator set grows. Every figure
          below is computed with the same reward formula the consensus client runs, seeded from live
          network data.
        </p>
      </header>

      <div className="card-simple border-warning/25 bg-warning/[0.04] px-4 py-3">
        <div className="flex gap-3">
          <InformationCircleIcon
            aria-hidden="true"
            className="h-4 w-4 shrink-0 text-warning mt-0.5"
          />
          <div className="text-xs text-text-secondary leading-relaxed space-y-1">
            <p>
              <span className="text-text-primary font-medium">Testnet figures.</span> Live values
              come from QRL 2.0 testnet, currently a 512-validator set with no meaningful fee
              market. They indicate the shape of the reward curve rather than the returns of a
              mature network.
            </p>
            <p>
              Consensus parameters are expected to change at the 64-byte address relaunch. Treat
              this as a model of the protocol as it stands today, and re-check it after the next
              testnet lands.
            </p>
          </div>
        </div>
      </div>

      {/* Live network snapshot */}
      <section aria-label="Network snapshot" className="space-y-3">
        <div className="flex items-center justify-between gap-3 flex-wrap">
          <h3 className="text-xs uppercase tracking-wider text-text-secondary">Network right now</h3>
          <div className="flex items-center gap-3">
            <span className="flex items-center gap-1.5 text-[11px] text-text-muted">
              <span
                aria-hidden="true"
                className={`h-1.5 w-1.5 rounded-full ${
                  loading ? 'bg-text-muted' : live ? 'bg-success' : 'bg-warning'
                }`}
              />
              {loading ? 'loading' : live ? `epoch ${snapshot.epoch}` : 'last known values'}
            </span>
            {isDirty ? (
              <button
                type="button"
                onClick={() => applySnapshot(snapshot)}
                className="text-[11px] font-medium text-accent hover:text-accent-hover transition-colors"
              >
                Reset to live
              </button>
            ) : null}
            <button
              type="button"
              onClick={handleCopyLink}
              className="flex items-center gap-1.5 text-[11px] font-medium text-accent hover:text-accent-hover transition-colors"
            >
              {copied ? (
                <CheckIcon aria-hidden="true" className="h-3.5 w-3.5" />
              ) : (
                <LinkIcon aria-hidden="true" className="h-3.5 w-3.5" />
              )}
              {copied ? 'Link copied' : 'Copy link'}
            </button>
          </div>
        </div>

        <div className="grid grid-cols-2 lg:grid-cols-4 gap-3 md:gap-4">
          <Stat
            label="Active validators"
            value={num(snapshot.validators)}
            sub={`${compact(snapshot.validators * MAX_EFFECTIVE_BALANCE_QUANTA)} ${NATIVE_UNIT} staked`}
          />
          <Stat
            label="Supply staked"
            value={pct(liveResult.stakingRatio, 1)}
            sub={`of ${compact(snapshot.circulatingSupply)} circulating`}
          />
          <Stat
            label="Gross APR"
            value={pct(liveResult.grossApr)}
            sub="perfect uptime, before fees"
            accent
          />
          <Stat
            label="Annual issuance"
            value={`${compact(liveResult.networkAnnualIssuance)} ${NATIVE_UNIT}`}
            sub={`${pct(liveResult.inflationRate, 2)} supply growth`}
          />
        </div>
      </section>

      <div className="grid lg:grid-cols-[minmax(0,340px)_minmax(0,1fr)] gap-6 items-start">
        {/* Inputs */}
        <section
          aria-label="Scenario inputs"
          className="card p-5 space-y-5 lg:sticky lg:top-6"
        >
          <h3 className="text-xs uppercase tracking-wider text-text-secondary">Your scenario</h3>

          <Field label="Validators you run" hint={`${quanta(result.ownStake, 0)} locked`}>
            <input
              type="number"
              min={0}
              step={1}
              value={ownValidators}
              onChange={(e) => {
                touch();
                setOwnValidators(Math.max(0, Number(e.target.value) || 0));
              }}
              className={INPUT_CLASSES}
              aria-describedby="own-validators-hint"
            />
            <p id="own-validators-hint" className="text-[11px] text-text-muted mt-1.5">
              Each validator requires exactly{' '}
              {num(MAX_EFFECTIVE_BALANCE_QUANTA)} {NATIVE_UNIT} to activate.
            </p>
          </Field>

          <Field
            label="Network validators"
            hint={`${pct(result.stakingRatio, 1)} of supply`}
          >
            <input
              type="range"
              min={1}
              max={maxValidators}
              step={1}
              value={Math.min(networkValidators, maxValidators)}
              onChange={(e) => {
                touch();
                setNetworkValidators(Number(e.target.value));
              }}
              className="w-full accent-accent"
              aria-label="Network validator count"
            />
            <div className="flex items-center gap-2 mt-2">
              <input
                type="number"
                min={1}
                max={maxValidators}
                step={1}
                value={networkValidators}
                onChange={(e) => {
                  touch();
                  setNetworkValidators(Math.max(1, Number(e.target.value) || 1));
                }}
                className={INPUT_CLASSES}
              />
              <span className="text-xs text-text-muted whitespace-nowrap">
                {compact(result.totalStaked)}
              </span>
            </div>
          </Field>

          <Field label="Your uptime" hint={pct(uptime, 1)}>
            <input
              type="range"
              min={0}
              max={1}
              step={0.005}
              value={uptime}
              onChange={(e) => {
                touch();
                setUptime(Number(e.target.value));
              }}
              className="w-full accent-accent"
              aria-label="Your validator uptime"
            />
          </Field>

          <Field label="Network participation" hint={pct(participation, 1)}>
            <input
              type="range"
              min={0}
              max={1}
              step={0.005}
              value={participation}
              onChange={(e) => {
                touch();
                setParticipation(Number(e.target.value));
              }}
              className="w-full accent-accent"
              aria-label="Network attestation participation"
            />
            <p className="text-[11px] text-text-muted mt-1.5">
              Attestation pay scales with how much of the network is attesting correctly.
            </p>
          </Field>

          <div className="pt-1">
            <button
              type="button"
              onClick={() => setShowAssumptions((v) => !v)}
              aria-expanded={showAssumptions}
              className="text-xs font-medium text-accent hover:text-accent-hover transition-colors"
            >
              {showAssumptions ? 'Hide' : 'Show'} advanced assumptions
            </button>
          </div>

          {showAssumptions ? (
            <div className="space-y-4 pt-1">
              <Field label={`Priority fee per block (${NATIVE_UNIT})`} hint="tips only">
                <input
                  type="number"
                  min={0}
                  step={0.001}
                  value={priorityFee}
                  onChange={(e) => {
                    touch();
                    setPriorityFee(Math.max(0, Number(e.target.value) || 0));
                  }}
                  className={INPUT_CLASSES}
                />
                <p className="text-[11px] text-text-muted mt-1.5">
                  Base fees are burned, so only the tip reaches the proposer. Testnet blocks carry
                  effectively none.
                </p>
              </Field>

              <Field label={`Circulating supply (${NATIVE_UNIT})`}>
                <input
                  type="number"
                  min={0}
                  step={1_000_000}
                  value={supply}
                  onChange={(e) => setSupply(Math.max(0, Number(e.target.value) || 0))}
                  className={INPUT_CLASSES}
                />
              </Field>

              <Field label="Quanta price (USD)" hint="0 hides fiat">
                <input
                  type="number"
                  min={0}
                  step={0.01}
                  value={price}
                  onChange={(e) => setPrice(Math.max(0, Number(e.target.value) || 0))}
                  className={INPUT_CLASSES}
                />
              </Field>
            </div>
          ) : null}
        </section>

        {/* Results */}
        <div className="space-y-6 min-w-0">
          <section aria-label="Projected returns" className="space-y-3">
            <div className="grid sm:grid-cols-3 gap-3 md:gap-4">
              <Stat
                label="Effective APR"
                value={pct(result.aprWithFees)}
                sub={`ceiling ${pct(result.grossApr)} at perfect duty`}
                accent
              />
              <Stat
                label="Real yield"
                value={pct(result.realYield)}
                sub={`after ${pct(result.inflationRate, 2)} issuance dilution at ${pct(
                  result.stakingRatio,
                  0,
                )} staked`}
              />
              <Stat
                label="Reward per validator"
                value={quanta(result.totalRewardPerValidator, 0)}
                sub="per year"
              />
            </div>
            <p className="text-[11px] text-text-muted leading-relaxed max-w-3xl">
              Effective APR is the balance you actually gain, and it is what to compare against
              holding {NATIVE_UNIT} without staking, since an idle balance is diluted{' '}
              {pct(result.inflationRate, 2)} a year. Real yield measures something narrower: how
              fast your share of the total supply grows once everyone else&apos;s rewards are
              netted out. It falls to zero as the staking ratio approaches 100%, because then
              every holder is earning issuance at the same rate and no share moves.
              {ownValidators > 0 && result.networkAnnualIssuanceAtParticipation > 0 ? (
                <>
                  {' '}
                  Your {num(ownValidators)}{' '}
                  {ownValidators === 1 ? 'validator captures' : 'validators capture'}{' '}
                  {pct(
                    (ownValidators * result.consensusRewardPerValidator) /
                      result.networkAnnualIssuanceAtParticipation,
                    3,
                  )}{' '}
                  of new issuance, which is the share of the staked set you hold.
                  {result.remainingEmission > 0 ? (
                    <>
                      {' '}
                      Against the {num(MAX_SUPPLY_QUANTA / 1e6)}M cap that is{' '}
                      {pct(result.remainingEmissionShare, 4)} a year of the{' '}
                      {compact(result.remainingEmission)} {NATIVE_UNIT} still to be issued. The
                      testnet issuance formula does not enforce the cap yet, so treat the pool as
                      context.
                    </>
                  ) : null}
                </>
              ) : null}
            </p>
          </section>

          {ownValidators > 0 ? (
            <section aria-label="Your payout" className="card p-5">
              <div className="flex items-baseline justify-between gap-3 flex-wrap mb-4">
                <h3 className="text-xs uppercase tracking-wider text-text-secondary">
                  Payout on {num(ownValidators)}{' '}
                  {ownValidators === 1 ? 'validator' : 'validators'}
                </h3>
                <span className="text-xs text-text-muted">
                  {quanta(result.ownStake, 0)} at stake
                  {price > 0 ? ` (${usd(result.ownStakeUsd)})` : ''}
                </span>
              </div>

              <div className="grid grid-cols-2 md:grid-cols-4 gap-px bg-border rounded-xl overflow-hidden">
                {(
                  [
                    ['Per epoch', result.epochReward, 4],
                    ['Per day', result.dailyReward, 3],
                    ['Per month', result.monthlyReward, 2],
                    ['Per year', result.annualReward, 2],
                  ] as const
                ).map(([label, value, digits]) => (
                  <div key={label} className="bg-background-secondary px-4 py-3">
                    <div className="text-xs uppercase tracking-wider text-text-secondary mb-1">{label}</div>
                    <div className="font-display text-base font-semibold text-text-primary">
                      {num(value, digits)}
                    </div>
                    {price > 0 ? (
                      <div className="text-[11px] text-text-muted mt-0.5">{usd(value * price)}</div>
                    ) : null}
                  </div>
                ))}
              </div>

              <div className="mt-4 pt-4 border-t border-border grid sm:grid-cols-2 gap-4 text-xs text-text-secondary">
                <div>
                  <div className="text-xs uppercase tracking-wider text-text-secondary mb-1">Rewards do not compound</div>
                  <p className="leading-relaxed">
                    Effective balance is capped at{' '}
                    {num(MAX_EFFECTIVE_BALANCE_QUANTA)} {NATIVE_UNIT}, and the
                    withdrawal sweep pays anything above it out to your withdrawal address. APR and
                    APY are the same number here. Funding another validator from rewards alone takes{' '}
                    <span className="text-text-primary font-medium">
                      {humanDuration(result.yearsToNextValidator)}
                    </span>
                    .
                  </p>
                </div>
                <div>
                  <div className="text-xs uppercase tracking-wider text-text-secondary mb-1">Duty frequency</div>
                  <p className="leading-relaxed">
                    You propose roughly{' '}
                    <span className="text-text-primary font-medium">
                      {result.proposalsPerYear.toFixed(0)}
                    </span>{' '}
                    blocks a year, about one every{' '}
                    <span className="text-text-primary font-medium">
                      {humanDuration((proposalGap * SECONDS_PER_EPOCH) / SECONDS_PER_YEAR)}
                    </span>
                    . Sync committee seats cover {pct(syncSeatShare, 1)} of the set and rotate every{' '}
                    {EPOCHS_PER_SYNC_COMMITTEE_PERIOD} epochs, so those rewards arrive in bursts.
                  </p>
                </div>
              </div>
            </section>
          ) : null}

          <section aria-label="Yield against network size" className="card p-5">
            <div className="flex items-baseline justify-between gap-3 flex-wrap mb-1">
              <h3 className="text-xs uppercase tracking-wider text-text-secondary">Yield against network size</h3>
              <span className="text-[11px] text-text-muted">
                gross APR, perfect duty
              </span>
            </div>
            <p className="text-xs text-text-secondary mb-4 max-w-2xl leading-relaxed">
              Issuance rises with the square root of total stake while stake rises linearly, so
              quadrupling the validator set halves the yield.
            </p>
            <AprCurve
              maxValidators={maxValidators}
              circulatingSupply={supply}
              currentValidators={snapshot.validators}
              scenarioValidators={networkValidators}
            />
          </section>

          <RewardBreakdown result={result} />
        </div>
      </div>

      {/* Parameters */}
      <section aria-label="Protocol parameters" className="card p-5">
        <h3 className="text-xs uppercase tracking-wider text-text-secondary mb-1">Protocol parameters</h3>
        <p className="text-xs text-text-secondary mb-4">
          Taken from the beacon-chain config in qrysm. These are what the calculator computes with.
        </p>
        <div className="grid grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-x-6 gap-y-3 text-xs">
          {(
            [
              ['Stake per validator', `${num(MAX_EFFECTIVE_BALANCE_QUANTA)} ${NATIVE_UNIT}`],
              ['Ejection balance', `${num(EJECTION_BALANCE_QUANTA)} ${NATIVE_UNIT}`],
              ['Slot time', `${SECONDS_PER_SLOT}s`],
              ['Slots per epoch', String(SLOTS_PER_EPOCH)],
              ['Epoch length', humanSeconds(SECONDS_PER_EPOCH)],
              ['Epochs per year', EPOCHS_PER_YEAR.toFixed(2)],
              ['Base reward factor', '2048'],
              ['Sync committee', `${SYNC_COMMITTEE_SIZE} seats`],
            ] as const
          ).map(([label, value]) => (
            <div key={label} className="flex items-baseline justify-between gap-2 border-b border-border pb-2">
              <span className="text-text-muted">{label}</span>
              <span className="font-display font-semibold text-text-primary">{value}</span>
            </div>
          ))}
        </div>
        <p className="text-[11px] text-text-muted mt-4 leading-relaxed">
          Estimates only. Actual returns vary with proposer and sync-committee luck, and fall with
          downtime, slashing, or an inactivity leak during a finality stall.
        </p>
      </section>
    </div>
  );
}
