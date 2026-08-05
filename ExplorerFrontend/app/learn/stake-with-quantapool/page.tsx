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

export const metadata: Metadata = learnMetadata('stake-with-quantapool');

export default function Page(): JSX.Element {
  return (
    <LearnArticle slug="stake-with-quantapool">
      <p>
        Running your own validator on QRL 2.0 takes 40,000 Quanta of stake plus a machine
        that stays online around the clock. Liquid staking removes both requirements. You
        deposit Quanta into a shared pool, the pool funds validators, and you receive a
        token that represents your slice of the staked position. Your stake keeps earning
        while the token stays in your wallet, transferable like any other asset.
      </p>
      <p>
        This tutorial walks through{' '}
        <a href="https://quantapool.com">QuantaPool</a>, a liquid staking protocol running
        on the QRL 2.0 testnet. You will connect a wallet, deposit Quanta, receive the
        stQRL liquid staking token, and learn how rewards, withdrawals, and the maturity
        lock work. You will also see how to follow your position on ZondScan, since stQRL
        is an ordinary token contract with a public holders list and transfer history.
      </p>
      <p>
        Everything here happens on the public testnet. Testnet coins have no monetary
        value, so this is a risk-free way to learn the flow before mainnet arrives. If
        you have never followed staking at the consensus layer, read{' '}
        <a href="/learn/validators-and-epochs">validators, epochs and staking</a> first
        for the background.
      </p>

      <LearnSection id="what-is-liquid-staking" title="What liquid staking is">
        <p>
          Proof of stake networks pay rewards to validators, and each QRL 2.0 validator
          requires exactly 40,000 Quanta. A liquid staking pool aggregates deposits from
          many users: once the pool&apos;s buffer reaches a full validator stake, it funds a
          new validator through the beacon chain deposit contract. Rewards earned by
          those validators flow back to everyone in the pool, proportional to their
          share.
        </p>
        <p>
          In exchange for your deposit you receive a liquid staking token, often
          shortened to LST. It is the receipt for your position. Holding it entitles you
          to your share of the pooled stake and its accumulated rewards, and because it
          is a normal token you stay liquid: you can transfer it or later redeem it for
          the underlying Quanta.
        </p>
      </LearnSection>

      <LearnSection id="how-stqrl-works" title="How stQRL works">
        <p>
          QuantaPool&apos;s liquid staking token is named Staked QRL with the symbol{' '}
          <code>stQRL</code>. It uses a fixed-balance design: your balance is counted in
          shares, and the share count stays constant over time. Rewards show up in the
          exchange rate instead. Each stQRL share is redeemable for a growing amount of
          Quanta, and the app displays the live rate on the staking page.
        </p>
        <p>
          The exchange rate needs no external oracle. The pool detects validator rewards
          directly from on-chain balance increases, so there is no separate price feed to
          trust or compromise. The same mechanism handles slashing: if a pool validator
          is penalized, the exchange rate drops and the loss is shared proportionally
          across all holders.
        </p>
        <p>
          The staking widget lists the protocol fee as none, and the app&apos;s own FAQ
          states that the protocol currently charges no fees. Your deposit transaction
          still pays normal network gas. The deposit contract ships with a minimum
          deposit of 100 Quanta; the widget always shows the live minimum, since the
          operator can adjust it.
        </p>
      </LearnSection>

      <LearnSection id="stake-step-by-step" title="Stake Quanta, step by step">
        <StepList>
          <Step title="Get a wallet and testnet Quanta">
            <p>
              You need a Q-prefixed QRL 2.0 address with enough balance to cover the
              minimum deposit plus gas. Set one up with the{' '}
              <a href="/learn/create-a-wallet">wallet creation guide</a>, then fund it
              from the <Link href="/faucet">ZondScan faucet</Link> as described in{' '}
              <a href="/learn/get-testnet-quanta">get testnet Quanta</a>.
            </p>
          </Step>
          <Step title="Open QuantaPool and connect your wallet">
            <p>
              Go to <a href="https://quantapool.com">quantapool.com</a> and select{' '}
              <strong>Connect wallet to stake</strong>. A wallet picker opens and lists
              every QRL-capable wallet it discovers through EIP-6963: the QRL browser
              extension is detected automatically, and MyQRLWallet on mobile or desktop
              pairs through an encrypted relay by scanning a QR code. The pairing flow
              is covered in detail in{' '}
              <a href="/learn/connect-dapp-wallet">connect your wallet to a QRL dApp</a>.
            </p>
          </Step>
          <Step title="Enter an amount and review the preview">
            <p>
              Type the amount of Quanta to stake. Below the input the widget previews
              exactly what you will receive: the estimated stQRL shares, the current
              exchange rate expressed as Quanta per stQRL, the minimum deposit, and the
              protocol fee row.
            </p>
          </Step>
          <Step title="Confirm the deposit in your wallet">
            <p>
              Select <strong>Stake QRL</strong> and approve the transaction in your
              wallet. The app tracks the transaction and links it to ZondScan, where you
              can watch it confirm. The guide to{' '}
              <a href="/learn/read-a-transaction">reading a transaction</a> explains
              every field on that page.
            </p>
          </Step>
          <Step title="Check your position">
            <p>
              Once the deposit confirms, the position card under the staking widget
              updates. It shows your stQRL share balance, the current value of those
              shares in Quanta, and any shares locked in pending withdrawals.
            </p>
          </Step>
        </StepList>
        <Callout type="warning" title="Know the risks">
          <p>
            QuantaPool is testnet software under active development. Smart contract risk
            applies: a bug in the pool contracts could affect deposited funds. Validator
            slashing risk applies too, with losses shared across all holders through the
            exchange rate. The operator can also pause deposits and withdrawal requests
            during upgrades; the app shows a notice whenever that is the case, and the
            pool has been paused for stretches of its testnet rollout. Testnet coins
            have no monetary value, so nothing real is at stake today, and that is
            exactly why now is the time to practice. The protocol&apos;s own legal notice
            at{' '}
            <a href="https://quantapool.com/legal">quantapool.com/legal</a> spells out
            its testnet-only status.
          </p>
        </Callout>
      </LearnSection>

      <LearnSection id="maturity-and-withdrawals" title="Maturity lock and withdrawals">
        <p>
          Freshly minted stQRL must mature before it can move. The lock defaults to
          1,536 blocks, about a day at the testnet&apos;s 60 second block time. Until then
          the new shares can neither be transferred nor queued for withdrawal. Topping
          up restarts the timer for any shares that have not yet matured, so plan
          deposits accordingly. The Withdrawals page shows how much of your stQRL is
          still maturing and when it becomes available.
        </p>
        <p>Unstaking is a two-step flow on the app&apos;s Withdrawals page:</p>
        <ul>
          <li>
            <strong>Request.</strong> Enter the stQRL amount and submit a withdrawal
            request. Your shares are locked and the app shows an estimate of the Quanta
            you will receive. A 128-block delay starts, a little over two hours. You can
            cancel the request at any point before claiming and the shares simply
            unlock.
          </li>
          <li>
            <strong>Claim.</strong> Requests are claimed oldest first, one request per
            claim. At claim time the locked shares are burned at the settled exchange
            rate and you receive that amount of Quanta, so the final payout can differ
            slightly from the estimate. Locked shares keep earning rewards, and keep
            bearing any slashing losses, until the claim executes. If the pool&apos;s
            withdrawal reserve cannot yet cover your request, the claim waits until the
            operator replenishes the reserve, which can mean waiting for a validator
            exit.
          </li>
        </ul>
      </LearnSection>

      <LearnSection id="track-on-zondscan" title="Track your position on ZondScan">
        <p>
          Because stQRL is a standard token contract, everything about it is public.
          Your deposit appears as a transaction from your address to the deposit pool
          contract, and the mint of your shares shows up as a token transfer on that
          transaction&apos;s page.
        </p>
        <p>
          The stQRL contract itself has a full token page. Find it through the{' '}
          <Link href="/contracts">smart contracts list</Link> or by searching its address, then
          explore the tabs: the holders tab shows every address with a share balance,
          and the transfers tab shows the complete transfer history, including mints
          from deposits and burns from withdrawal claims.
        </p>
        <p>
          You can also watch the other side of the system. The validators that
          QuantaPool funds join the same validator set as everyone else, visible on the{' '}
          <Link href="/validators">validators page</Link>. The article on{' '}
          <a href="/learn/validators-and-epochs">validators and epochs</a> explains how
          their attestations and proposals turn into the rewards that raise the stQRL
          exchange rate.
        </p>
      </LearnSection>

      <LearnFAQ
        items={[
          {
            q: 'Do I need 40,000 Quanta to stake?',
            a: (
              <p>
                No. The pool aggregates deposits from many users and funds a validator
                whenever the buffer reaches a full 40,000 Quanta stake. You only need to
                meet the pool&apos;s minimum deposit, which the staking widget displays.
              </p>
            ),
          },
          {
            q: 'Why does my stQRL balance never change?',
            a: (
              <p>
                stQRL is a fixed-balance token. Your share count stays constant while
                the exchange rate between shares and Quanta grows as rewards accrue.
                The position card on the staking page shows the current Quanta value of
                your shares.
              </p>
            ),
          },
          {
            q: 'Can I transfer stQRL to another address?',
            a: (
              <p>
                Yes, once the shares have matured. Fresh deposits are locked for about a
                day, and after that stQRL transfers like any other token while the
                underlying stake keeps earning.
              </p>
            ),
          },
          {
            q: 'How long does unstaking take?',
            a: (
              <p>
                A withdrawal request unlocks after 128 blocks, a little over two hours.
                Add the roughly one day maturity period if the shares come from a recent
                deposit, and note that a claim can wait longer if the withdrawal reserve
                needs a validator exit to cover it.
              </p>
            ),
          },
          {
            q: 'Is QuantaPool live on mainnet?',
            a: (
              <p>
                No. The contracts are deployed only on the QRL 2.0 testnet, and testnet
                assets have no monetary value. See the legal notice at{' '}
                <a href="https://quantapool.com/legal">quantapool.com/legal</a> for the
                protocol&apos;s own statement on this.
              </p>
            ),
          },
        ]}
      />
    </LearnArticle>
  );
}
