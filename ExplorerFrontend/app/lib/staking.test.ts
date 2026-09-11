import {
  BASE_REWARD_FACTOR,
  EFFECTIVE_BALANCE_INCREMENT,
  EPOCHS_PER_YEAR,
  MAX_EFFECTIVE_BALANCE_QUANTA,
  MAX_SUPPLY_QUANTA,
  SECONDS_PER_EPOCH,
  SHOR_PER_QUANTA,
  baseRewardPerEpoch,
  baseRewardPerIncrement,
  computeStaking,
  grossAprForValidators,
  integerSquareRoot,
  quantaToValidators,
  validatorsToQuanta,
  type StakingScenario,
} from './staking';

const baseScenario: StakingScenario = {
  networkValidators: 512,
  ownValidators: 1,
  uptime: 1,
  participation: 1,
  priorityFeePerBlock: 0,
  circulatingSupply: 80_388_299,
  priceUsd: 0,
};

const scenario = (overrides: Partial<StakingScenario> = {}): StakingScenario => ({
  ...baseScenario,
  ...overrides,
});

describe('consensus constants', () => {
  it('matches the qrysm beacon config', () => {
    // Guards against a silent drift from config/params/mainnet_config.go.
    expect(SECONDS_PER_EPOCH).toBe(7680);
    expect(BASE_REWARD_FACTOR).toBe(BigInt(2048));
    expect(EFFECTIVE_BALANCE_INCREMENT).toBe(BigInt(1_000_000_000));
    expect(MAX_EFFECTIVE_BALANCE_QUANTA).toBe(40_000);
    expect(EPOCHS_PER_YEAR).toBeCloseTo(4109.0625, 4);
  });
});

describe('integerSquareRoot', () => {
  it('returns the floor of the square root', () => {
    expect(integerSquareRoot(BigInt(0))).toBe(BigInt(0));
    expect(integerSquareRoot(BigInt(1))).toBe(BigInt(1));
    expect(integerSquareRoot(BigInt(2))).toBe(BigInt(1));
    expect(integerSquareRoot(BigInt(15))).toBe(BigInt(3));
    expect(integerSquareRoot(BigInt(16))).toBe(BigInt(4));
    expect(integerSquareRoot(BigInt(24))).toBe(BigInt(4));
  });

  it('stays exact past the float53 boundary', () => {
    // 4e15 Shor = 4M Quanta staked. Math.sqrt would round here.
    expect(integerSquareRoot(BigInt(4_000_000_000_000_000))).toBe(BigInt(63_245_553));
    const big = BigInt(12_345_678_901_234_567_890);
    const root = integerSquareRoot(big);
    expect(root * root).toBeLessThanOrEqual(big);
    expect((root + BigInt(1)) * (root + BigInt(1))).toBeGreaterThan(big);
  });

  it('rejects negative input', () => {
    expect(() => integerSquareRoot(-BigInt(1))).toThrow(RangeError);
  });
});

describe('baseRewardPerIncrement', () => {
  it('applies integer division against the integer square root', () => {
    // 4M Quanta staked: 1e9 * 2048 / 63_245_553 = 32381.72 -> 32381.
    const total = BigInt(4_000_000) * SHOR_PER_QUANTA;
    expect(baseRewardPerIncrement(total)).toBe(BigInt(32_381));
  });

  it('is zero for an empty validator set rather than dividing by zero', () => {
    expect(baseRewardPerIncrement(BigInt(0))).toBe(BigInt(0));
  });
});

describe('baseRewardPerEpoch', () => {
  it('scales the per-increment reward by the validator increments', () => {
    const effective = BigInt(MAX_EFFECTIVE_BALANCE_QUANTA) * SHOR_PER_QUANTA;
    const total = BigInt(4_000_000) * SHOR_PER_QUANTA;
    // 40_000 increments x 32_381 Shor.
    expect(baseRewardPerEpoch(effective, total)).toBe(BigInt(1_295_240_000));
  });
});

describe('network issuance', () => {
  // Reference table published by Cyyber (QRL core dev) in Discord, Jan 2026.
  // Reproducing it confirms this module models the same protocol the client
  // implements. Tolerance is 0.5%: the published figures use a slightly
  // different seconds-per-year convention, everything else agrees exactly.
  it.each([
    [100, 531_716.246890984],
    [500, 1_188_935.344535248],
    [1000, 1_681_471.998554688],
    [1500, 2_059_146.367921256],
  ])('matches the published estimate for %i validators', (validators, expected) => {
    const result = computeStaking(scenario({ networkValidators: validators }));
    expect(result.networkAnnualIssuance).toBeGreaterThan(expected * 0.995);
    expect(result.networkAnnualIssuance).toBeLessThan(expected * 1.005);
  });

  it('reproduces the published ROI band', () => {
    // Cyyber quotes 12-13% at 100 validators and 3.2-3.5% at 1500.
    expect(grossAprForValidators(100)).toBeGreaterThan(0.12);
    expect(grossAprForValidators(100)).toBeLessThan(0.14);
    expect(grossAprForValidators(1500)).toBeGreaterThan(0.03);
    expect(grossAprForValidators(1500)).toBeLessThan(0.036);
  });
});

