/**
 * QRL 2.0 staking reward math.
 *
 * Every constant below mirrors the beacon-chain config in qrysm
 * (`config/params/mainnet_config.go`). The reward formula mirrors
 * `beacon-chain/core/altair/reward.go` and the attestation deltas in
 * `beacon-chain/core/altair/epoch_precompute.go`. Keep the two in sync:
 * if the network re-parameterises (the 64-byte relaunch is expected to),
 * this file is the single place the explorer has to change.
 *
 * Balances are Shor-denominated (1 Quanta = 10^9 Shor) because that is the
 * consensus-layer unit. Execution-layer amounts are Planck (10^18) and never
 * appear here.
 */

// ---------------------------------------------------------------------------
// Consensus constants
// ---------------------------------------------------------------------------

// ES2017 target: BigInt literals (0n) are unavailable, so every bigint here
// is built with the BigInt() call form, matching app/lib/helpers.ts.
const ZERO = BigInt(0);
const ONE = BigInt(1);
const TWO = BigInt(2);

export const SHOR_PER_QUANTA = BigInt(1_000_000_000);

export const SECONDS_PER_SLOT = 60;
export const SLOTS_PER_EPOCH = 128;
export const SECONDS_PER_EPOCH = SECONDS_PER_SLOT * SLOTS_PER_EPOCH; // 7680s = 2h8m

export const BASE_REWARD_FACTOR = BigInt(2048);
export const EFFECTIVE_BALANCE_INCREMENT = SHOR_PER_QUANTA; // 1 Quanta

/**
 * A validator must deposit exactly this to activate: `IsActiveValidator`
 * requires `effectiveBalance == MaxEffectiveBalance` (qrysm
 * helpers/validators.go). Rewards accrued above it are swept out by the
 * withdrawal sweep, so a single validator never compounds.
 */
export const MAX_EFFECTIVE_BALANCE_QUANTA = 40_000;
/** Below this the validator is ejected from the active set. */
export const EJECTION_BALANCE_QUANTA = 20_000;

/**
 * Network-wide supply ceiling in Quanta. The 105M cap is QRL policy: QIP-016
 * keeps it through the proof-of-stake transition, with staking rewards meant
 * to draw down the pool still to be issued. The beacon-chain testnet does not
 * enforce it yet: issuance there is the bare sqrt(stake) formula above, so
 * the calculator reports the pool for context and leaves the formula alone.
 */
export const MAX_SUPPLY_QUANTA = 105_000_000;

export const SYNC_COMMITTEE_SIZE = 128;
export const EPOCHS_PER_SYNC_COMMITTEE_PERIOD = 8;

// Altair incentive weights. These sum to WEIGHT_DENOMINATOR, which is why
// full network issuance per epoch equals the sum of every validator's base
// reward exactly.
export const TIMELY_SOURCE_WEIGHT = 14;
export const TIMELY_TARGET_WEIGHT = 26;
export const TIMELY_HEAD_WEIGHT = 14;
export const SYNC_REWARD_WEIGHT = 2;
export const PROPOSER_WEIGHT = 8;
export const WEIGHT_DENOMINATOR = 64;

/** Julian year, so leap years average out over a multi-year horizon. */
export const SECONDS_PER_YEAR = 31_557_600;
export const EPOCHS_PER_YEAR = SECONDS_PER_YEAR / SECONDS_PER_EPOCH; // 4109.0625

// ---------------------------------------------------------------------------
// Protocol primitives (exact integer arithmetic)
// ---------------------------------------------------------------------------

/**
 * Newton's method integer square root, matching the spec's
 * `integer_squareroot` (and qrysm's `math.CachedSquareRoot`). The floor
 * matters: `baseRewardPerIncrement` divides by it under integer division,
 * so using Math.sqrt here would drift from consensus by a few Shor per
 * validator per epoch.
 */
export function integerSquareRoot(n: bigint): bigint {
  if (n < ZERO) throw new RangeError('integerSquareRoot: negative input');
  if (n < TWO) return n;
  let x = n;
  let y = (x + ONE) / TWO;
  while (y < x) {
    x = y;
    y = (x + n / x) / TWO;
  }
  return x;
}

/**
 * `EFFECTIVE_BALANCE_INCREMENT * BASE_REWARD_FACTOR / sqrt(totalActiveBalance)`,
 * in Shor. Integer division, exactly as the client computes it.
 */
export function baseRewardPerIncrement(totalActiveBalanceShor: bigint): bigint {
  if (totalActiveBalanceShor <= ZERO) return ZERO;
  return (
    (EFFECTIVE_BALANCE_INCREMENT * BASE_REWARD_FACTOR) /
    integerSquareRoot(totalActiveBalanceShor)
  );
}

/**
 * One validator's per-epoch base reward in Shor: the whole pot it can earn
 * across all five duty weights in a perfect epoch.
 */
export function baseRewardPerEpoch(
  effectiveBalanceShor: bigint,
  totalActiveBalanceShor: bigint,
): bigint {
  const increments = effectiveBalanceShor / EFFECTIVE_BALANCE_INCREMENT;
  return increments * baseRewardPerIncrement(totalActiveBalanceShor);
}

