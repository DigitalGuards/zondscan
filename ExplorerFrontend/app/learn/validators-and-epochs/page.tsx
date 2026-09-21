import type { Metadata } from 'next';
import Link from 'next/link';
import { learnMetadata } from '../components/learn-metadata';
import LearnArticle, {
  LearnSection,
  StepList,
  Step,
  Callout,
  LearnFAQ,
} from '../components/LearnArticle';

export const metadata: Metadata = learnMetadata('validators-and-epochs');

export default function Page(): JSX.Element {
  return (
    <LearnArticle slug="validators-and-epochs">
      <p>
        QRL 2.0 is a proof of stake network. A set of validators takes turns
        proposing blocks and voting on them, and the chain&apos;s clock is
        divided into slots and epochs. Once you understand those three ideas,
        every number on the ZondScan{' '}
        <Link href="/validators">validators dashboard</Link> and{' '}
        <Link href="/epochs/1">epoch browser</Link> starts to make sense.
      </p>
      <p>
        This article explains what a validator actually does, how slots and
        epochs structure time on the beacon chain, how a validator moves
        through its lifecycle from activation to exit, and how rewards reach a
        regular address. Along the way you will see exactly where each piece
        of data lives on ZondScan.
      </p>
      <p>
        If you are new to the chain itself, read{' '}
        <a href="/learn/what-is-qrl-2">What is QRL 2.0?</a> first. Everything
        below applies to the current public testnet, where coins have no
        monetary value.
      </p>

      <LearnSection id="what-a-validator-does" title="What a validator does">
        <p>
          QRL 2.0 splits work across two layers. The execution layer, gqrl,
          descends from go-ethereum: it runs EVM transactions and smart
          contracts. The consensus layer is a beacon chain run by qrysm, a
          fork of Prysm. Validators live on the consensus layer, and they have
          two jobs:
        </p>
        <ul>
          <li>
            <strong>Proposing.</strong> For each slot, the protocol selects
            one validator to build and sign the block for that slot. If the
            proposer is offline or late, the slot simply passes with no block.
          </li>
          <li>
            <strong>Attesting.</strong> All other active validators regularly
            vote on what they believe the head of the chain is. These
            attestations are aggregated and eventually make blocks final.
          </li>
        </ul>
        <p>
          Both duties are backed by stake. Honest, timely work earns rewards.
          Going offline costs a little. Provably signing contradictory
          messages costs a lot, as covered below.
        </p>
      </LearnSection>

      <LearnSection id="slots-and-epochs" title="Slots and epochs">
        <p>
          A <strong>slot</strong> is the smallest unit of time on the beacon
          chain: one opportunity for one proposer to publish one block. On QRL
          2.0 a slot lasts 60 seconds. An <strong>epoch</strong> groups 128
          consecutive slots, so one epoch takes 128 minutes, a little over two
          hours. Validator duties are shuffled and assigned per epoch, and the
          validator set itself only changes at epoch boundaries.
        </p>
        <p>
          Epochs are also the unit of finality. Through accumulated
          attestations, a recent epoch checkpoint first becomes{' '}
          <strong>justified</strong> and then <strong>finalized</strong>.
          Finalized blocks are permanent: reverting them would require a large
          share of all staked Quanta to be destroyed. ZondScan shows the
          current head epoch, the justified epoch, and the finalized epoch at
          the top of the <Link href="/validators">validators page</Link>, along with
          a live progress bar counting slots toward the next epoch. The{' '}
          <Link href="/epochs/1">epoch list</Link> labels each epoch as finalized,
          justified, or pending.
        </p>
        <p>
          Open any single epoch and you get its full 128-slot table: which
          slots got a block, which were missed, and for each proposed block
          the transaction count and gas used. A missed slot is ordinary
          network life; a validator that misses many of its assignments will
          show up with weak participation over time.
        </p>
      </LearnSection>

      <LearnSection id="post-quantum-validator-keys" title="Post-quantum validator keys">
        <p>
          Every duty a validator performs is a signature, so the signature
          scheme is the security foundation of the whole system. QRL 2.0
          validators sign with <strong>ML-DSA-87</strong>, the lattice based
          scheme standardized in NIST FIPS 204 and better known by its
          research name CRYSTALS-Dilithium. The legacy QRL chain uses XMSS, a
          stateful hash based scheme from RFC 8391. Both are designed to
          withstand attacks from quantum computers;{' '}
          <a href="/learn/why-post-quantum">Why QRL is built for the quantum era</a>{' '}
          explains the threat model in depth.
        </p>
        <p>
          One practical consequence is key size. An ML-DSA-87 public key is
          2,592 bytes, which is why ZondScan collapses the public key field on
          validator detail pages behind an expandable view.
        </p>
      </LearnSection>

      <LearnSection id="the-validator-lifecycle" title="The validator lifecycle">
        <p>
          A validator passes through a well defined sequence of epochs, and
          ZondScan renders the whole timeline on each validator&apos;s detail
          page:
        </p>
        <ul>
          <li>
            <strong>Activation eligibility.</strong> The epoch at which the
            validator&apos;s deposit was recognized and it joined the
            activation queue.
          </li>
          <li>
            <strong>Activation.</strong> The epoch it entered the active set
            and began receiving duties. From here ZondScan counts its age in
            epochs and days.
          </li>
          <li>
            <strong>Exit.</strong> The epoch it leaves the active set, either
            voluntarily or by force. For a healthy running validator this
            shows as &quot;Not scheduled&quot;.
          </li>
          <li>
            <strong>Withdrawable.</strong> The epoch after exit at which the
            remaining stake becomes available for withdrawal.
          </li>
        </ul>
        <p>
          From these epochs ZondScan derives one of four statuses for every
          validator: <strong>active</strong>, <strong>pending</strong>{' '}
          (deposited, waiting for activation), <strong>exited</strong>, or{' '}
          <strong>slashed</strong>.
        </p>
        <p>
          Each validator also carries an <strong>effective balance</strong>: a
          smoothed, whole-Quanta measure of its stake that the protocol uses
          for duty weighting and rewards. In the current network
          configuration it is capped at 40,000 Quanta, so a validator at full
          weight has 40,000 Quanta at stake. If a validator&apos;s balance
          decays below 20,000 Quanta, the protocol ejects it from the active
          set.
        </p>
        <p>
          <strong>Slashing</strong> is the penalty for provable misbehavior.
          If a validator signs two different blocks for the same slot, or
          signs contradictory attestations, those signatures are cryptographic
          evidence anyone can submit on chain. The protocol destroys part of
          the offender&apos;s stake and forces an exit. Slashed validators are
          flagged with their own badge on the{' '}
          <Link href="/validators">validators dashboard</Link>.
        </p>
      </LearnSection>

      <LearnSection id="rewards-and-withdrawals" title="Rewards and withdrawals">
        <p>
          Rewards for timely attestations and proposals accrue to the
          validator&apos;s balance on the beacon chain. Payout happens through
          protocol level withdrawals: the network periodically sweeps balance
          above the effective balance cap to the validator&apos;s registered
          withdrawal address, and after a validator exits and reaches its
          withdrawable epoch, the full remaining stake is paid out the same
          way. Withdrawals are a built in protocol operation, so there is no
          claim transaction to send.
        </p>
        <p>
          The withdrawal address is an ordinary Q-prefixed account. ZondScan
          decodes it from the validator&apos;s withdrawal credentials and
          links it directly on the detail page, so you can follow rewards
          landing on a normal address page. All amounts display in Quanta;
          see <a href="/learn/quanta-shor-planck">Quanta, Shor and Planck</a>{' '}
          for how the units relate.
        </p>
      </LearnSection>

      <LearnSection id="follow-it-on-zondscan" title="Follow it all on ZondScan">
        <StepList>
          <Step title="Open the validators dashboard">
            <p>
              Go to <Link href="/validators">zondscan.com/validators</Link>. The top
              panel shows the current epoch and slot, the finalized and
              justified epochs, and a countdown to the next epoch. Below it,
              stat cards break the validator set into total, active, pending,
              exited, and slashed, next to the total staked amount in Quanta.
            </p>
          </Step>
          <Step title="Inspect a single validator">
            <p>
              Click a validator&apos;s index number in the table below the
              charts. The detail page shows the
              validator&apos;s status, effective balance, age, ML-DSA-87
              public key, withdrawal credentials with the decoded withdrawal
              address, and the full epoch timeline from activation eligibility
              to withdrawable.
            </p>
          </Step>
          <Step title="Browse epochs">
            <p>
              Open the <Link href="/epochs/1">epoch list</Link> to see recent epochs
              with their finality status, validator counts, and total stake.
            </p>
          </Step>
          <Step title="Drill into one epoch">
            <p>
              Click an epoch number to open its detail page: a summary of
              proposed versus missed slots plus the full slot-by-slot table,
              with each proposed block linking to the block and its
              transactions.
            </p>
          </Step>
        </StepList>
        <Callout type="tip" title="Stake without running a validator">
          <p>
            Running a validator means operating a machine around the clock
            and locking a full stake. If you want staking exposure without
            that overhead, <a href="https://quantapool.com">QuantaPool</a>{' '}
            offers liquid staking on the testnet: you stake Quanta and receive
            a liquid staking token that represents your position. The
            walkthrough is in{' '}
            <a href="/learn/stake-with-quantapool">Liquid staking with QuantaPool</a>.
          </p>
        </Callout>
        <Callout type="info" title="Testnet numbers">
          <p>
            Slot timing, epoch length, and balance thresholds on this page
            reflect the current public testnet configuration and may change
            before mainnet. Testnet Quanta has no monetary value.
          </p>
        </Callout>
      </LearnSection>

      <LearnFAQ
        items={[
          {
            q: 'How long is an epoch on QRL 2.0?',
            a: (
              <p>
                One epoch is 128 slots of 60 seconds each, which works out to
                128 minutes, or 2 hours and 8 minutes. The{' '}
                <Link href="/validators">validators page</Link> shows a live
                countdown to the next epoch boundary.
              </p>
            ),
          },
          {
            q: 'Do I need to run a validator to earn staking rewards?',
            a: (
              <p>
                No. <a href="/learn/stake-with-quantapool">QuantaPool</a>{' '}
                provides liquid staking on the testnet: you stake Quanta
                through the pool, receive a liquid staking token, and track
                your position on ZondScan.
              </p>
            ),
          },
          {
            q: 'What does the slashed status mean?',
            a: (
              <p>
                The validator was caught signing contradictory messages, such
                as two different blocks for the same slot. The protocol
                destroyed part of its stake and forced it out of the active
                set. Slashing punishes provable misbehavior; simply being
                offline only causes small inactivity penalties.
              </p>
            ),
          },
          {
            q: 'Why does a validator show "Not scheduled" for its exit epoch?',
            a: (
              <p>
                The protocol represents &quot;no exit requested&quot; with a
                far-future epoch value. ZondScan renders that as Not
                scheduled, which means the validator is expected to keep
                operating indefinitely.
              </p>
            ),
          },
          {
            q: 'Where do validator rewards actually arrive?',
            a: (
              <p>
                At the validator&apos;s withdrawal address, a normal
                Q-prefixed account. ZondScan links it from each validator
                detail page, and the incoming protocol withdrawals appear on
                that address like any other balance change, denominated in
                Quanta.
              </p>
            ),
          },
        ]}
      />
    </LearnArticle>
  );
}