describe('dilution', () => {
  it('halves the yield when the validator set quadruples', () => {
    // Issuance grows with sqrt(stake), stake grows linearly: APR ~ 1/sqrt(n).
    const at512 = grossAprForValidators(512);
    const at2048 = grossAprForValidators(2048);
    expect(at2048).toBeCloseTo(at512 / 2, 4);
  });

  it('is monotonically decreasing in validator count', () => {
    const counts = [128, 256, 512, 1024, 4096, 16384];
    const aprs = counts.map(grossAprForValidators);
    for (let i = 1; i < aprs.length; i += 1) {
      expect(aprs[i]).toBeLessThan(aprs[i - 1]);
    }
  });
});

describe('computeStaking', () => {
  it('pays the whole base reward when uptime and participation are perfect', () => {
    const result = computeStaking(scenario());
    const { source, target, head, sync, proposer } = result.breakdown;
    const summed = source + target + head + sync + proposer;
    // The five Altair weights total WEIGHT_DENOMINATOR, so a perfect epoch
    // pays exactly one base reward and netApr meets the gross ceiling.
    expect(summed).toBeCloseTo(result.baseRewardPerEpoch * EPOCHS_PER_YEAR, 6);
    expect(result.netApr).toBeCloseTo(result.grossApr, 12);
  });

  it('reports the live testnet yield', () => {
    // 512 validators x 40_000 Quanta = 20.48M staked, which is what
    // /api/validators/stats reports today.
    const result = computeStaking(scenario());
    expect(result.totalStaked).toBe(20_480_000);
    expect(result.grossApr).toBeGreaterThan(0.058);
    expect(result.grossApr).toBeLessThan(0.06);
  });

  it('penalises missed source and target but not missed head', () => {
    const perfect = computeStaking(scenario());
    const flaky = computeStaking(scenario({ uptime: 0.9 }));
    // A 10% miss rate costs more than 10% of pay, because source and target
    // are clawed back on top of going unpaid.
    expect(flaky.netApr).toBeLessThan(perfect.netApr * 0.9);
    // Head is never penalised, so the loss stops short of the naive doubling.
    expect(flaky.netApr).toBeGreaterThan(perfect.netApr * 0.8);
  });

  it('scales attestation pay by network participation', () => {
    const full = computeStaking(scenario());
    const degraded = computeStaking(scenario({ participation: 0.8 }));
    expect(degraded.netApr).toBeLessThan(full.netApr);
    expect(degraded.netApr).toBeCloseTo(full.netApr * 0.8, 6);
  });

  it('adds priority fees on top of consensus rewards', () => {
    const noFees = computeStaking(scenario());
    const withFees = computeStaking(scenario({ priorityFeePerBlock: 0.01 }));
    expect(withFees.aprWithFees).toBeGreaterThan(noFees.aprWithFees);
    // 128 slots/epoch shared over 512 validators = a proposal every 4 epochs.
    expect(withFees.proposalsPerYear).toBeCloseTo((128 * EPOCHS_PER_YEAR) / 512, 6);
    expect(withFees.feeRewardPerValidator).toBeCloseTo(withFees.proposalsPerYear * 0.01, 6);
  });

  it('discounts the nominal yield by supply inflation', () => {
    const result = computeStaking(scenario());
    expect(result.inflationRate).toBeGreaterThan(0);
    expect(result.realYield).toBeLessThan(result.aprWithFees);
    const expected = (1 + result.aprWithFees) / (1 + result.inflationRate) - 1;
    expect(result.realYield).toBeCloseTo(expected, 12);
  });

  it('lands on a zero real yield when the whole supply is staked', () => {
    // Reported from Discord as a units bug ("subtracting the % as an
    // integer"). It is the intended result: at a 100% staking ratio the APR
    // and the inflation rate are the same number, so no holder gains share.
    // Both terms are fractions, and the discount stays multiplicative.
    const full = computeStaking(
      scenario({ networkValidators: quantaToValidators(baseScenario.circulatingSupply) }),
    );
    expect(full.stakingRatio).toBeGreaterThan(0.999);
    expect(full.inflationRate).toBeCloseTo(full.aprWithFees, 4);
    expect(Math.abs(full.realYield)).toBeLessThan(0.0005);

    // Below full staking the discount leaves a real gain, and priority fees
    // are a transfer instead of issuance, so they survive the discount.
    const half = computeStaking({ ...baseScenario, networkValidators: 1004 });
    expect(half.stakingRatio).toBeGreaterThan(0.49);
    expect(half.stakingRatio).toBeLessThan(0.51);
    // Half the supply staked means half the APR shows up as inflation, so
    // roughly half the nominal rate survives (a shade under, the discount
    // being multiplicative).
    expect(half.realYield).toBeGreaterThan(half.aprWithFees * 0.45);
    expect(half.realYield).toBeLessThan(half.aprWithFees * 0.55);
    const withFees = computeStaking(
      scenario({
        networkValidators: quantaToValidators(baseScenario.circulatingSupply),
        priorityFeePerBlock: 0.01,
      }),
    );
    expect(withFees.realYield).toBeGreaterThan(0);
  });

  it('reports the share of the capped emission pool a validator draws down', () => {
    const result = computeStaking(scenario());
    expect(result.remainingEmission).toBe(MAX_SUPPLY_QUANTA - baseScenario.circulatingSupply);
    expect(result.remainingEmissionShare).toBeCloseTo(
      result.consensusRewardPerValidator / result.remainingEmission,
      12,
    );
    // Past the cap there is no pool left to share.
    const exhausted = computeStaking(scenario({ circulatingSupply: MAX_SUPPLY_QUANTA }));
    expect(exhausted.remainingEmission).toBe(0);
    expect(exhausted.remainingEmissionShare).toBe(0);
  });

  it('treats a zero supply as unknown instead of dividing by zero', () => {
    const result = computeStaking(scenario({ circulatingSupply: 0 }));
    expect(result.inflationRate).toBe(0);
    expect(result.stakingRatio).toBe(0);
    expect(Number.isFinite(result.realYield)).toBe(true);
  });

  it('scales operator totals by validator count', () => {
    const one = computeStaking(scenario({ ownValidators: 1 }));
    const ten = computeStaking(scenario({ ownValidators: 10 }));
    expect(ten.ownStake).toBe(400_000);
    expect(ten.annualReward).toBeCloseTo(one.annualReward * 10, 6);
    // Yield is per-stake, so running more validators does not raise the rate.
    expect(ten.aprWithFees).toBeCloseTo(one.aprWithFees, 12);
  });

  it('keeps the period breakdown internally consistent', () => {
    const result = computeStaking(scenario());
    expect(result.monthlyReward * 12).toBeCloseTo(result.annualReward, 6);
    expect(result.dailyReward * 365.25).toBeCloseTo(result.annualReward, 6);
    expect(result.epochReward * EPOCHS_PER_YEAR).toBeCloseTo(result.annualReward, 6);
  });

  it('reports the wait to self-fund another validator', () => {
    // Rewards above the 40,000 cap are swept out, so nothing compounds:
    // growth means saving up a whole new validator.
    const result = computeStaking(scenario());
    expect(result.yearsToNextValidator).toBeCloseTo(
      MAX_EFFECTIVE_BALANCE_QUANTA / result.annualReward,
      6,
    );
    const idle = computeStaking(scenario({ ownValidators: 0 }));
    expect(idle.annualReward).toBe(0);
    expect(idle.yearsToNextValidator).toBe(Infinity);
  });

  it('converts to USD only when a price is supplied', () => {
    const unpriced = computeStaking(scenario());
    expect(unpriced.annualRewardUsd).toBe(0);
    const priced = computeStaking(scenario({ priceUsd: 0.7171 }));
    expect(priced.ownStakeUsd).toBeCloseTo(40_000 * 0.7171, 6);
    expect(priced.annualRewardUsd).toBeCloseTo(priced.annualReward * 0.7171, 6);
  });

  it('clamps out-of-range inputs instead of producing nonsense', () => {
    const result = computeStaking(
      scenario({
        networkValidators: 0,
        ownValidators: -5,
        uptime: 4,
        participation: -1,
        priorityFeePerBlock: -3,
        priceUsd: -2,
      }),
    );
    expect(result.totalStaked).toBe(MAX_EFFECTIVE_BALANCE_QUANTA);
    expect(result.ownStake).toBe(0);
    expect(result.feeRewardPerValidator).toBe(0);
    expect(result.annualRewardUsd).toBe(0);
    expect(Number.isNaN(result.netApr)).toBe(false);
  });
});

describe('stake and validator conversion', () => {
  it('round-trips whole validators', () => {
    expect(validatorsToQuanta(512)).toBe(20_480_000);
    expect(quantaToValidators(20_480_000)).toBe(512);
  });

  it('floors partial stakes, since activation needs the full balance', () => {
    expect(quantaToValidators(39_999)).toBe(0);
    expect(quantaToValidators(79_999)).toBe(1);
    expect(validatorsToQuanta(-3)).toBe(0);
  });
});