// ---------------------------------------------------------------------------
// Scenario model
// ---------------------------------------------------------------------------

export interface StakingScenario {
  /** Active validators network-wide. Each holds MAX_EFFECTIVE_BALANCE. */
  networkValidators: number;
  /** How many validators the operator runs. */
  ownValidators: number;
  /** Operator's own share of duties performed correctly, 0..1. */
  uptime: number;
  /** Network-wide attestation participation, 0..1. Scales attestation pay. */
  participation: number;
  /** Average priority fee (tip) captured per block proposed, in Quanta. */
  priorityFeePerBlock: number;
  /** Circulating supply in Quanta, for the inflation and real-yield figures. */
  circulatingSupply: number;
  /** Quanta price in USD. Zero hides the fiat column. */
  priceUsd: number;
}

export interface StakingResult {
  // Network
  totalStaked: number;
  stakingRatio: number;
  /** Issuance if every validator performed perfectly, Quanta per year. */
  networkAnnualIssuance: number;
  /** Issuance actually paid at the scenario's participation rate. */
  networkAnnualIssuanceAtParticipation: number;
  inflationRate: number;

  // Per validator
  baseRewardPerEpoch: number;
  /** Consensus rewards for one validator, Quanta per year. */
  consensusRewardPerValidator: number;
  proposalsPerYear: number;
  feeRewardPerValidator: number;
  totalRewardPerValidator: number;

  // Rates
  /** Ceiling: perfect uptime, perfect network participation, no fees. */
  grossApr: number;
  /** What the scenario's uptime and participation actually pay. */
  netApr: number;
  /** netApr including priority fees. */
  aprWithFees: number;
  /**
   * aprWithFees discounted by supply inflation: how fast the operator's share
   * of the total supply grows. At a 100% staking ratio it is zero by
   * construction, because every holder then earns issuance at the same rate.
   * `aprWithFees` remains the gain against an unstaked balance.
   */
  realYield: number;
  /** Quanta still to be issued under MAX_SUPPLY_QUANTA, 0 once exhausted. */
  remainingEmission: number;
  /** Operator's consensus rewards as a share of remainingEmission, per year. */
  remainingEmissionShare: number;

  // Operator totals
  ownStake: number;
  annualReward: number;
  monthlyReward: number;
  dailyReward: number;
  epochReward: number;
  annualRewardUsd: number;
  ownStakeUsd: number;
  /** Years until rewards fund one more 40,000 Quanta validator. */
  yearsToNextValidator: number;

  // Duty mix, Quanta per validator per year
  breakdown: {
    source: number;
    target: number;
    head: number;
    sync: number;
    proposer: number;
    fees: number;
  };
}

const QUANTA = (shor: bigint): number => Number(shor) / Number(SHOR_PER_QUANTA);

const clamp01 = (n: number): number => (Number.isFinite(n) ? Math.min(1, Math.max(0, n)) : 0);

/**
 * Expected annual return for one validator, decomposed by duty.
 *
 * Attestation pay is spec-exact in shape: each flag pays
 * `baseReward * weight/64 * participatingBalance/activeBalance`, and missing
 * source or target costs `baseReward * weight/64` outright. Head has a reward
 * but no penalty, which is why a flaky validator loses less than a naive
 * "uptime x APR" model predicts.
 *
 * Proposer and sync-committee pay is modelled at expectation. Both are
 * lotteries: with a 128-seat sync committee rotating every 8 epochs, a small
 * validator set means a large slice of validators is seated at any moment and
 * individual months swing hard around this mean. Over a year the expectation
 * is the right planning number.
 */
export function computeStaking(scenario: StakingScenario): StakingResult {
  const networkValidators = Math.max(1, Math.floor(scenario.networkValidators));
  const ownValidators = Math.max(0, Math.floor(scenario.ownValidators));
  const uptime = clamp01(scenario.uptime);
  const participation = clamp01(scenario.participation);

  const effectiveBalanceShor = BigInt(MAX_EFFECTIVE_BALANCE_QUANTA) * SHOR_PER_QUANTA;
  const totalActiveBalanceShor = BigInt(networkValidators) * effectiveBalanceShor;

  const baseShor = baseRewardPerEpoch(effectiveBalanceShor, totalActiveBalanceShor);
  const base = QUANTA(baseShor);

  const w = (weight: number): number => (base * weight) / WEIGHT_DENOMINATOR;

  // Per epoch, per validator. Source and target carry a penalty on a miss;
  // head does not; sync is symmetric while seated.
  const missed = 1 - uptime;
  const perEpoch = {
    source: w(TIMELY_SOURCE_WEIGHT) * (uptime * participation - missed),
    target: w(TIMELY_TARGET_WEIGHT) * (uptime * participation - missed),
    head: w(TIMELY_HEAD_WEIGHT) * (uptime * participation),
    sync: w(SYNC_REWARD_WEIGHT) * (uptime * participation - missed),
    proposer: w(PROPOSER_WEIGHT) * (uptime * participation),
  };

  const breakdown = {
    source: perEpoch.source * EPOCHS_PER_YEAR,
    target: perEpoch.target * EPOCHS_PER_YEAR,
    head: perEpoch.head * EPOCHS_PER_YEAR,
    sync: perEpoch.sync * EPOCHS_PER_YEAR,
    proposer: perEpoch.proposer * EPOCHS_PER_YEAR,
    fees: 0,
  };

  const consensusRewardPerValidator =
    breakdown.source + breakdown.target + breakdown.head + breakdown.sync + breakdown.proposer;

  // Every slot is a proposal opportunity shared across the active set.
  const proposalsPerYear = (SLOTS_PER_EPOCH * EPOCHS_PER_YEAR) / networkValidators;
  const feeRewardPerValidator =
    proposalsPerYear * Math.max(0, scenario.priorityFeePerBlock) * uptime;
  breakdown.fees = feeRewardPerValidator;

  const totalRewardPerValidator = consensusRewardPerValidator + feeRewardPerValidator;

  const stakePerValidator = MAX_EFFECTIVE_BALANCE_QUANTA;
  const totalStaked = networkValidators * stakePerValidator;

  // Ceiling case: every weight paid in full, which is exactly the base reward.
  const grossApr = (base * EPOCHS_PER_YEAR) / stakePerValidator;
  const netApr = consensusRewardPerValidator / stakePerValidator;
  const aprWithFees = totalRewardPerValidator / stakePerValidator;

  const networkAnnualIssuance = base * EPOCHS_PER_YEAR * networkValidators;
  const networkAnnualIssuanceAtParticipation =
    consensusRewardPerValidator * networkValidators;

  const circulatingSupply = Math.max(0, scenario.circulatingSupply);
  const inflationRate =
    circulatingSupply > 0 ? networkAnnualIssuanceAtParticipation / circulatingSupply : 0;
  // Dilution is multiplicative: holding stake flat while supply grows. Both
  // terms are fractions (0.0295 = 2.95%), so the result is a fraction too.
  // As stakingRatio approaches 1 the two converge and this tends to zero,
  // which is the correct reading: with everyone staked, nobody gains share.
  const realYield = (1 + aprWithFees) / (1 + inflationRate) - 1;

  const ownStake = ownValidators * stakePerValidator;
  const annualReward = ownValidators * totalRewardPerValidator;

  // Fees are a transfer between holders, so only consensus issuance comes
  // out of the pool.
  const remainingEmission = Math.max(0, MAX_SUPPLY_QUANTA - circulatingSupply);
  const remainingEmissionShare =
    remainingEmission > 0 ? (ownValidators * consensusRewardPerValidator) / remainingEmission : 0;
  const priceUsd = Math.max(0, scenario.priceUsd);

  // Rewards above the 40,000 cap are swept to the withdrawal address rather
  // than compounding, so growth means funding an entire new validator.
  const yearsToNextValidator =
    annualReward > 0 ? MAX_EFFECTIVE_BALANCE_QUANTA / annualReward : Infinity;

  return {
    totalStaked,
    stakingRatio: circulatingSupply > 0 ? totalStaked / circulatingSupply : 0,
    networkAnnualIssuance,
    networkAnnualIssuanceAtParticipation,
    inflationRate,

    baseRewardPerEpoch: base,
    consensusRewardPerValidator,
    proposalsPerYear,
    feeRewardPerValidator,
    totalRewardPerValidator,

    grossApr,
    netApr,
    aprWithFees,
    realYield,
    remainingEmission,
    remainingEmissionShare,

    ownStake,
    annualReward,
    monthlyReward: annualReward / 12,
    dailyReward: annualReward / 365.25,
    epochReward: annualReward / EPOCHS_PER_YEAR,
    annualRewardUsd: annualReward * priceUsd,
    ownStakeUsd: ownStake * priceUsd,
    yearsToNextValidator,

    breakdown,
  };
}

/**
 * Gross APR as a function of validator count, for the dilution curve.
 * Issuance rises with the square root of total stake while stake rises
 * linearly, so the yield falls as 1/sqrt(validators).
 */
export function grossAprForValidators(networkValidators: number): number {
  const n = Math.max(1, Math.floor(networkValidators));
  const effectiveBalanceShor = BigInt(MAX_EFFECTIVE_BALANCE_QUANTA) * SHOR_PER_QUANTA;
  const totalActiveBalanceShor = BigInt(n) * effectiveBalanceShor;
  const base = QUANTA(baseRewardPerEpoch(effectiveBalanceShor, totalActiveBalanceShor));
  return (base * EPOCHS_PER_YEAR) / MAX_EFFECTIVE_BALANCE_QUANTA;
}

/** Quanta staked -> validators, both directions, for input round-tripping. */
export function validatorsToQuanta(validators: number): number {
  return Math.max(0, Math.floor(validators)) * MAX_EFFECTIVE_BALANCE_QUANTA;
}

export function quantaToValidators(quanta: number): number {
  return Math.max(0, Math.floor(quanta / MAX_EFFECTIVE_BALANCE_QUANTA));
}
